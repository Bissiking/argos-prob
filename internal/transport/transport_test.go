package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Bissiking/argos-prob/internal/actions"
	"github.com/Bissiking/argos-prob/internal/config"
	"github.com/Bissiking/argos-prob/internal/host"
)

func TestPushLoopAppliesLiveGrantsAndFullRevocation(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ARGOS_PROB_CONFIG", configPath)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var pushed, lookups, reported atomic.Int32
	master := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/agents/register":
			_, _ = w.Write([]byte(`{"status":"approved","intervalMs":1}`))
		case "/api/v1/agents/config":
			lookups.Add(1)
			if pushed.Load() == 1 {
				_, _ = w.Write([]byte(`{"status":"approved","intervalMs":1,"actions":{"services":["nginx"],"containers":["Aion_PRD"],"vms":[101]}}`))
			} else {
				_, _ = w.Write([]byte(`{"status":"approved","intervalMs":1,"actions":{}}`))
			}
		case "/api/v1/agents/metrics":
			pushed.Add(1)
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/api/v1/agents/commands":
			if pushed.Load() < 3 {
				_, _ = w.Write([]byte(`[]`))
			} else {
				_, _ = w.Write([]byte(`[{"id":"revoked","category":"container","target":"Aion_PRD","action":"stop"}]`))
			}
		case "/api/v1/agents/commands/revoked/result":
			var result struct {
				OK     bool   `json:"ok"`
				Output string `json:"output"`
			}
			if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
				t.Error(err)
			}
			if result.OK || result.Output == "" {
				t.Errorf("revoked command must fail: %+v", result)
			}
			reported.Add(1)
			_, _ = w.Write([]byte(`{"ok":true}`))
			cancel()
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer master.Close()

	var controls []bool
	err := pushLoop(ctx, config.Config{AgentID: "qa", Endpoint: master.URL, Token: "qa"}, func(cfg config.Config) (host.Snapshot, error) {
		allowed := cfg.ActionPolicy().Controllable(actions.CategoryContainer, "Aion_PRD", 0)
		controls = append(controls, allowed)
		return host.Snapshot{Hostname: "qa", Docker: []host.DockerContainer{{Name: "Aion_PRD", Controllable: allowed}}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(controls) != 3 || controls[0] || !controls[1] || controls[2] {
		t.Fatalf("live policy in snapshots = %v, want [false true false]", controls)
	}
	if lookups.Load() < 4 || reported.Load() != 1 {
		t.Fatalf("configuration lookups=%d, revoked result reports=%d", lookups.Load(), reported.Load())
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var saved config.Config
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.Actions.Services)+len(saved.Actions.Containers)+len(saved.Actions.VMs) != 0 {
		t.Fatalf("revocation was not persisted: %+v", saved.Actions)
	}
}

func TestRefreshRuntimeRejectsUnavailableOrInvalidPolicies(t *testing.T) {
	t.Setenv("ARGOS_PROB_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	for _, tc := range []struct {
		name, body string
		status     int
		revoked    bool
	}{
		{"offline", `unavailable`, 503, false},
		{"unauthorized", `{}`, 401, true},
		{"pending", `{"status":"pending"}`, 200, true},
		{"rejected", `{"status":"rejected"}`, 200, true},
		{"missing actions", `{"status":"approved"}`, 200, false},
		{"null actions", `{"status":"approved","actions":null}`, 200, false},
		{"invalid actions", `{"status":"approved","actions":{"containers":42}}`, 200, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			master := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer master.Close()
			cfg := config.Config{AgentID: "qa", Endpoint: master.URL, Actions: config.Actions{Containers: []string{"Aion_PRD"}}}
			_, err := refreshRuntime(context.Background(), master.Client(), &cfg)
			if err == nil || errors.Is(err, ErrNotApproved) != tc.revoked {
				t.Fatalf("unexpected refresh error: %v", err)
			}
			if !cfg.ActionPolicy().Controllable(actions.CategoryContainer, "Aion_PRD", 0) {
				t.Fatal("invalid response overwrote the cached policy")
			}
		})
	}
}

func TestPushLoopSuspendsCommandsWhenPolicyRefreshFails(t *testing.T) {
	t.Setenv("ARGOS_PROB_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var lookups, commands atomic.Int32
	master := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/agents/register":
			_, _ = w.Write([]byte(`{"status":"approved"}`))
		case "/api/v1/agents/config":
			if lookups.Add(1) == 1 {
				_, _ = w.Write([]byte(`{"status":"approved","actions":{}}`))
			} else {
				w.WriteHeader(503)
			}
		case "/api/v1/agents/metrics":
			cancel()
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/api/v1/agents/commands":
			commands.Add(1)
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer master.Close()
	err := pushLoop(ctx, config.Config{AgentID: "qa", Endpoint: master.URL}, func(config.Config) (host.Snapshot, error) { return host.Snapshot{Hostname: "qa"}, nil })
	if err != nil {
		t.Fatal(err)
	}
	if lookups.Load() != 2 || commands.Load() != 0 {
		t.Fatalf("lookups=%d, commands=%d: failed policy refresh must suspend commands", lookups.Load(), commands.Load())
	}
}
