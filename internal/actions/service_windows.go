//go:build windows

package actions

import (
	"errors"
	"os/exec"
)

func serviceArgv(action, target string) (string, []string, error) {
	bin, err := exec.LookPath("powershell.exe")
	if err != nil {
		return "", nil, errors.New("PowerShell introuvable : contrôles de services Windows indisponibles sur cet hôte")
	}
	commands := map[string]string{
		"start":   "Start-Service",
		"stop":    "Stop-Service",
		"restart": "Restart-Service",
	}
	script := `& { param($serviceName) ` + commands[action] + ` -Name $serviceName -ErrorAction Stop }`
	return bin, []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script, target}, nil
}
