// internal/host/inventory_linux.go
//go:build linux

package host

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func collectServices() []ServiceInfo {
	systemctl, err := exec.LookPath("systemctl")
	if err != nil {
		return nil
	}
	raw, err := runFor(20*time.Second, systemctl, "list-units", "--type=service", "--all", "--plain", "--no-legend", "--output=json")
	if err != nil {
		return nil
	}
	var units []struct {
		Unit        string `json:"unit"`
		Description string `json:"description"`
		Active      string `json:"active"`
		Sub         string `json:"sub"`
	}
	if err := json.Unmarshal(raw, &units); err != nil {
		return nil
	}
	out := make([]ServiceInfo, 0, len(units))
	for _, unit := range units {
		out = append(out, ServiceInfo{
			Name: unit.Unit, Description: unit.Description,
			ActiveState: unit.Active, SubState: unit.Sub,
			Controllable: true,
		})
	}
	return out
}

func collectDocker() []DockerContainer {
	docker, err := exec.LookPath("docker")
	if err != nil {
		return nil
	}
	raw, err := runFor(20*time.Second, docker, "ps", "-a", "--format", "{{json .}}")
	if err != nil {
		return nil
	}
	var containers []DockerContainer
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var item struct {
			ID     string `json:"ID"`
			Names  string `json:"Names"`
			Image  string `json:"Image"`
			Status string `json:"Status"`
			Ports  string `json:"Ports"`
		}
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			continue
		}
		state := "exited"
		if strings.HasPrefix(item.Status, "Up") {
			state = "running"
		}
		id := item.ID
		if len(id) > 12 {
			id = id[:12]
		}
		containers = append(containers, DockerContainer{
			ID: id, Name: item.Names, Image: item.Image, State: state, Status: item.Status, Ports: item.Ports,
		})
	}
	return containers
}

func collectProxmox() []ProxmoxGuest {
	pvesh, err := exec.LookPath("pvesh")
	if err != nil {
		return nil
	}
	raw, err := runFor(20*time.Second, pvesh, "get", "/cluster/resources", "--output-format", "json")
	if err != nil {
		return nil
	}
	var resources []struct {
		Vmid     int     `json:"vmid"`
		Name     string  `json:"name"`
		Type     string  `json:"type"`
		Node     string  `json:"node"`
		Status   string  `json:"status"`
		CPU      float64 `json:"cpu"`
		Mem      uint64  `json:"mem"`
		MaxMem   uint64  `json:"maxmem"`
		Disk     uint64  `json:"disk"`
		MaxDisk  uint64  `json:"maxdisk"`
		Uptime   uint64  `json:"uptime"`
		Template int     `json:"template"`
	}
	if err := json.Unmarshal(raw, &resources); err != nil {
		return nil
	}
	guests := make([]ProxmoxGuest, 0, len(resources))
	for _, r := range resources {
		if r.Type != "qemu" && r.Type != "lxc" {
			continue
		}
		guests = append(guests, ProxmoxGuest{
			ID: r.Vmid, Name: r.Name, Type: r.Type, Node: r.Node, Status: r.Status,
			CPU: r.CPU, MaxMemory: r.MaxMem, Memory: r.Mem,
			MaxDisk: r.MaxDisk, Disk: r.Disk, Uptime: r.Uptime, Template: r.Template != 0,
		})
	}
	return guests
}

func runFor(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}
