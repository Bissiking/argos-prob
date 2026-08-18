// internal/capabilities/capabilities.go
package capabilities

import (
	"os/exec"
	"runtime"
)

type Capabilities struct {
	OS              string `json:"os"`
	Systemd         bool   `json:"systemd"`
	WindowsServices bool   `json:"windows_services"`
	Launchd         bool   `json:"launchd"`
	Docker          bool   `json:"docker"`
	Proxmox         bool   `json:"proxmox"`
}

func Detect() Capabilities {
	c := Capabilities{OS: runtime.GOOS}
	c.Docker = exists("docker")

	switch runtime.GOOS {
	case "linux":
		c.Systemd = exists("systemctl")
		c.Proxmox = exists("pvesh") || exists("pveversion")
	case "windows":
		c.WindowsServices = true
	case "darwin":
		c.Launchd = exists("launchctl")
	}
	return c
}

func exists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
