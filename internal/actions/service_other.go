//go:build !windows

package actions

import (
	"errors"
	"os/exec"
)

func serviceArgv(action, target string) (string, []string, error) {
	bin, err := exec.LookPath("systemctl")
	if err != nil {
		return "", nil, errors.New("systemctl introuvable : contrôles de services indisponibles sur cet hôte")
	}
	return bin, []string{action, target}, nil
}
