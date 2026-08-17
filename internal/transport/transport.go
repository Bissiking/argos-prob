// internal/transport/transport.go
package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Bissiking/argos-prob/internal/capabilities"
	"github.com/Bissiking/argos-prob/internal/config"
	"github.com/Bissiking/argos-prob/internal/host"
)

const userAgent = "argos-prob"

// PushLoop runs the passive mode: every config interval, the agent collects
// the host snapshot and POSTs it to the configured Argos endpoint.
func PushLoop(ctx context.Context, cfg config.Config) error {
	if cfg.Endpoint == "" {
		return errors.New(`passive mode requires an "endpoint" in the configuration`)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	url := cfg.Endpoint + "/metrics"
	interval := cfg.Interval()

	log.Printf("passive mode: pushing every %s to %s", interval, url)

	for {
		snapshot, err := collect(cfg)
		if err != nil {
			log.Printf("collect failed: %v", err)
		} else if err := push(client, url, cfg.Token, snapshot); err != nil {
			log.Printf("push failed: %v", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(interval):
		}
	}
}

// Serve runs the active mode: the agent exposes a local HTTP endpoint that
// Argos can pull from at its own rate, so the agent is never overloaded.
func Serve(ctx context.Context, cfg config.Config) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", health(cfg))
	mux.HandleFunc("/metrics", metrics(cfg))

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

func metrics(cfg config.Config) http.HandlerFunc {
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

func collect(cfg config.Config) (host.Snapshot, error) {
	return host.Collect(cfg.AgentID, capabilities.Detect())
}

func push(client *http.Client, url, token string, snapshot host.Snapshot) error {
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("push to %s returned %s", url, resp.Status)
	}
	return nil
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