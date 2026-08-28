//go:build windows

package host

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func runPowerShell(script string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	bin, err := exec.LookPath("powershell.exe")
	if err != nil {
		return nil, fmt.Errorf("PowerShell introuvable: %w", err)
	}
	argv := []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", "[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false); " + script}
	argv = append(argv, args...)
	cmd := exec.CommandContext(ctx, bin, argv...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("powershell.exe: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}
