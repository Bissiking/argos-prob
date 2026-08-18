// internal/config/config.go
package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Bissiking/argos-prob/internal/actions"
)

const (
	ModePassive = "passive"
	ModeActive  = "active"
)

type Actions struct {
	Services   []string `json:"services,omitempty"`
	Containers []string `json:"containers,omitempty"`
	VMs        []int    `json:"vms,omitempty"`
}

type Config struct {
	AgentID      string  `json:"agent_id"`
	Name         string  `json:"name,omitempty"`
	Mode         string  `json:"mode,omitempty"`
	Endpoint     string  `json:"endpoint,omitempty"`
	Token        string  `json:"token,omitempty"`
	PushInterval uint64  `json:"push_interval_seconds,omitempty"`
	ListenAddr   string  `json:"listen_addr,omitempty"`
	Actions      Actions `json:"actions,omitempty"`
}

// ActionPolicy builds the remote-control allowlist from the configuration.
// An empty allowlist forbids every remote action: the agent stays read-only
// until its operator explicitly authorizes services, containers and VMs.
func (c Config) ActionPolicy() actions.Policy {
	return actions.Policy{
		Services:   c.Actions.Services,
		Containers: c.Actions.Containers,
		VMs:        c.Actions.VMs,
	}
}

func (c Config) TransportMode() string {
	switch c.Mode {
	case ModeActive, ModePassive:
		return c.Mode
	default:
		return ModePassive
	}
}

func (c Config) Interval() time.Duration {
	if c.PushInterval <= 0 {
		return 60 * time.Second
	}
	return time.Duration(c.PushInterval) * time.Second
}

func (c Config) Address() string {
	if c.ListenAddr == "" {
		return "127.0.0.1:8456"
	}
	return c.ListenAddr
}

func Path() (string, error) {
	if override := os.Getenv("ARGOS_PROB_CONFIG"); override != "" {
		return override, nil
	}

	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("ProgramData")
		if base == "" {
			return "", errors.New("ProgramData is not defined")
		}
		return filepath.Join(base, "ArgosProb", "config.json"), nil
	default:
		if os.Geteuid() == 0 {
			return filepath.Join(string(filepath.Separator), "etc", "argos-prob", "config.json"), nil
		}
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "argos-prob", "config.json"), nil
	}
}

func Ensure() (string, bool, error) {
	path, err := Path()
	if err != nil {
		return "", false, err
	}
	if _, err := os.Stat(path); err == nil {
		return path, false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", false, err
	}

	id, err := randomID()
	if err != nil {
		return "", false, err
	}
	hostname, _ := os.Hostname()
	cfg := Config{AgentID: id, Name: hostname}
	if err := write(path, cfg); err != nil {
		return "", false, err
	}
	return path, true, nil
}

func LoadOrCreate() (Config, string, error) {
	path, _, err := Ensure()
	if err != nil {
		return Config{}, "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, path, err
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, path, fmt.Errorf("invalid configuration: %w", err)
	}
	if cfg.AgentID == "" {
		return Config{}, path, errors.New("configuration has no agent_id")
	}
	if cfg.Mode != "" && cfg.Mode != ModePassive && cfg.Mode != ModeActive {
		return Config{}, path, fmt.Errorf("invalid mode %q (expected %q or %q)", cfg.Mode, ModePassive, ModeActive)
	}
	return cfg, path, nil
}

// Save persists the configuration to the given path, creating its directory
// and tightening file permissions to the current user only.
func Save(path string, cfg Config) error {
	if cfg.AgentID == "" {
		return errors.New("cannot save a configuration without agent_id")
	}
	return write(path, cfg)
}

func write(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(path, raw, 0o600)
}

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
