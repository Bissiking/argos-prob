// internal/transport/transport.go
package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Bissiking/argos-prob/internal/actions"
	"github.com/Bissiking/argos-prob/internal/capabilities"
	"github.com/Bissiking/argos-prob/internal/config"
	"github.com/Bissiking/argos-prob/internal/host"
	"github.com/Bissiking/argos-prob/internal/version"
)

const userAgent = "argos-prob"

// retryDelay controls how fast the agent re-checks its association with the
// master while waiting for approval.
const retryDelay = 10 * time.Second

// ErrNotApproved is returned when the master answers 401/403 on a push,
// meaning the token was rejected or no longer associated.
var ErrNotApproved = errors.New("association non approuvée par le master")

type registrationResponse struct {
	Status     string `json:"status"` // pending | approved | rejected
	ServerID   string `json:"serverId,omitempty"`
	IntervalMs int    `json:"intervalMs,omitempty"`
}

type configResponse struct {
	Mode       string          `json:"mode"`
	Status     string          `json:"status"` // pending | approved | rejected
	IntervalMs int             `json:"intervalMs,omitempty"`
	Actions    json.RawMessage `json:"actions,omitempty"`
}

type metricsPayload struct {
	Token    string        `json:"token"`
	Snapshot host.Snapshot `json:"snapshot"`
}

// Associate registers the agent on the master. It is idempotent: an approved
// or pending token answers without side effect. It returns the association
// status the master reports.
func Associate(ctx context.Context, cfg config.Config, hostname string) (string, error) {
	base := baseURL(cfg.Endpoint)
	if base == "" {
		return "", errors.New(`passive mode requires an "endpoint" in the configuration`)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	reg, err := register(ctx, client, base, cfg.Token, hostname)
	if err != nil {
		return "", err
	}
	return reg.Status, nil
}

// PushLoop runs the passive mode: first it makes sure the association is
// approved by the master (waiting for the human to accept it), then it POSTs
// the host snapshot every configured interval.
func PushLoop(ctx context.Context, cfg config.Config) error {
	base := baseURL(cfg.Endpoint)
	if base == "" {
		return errors.New(`passive mode requires an "endpoint" in the configuration`)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	hostname := cfg.Name
	if hostname == "" {
		hostname, _ = os.Hostname()
	}

	interval := retryDelay
	var actions json.RawMessage
watch:
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		status, intervalMs, masterActions, err := syncWithMaster(ctx, client, base, cfg.Token, hostname)
		if err != nil {
			log.Printf("master injoignable (%v) — nouvelle tentative dans %s", err, retryDelay)
		} else {
			switch status {
			case "approved":
				if intervalMs > 0 {
					interval = time.Duration(intervalMs) * time.Millisecond
				}
				if masterActions != nil {
					actions = masterActions
				}
				log.Printf("association approuvée : poussée toutes les %s vers %s/api/v1/agents/metrics", interval, base)
				break watch
			case "rejected":
				return errors.New("association refusée par le master — générez une nouvelle invitation, puis relancez `argos-prob init`")
			default:
				log.Printf("en attente d'approbation par le master… nouvelle vérification dans %s", retryDelay)
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(retryDelay):
		}
	}

	if actions != nil {
		var a config.Actions
		if json.Unmarshal(actions, &a) == nil && (len(a.Services) > 0 || len(a.Containers) > 0 || len(a.VMs) > 0) {
			cfg.Actions = a
			if err := config.SaveConfig(cfg); err != nil {
				log.Printf("sauvegarde des actions depuis le master échouée: %v", err)
			} else {
				log.Printf("actions synchronisées depuis le master: %d services, %d conteneurs, %d VMs", len(a.Services), len(a.Containers), len(a.VMs))
			}
		}
	}

	for {
		snapshot, err := collect(cfg)
		if err != nil {
			log.Printf("collect failed: %v", err)
		} else if err := pushMetrics(ctx, client, base, cfg.Token, snapshot); err != nil {
			if errors.Is(err, ErrNotApproved) {
				log.Printf("association révoquée — retour en attente")
				time.Sleep(retryDelay)
				goto watch
			}
			log.Printf("push failed: %v", err)
		}

		if err := refreshCommands(ctx, client, cfg); err != nil {
			if errors.Is(err, ErrNotApproved) {
				log.Printf("association révoquée — retour en attente")
				time.Sleep(retryDelay)
				goto watch
			}
			log.Printf("commandes maîtresses injoignables: %v", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(interval):
		}
	}
}

// runCommand executes a typed command locally, provided the operator
// explicitly allowed it through the configuration allowlist.
func runCommand(cfg config.Config, cmd actions.Command) (string, error) {
	if !cfg.ActionPolicy().Allows(cmd) {
		return "", fmt.Errorf("action non autorisée par la liste d'autorisation Argos Prob (%s %s)", cmd.Category, cmd.Target)
	}
	return actions.Execute(cmd)
}

// pendingCommand is a queued typed operation the master can't reach the agent
// with in passive (push) mode: the master stores it and the agent pulls it.
type pendingCommand struct {
	ID          string `json:"id"`
	Category    string `json:"category"`
	Target      string `json:"target"`
	Action      string `json:"action"`
	ProxmoxType string `json:"proxmoxType,omitempty"`
}

// refreshCommands pulls the pending commands from the master, executes each
// allowed one locally and reports the result. It returns ErrNotApproved when
// the association was revoked.
func refreshCommands(ctx context.Context, client *http.Client, cfg config.Config) error {
	cmds, err := fetchPendingCommands(ctx, client, cfg)
	if err != nil {
		return err
	}
	for _, cmd := range cmds {
		command := actions.Command{Category: cmd.Category, Target: cmd.Target, Action: cmd.Action, Kind: cmd.ProxmoxType}
		if cmd.Category == actions.CategoryProxmox {
			vmid, convErr := strconv.Atoi(cmd.Target)
			if convErr != nil {
				_ = reportCommandResult(ctx, client, cfg, cmd.ID, false, "identifiant de machine invalide")
				continue
			}
			command.VMID = vmid
		}
		output, err := runCommand(cfg, command)
		ok := err == nil
		if err != nil {
			output = err.Error()
		}
		log.Printf("commande maîtresse exécutée (%s %s %s): %s", cmd.Category, cmd.Action, cmd.Target, output)
		if reportErr := reportCommandResult(ctx, client, cfg, cmd.ID, ok, output); reportErr != nil {
			log.Printf("rapport de commande impossible: %v", reportErr)
		}
	}
	return nil
}

func fetchPendingCommands(ctx context.Context, client *http.Client, cfg config.Config) ([]pendingCommand, error) {
	endpoint := baseURL(cfg.Endpoint) + agentsPath("/commands") + "?token=" + url.QueryEscape(cfg.Token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, agentError(resp.StatusCode, raw)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	var out []pendingCommand
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("commands: réponse illisible: %w", err)
	}
	return out, nil
}

func reportCommandResult(ctx context.Context, client *http.Client, cfg config.Config, id string, ok bool, output string) error {
	payload, _ := json.Marshal(commandResultPayload{Token: cfg.Token, OK: ok, Output: output})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL(cfg.Endpoint)+agentsPath("/commands/"+id+"/result"), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return agentError(resp.StatusCode, body)
	}
	return nil
}

type commandResultPayload struct {
	Token  string `json:"token"`
	OK     bool   `json:"ok"`
	Output string `json:"output,omitempty"`
}

// Serve runs the active mode: the agent exposes a local endpoint that Argos
// can pull from at its own rate, so the agent is never overloaded.
func Serve(ctx context.Context, cfg config.Config) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", health(cfg))
	mux.HandleFunc("/metrics", metrics(cfg))
	mux.HandleFunc("/api/v1/snapshot", snapshotV1(cfg))
	mux.HandleFunc("POST /api/v1/services/{unit}/{action}", actionV1(cfg, actions.CategoryService))
	mux.HandleFunc("POST /api/v1/containers/{name}/{action}", actionV1(cfg, actions.CategoryContainer))
	mux.HandleFunc("POST /api/v1/proxmox/{kind}/{vmid}/{action}", actionV1(cfg, actions.CategoryProxmox))

	srv := &http.Server{Addr: cfg.Address(), Handler: mux}
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	log.Printf("active mode: listening on http://%s", cfg.Address())

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func health(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Argos-Agent-ID", cfg.AgentID)
		w.Header().Set("X-Argos-Agent-Version", version.Version)
		fmt.Fprintln(w, "ok")
	}
}

func snapshotV1(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, cfg.Token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		snapshot, err := collect(cfg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(snapshot); err != nil {
			log.Printf("encode failed: %v", err)
		}
	}
}

func metrics(cfg config.Config) http.HandlerFunc {
	return snapshotV1(cfg)
}

func collect(cfg config.Config) (host.Snapshot, error) {
	return host.Collect(cfg.AgentID, capabilities.Detect(), cfg.ActionPolicy())
}

// actionV1 builds the typed-operation handler for a category. Every target is
// validated twice: strictly here (defense in depth) and against the operator
// allowlist before any process is launched.
func actionV1(cfg config.Config, category string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, cfg.Token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		cmd := actions.Command{Category: category, Action: r.PathValue("action")}
		switch category {
		case actions.CategoryService:
			cmd.Target = r.PathValue("unit")
		case actions.CategoryContainer:
			cmd.Target = r.PathValue("name")
		case actions.CategoryProxmox:
			cmd.Kind = r.PathValue("kind")
			vmid, err := strconv.Atoi(r.PathValue("vmid"))
			if err != nil || vmid <= 0 {
				http.Error(w, "identifiant de machine invalide", http.StatusBadRequest)
				return
			}
			cmd.VMID = vmid
		}
		output, err := runCommand(cfg, cmd)
		if err != nil {
			writeJSONError(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"ok": "true", "action": cmd.Action, "output": output}); err != nil {
			log.Printf("encode failed: %v", err)
		}
	}
}

func writeJSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"message": message}); err != nil {
		log.Printf("encode failed: %v", err)
	}
}

func syncWithMaster(ctx context.Context, client *http.Client, base, token, hostname string) (string, int, json.RawMessage, error) {
	reg, err := register(ctx, client, base, token, hostname)
	if err != nil {
		return "", 0, nil, err
	}
	if reg.Status == "rejected" {
		return "rejected", 0, nil, nil
	}
	conf, err := fetchConfig(ctx, client, base, token)
	if err != nil {
		return "", 0, nil, err
	}
	intervalMs := conf.IntervalMs
	if intervalMs <= 0 {
		intervalMs = reg.IntervalMs
	}
	return conf.Status, intervalMs, conf.Actions, nil
}

func register(ctx context.Context, client *http.Client, base, token, hostname string) (registrationResponse, error) {
	var out registrationResponse
	body, _ := json.Marshal(map[string]string{"token": token, "hostname": hostname})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+agentsPath("/register"), bytes.NewReader(body))
	if err != nil {
		return out, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return out, agentError(resp.StatusCode, raw)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("register: réponse illisible: %w", err)
	}
	return out, nil
}

func fetchConfig(ctx context.Context, client *http.Client, base, token string) (configResponse, error) {
	var out configResponse
	endpoint := base + agentsPath("/config") + "?token=" + url.QueryEscape(token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return out, agentError(resp.StatusCode, raw)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("config: réponse illisible: %w", err)
	}
	return out, nil
}

func pushMetrics(ctx context.Context, client *http.Client, base, token string, snapshot host.Snapshot) error {
	payload := metricsPayload{Token: token, Snapshot: snapshot}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+agentsPath("/metrics"), bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("%w (%s): %s", ErrNotApproved, resp.Status, strings.TrimSpace(string(body)))
		}
		return fmt.Errorf("push to %s returned %s", base+agentsPath("/metrics"), resp.Status)
	}
	return nil
}

func agentError(status int, body []byte) error {
	var payload struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &payload)
	if payload.Message == "" {
		payload.Message = strings.TrimSpace(string(body))
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return fmt.Errorf("%w (%s): %s", ErrNotApproved, http.StatusText(status), payload.Message)
	}
	return fmt.Errorf("master HTTP %d: %s", status, payload.Message)
}

func agentsPath(suffix string) string {
	return "/api/v1/agents" + suffix
}

func baseURL(endpoint string) string {
	return strings.TrimRight(endpoint, "/")
}

func authorized(r *http.Request, token string) bool {
	if token == "" {
		return true
	}
	auth := r.Header.Get("Authorization")
	if auth == "" || len(auth) <= len("Bearer ") {
		return false
	}
	return auth[len("Bearer "):] == token
}
