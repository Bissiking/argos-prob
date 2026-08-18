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
	"strings"
	"time"

	"github.com/Bissiking/argos-prob/internal/capabilities"
	"github.com/Bissiking/argos-prob/internal/config"
	"github.com/Bissiking/argos-prob/internal/host"
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
	Mode       string `json:"mode"`
	Status     string `json:"status"` // pending | approved | rejected
	IntervalMs int    `json:"intervalMs,omitempty"`
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
watch:
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		status, intervalMs, err := syncWithMaster(ctx, client, base, cfg.Token, hostname)
		if err != nil {
			log.Printf("master injoignable (%v) — nouvelle tentative dans %s", err, retryDelay)
		} else {
			switch status {
			case "approved":
				if intervalMs > 0 {
					interval = time.Duration(intervalMs) * time.Millisecond
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

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(interval):
		}
	}
}

// Serve runs the active mode: the agent exposes a local endpoint that Argos
// can pull from at its own rate, so the agent is never overloaded.
func Serve(ctx context.Context, cfg config.Config) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", health(cfg))
	mux.HandleFunc("/metrics", metrics(cfg))
	mux.HandleFunc("/api/v1/snapshot", snapshotV1(cfg))

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
	return host.Collect(cfg.AgentID, capabilities.Detect())
}

func syncWithMaster(ctx context.Context, client *http.Client, base, token, hostname string) (string, int, error) {
	reg, err := register(ctx, client, base, token, hostname)
	if err != nil {
		return "", 0, err
	}
	if reg.Status == "rejected" {
		return "rejected", 0, nil
	}
	conf, err := fetchConfig(ctx, client, base, token)
	if err != nil {
		return "", 0, err
	}
	intervalMs := conf.IntervalMs
	if intervalMs <= 0 {
		intervalMs = reg.IntervalMs
	}
	return conf.Status, intervalMs, nil
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
