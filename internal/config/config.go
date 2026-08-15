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
)

type Config struct {
	AgentID  string `json:"agent_id"`
	Name     string `json:"name,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Token    string `json:"token,omitempty"`
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
	return cfg, path, nil
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
