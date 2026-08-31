// internal/host/inventory_linux.go
//go:build linux

package host

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Bissiking/argos-prob/internal/actions"
)

var socketProcessPattern = regexp.MustCompile(`\(\("([^\"]+)"[^)]*pid=(\d+)`)

func collectVirtualizationIdentity() *VirtualizationIdentity {
	identity := &VirtualizationIdentity{Kind: "unknown"}
	if detector, err := exec.LookPath("systemd-detect-virt"); err == nil {
		if raw, runErr := runFor(3*time.Second, detector); runErr == nil {
			provider := strings.TrimSpace(string(raw))
			switch provider {
			case "", "none":
				identity.Kind = "physical"
			case "lxc":
				identity.Kind = "lxc"
			case "docker", "podman", "container-other":
				identity.Kind = "container"
			default:
				identity.Kind = "vm"
			}
			if provider != "" && provider != "none" {
				identity.Provider = &provider
			}
		}
	}
	if raw, err := os.ReadFile("/sys/class/dmi/id/product_uuid"); err == nil {
		if value := strings.ToLower(strings.TrimSpace(string(raw))); value != "" {
			identity.ProductUUID = &value
		}
	}
	if raw, err := os.ReadFile("/etc/machine-id"); err == nil {
		if value := strings.TrimSpace(string(raw)); value != "" {
			sum := fmt.Sprintf("%x", sha256.Sum256([]byte(value)))
			identity.MachineIDHash = &sum
		}
	}
	return identity
}

func collectSockets() []NetworkSocket {
	ss, err := exec.LookPath("ss")
	if err != nil {
		return nil
	}
	raw, err := runFor(10*time.Second, ss, "-H", "-tunap")
	if err != nil {
		return nil
	}
	out := make([]NetworkSocket, 0)
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 || (fields[0] != "tcp" && fields[0] != "udp") {
			continue
		}
		state := strings.ToLower(fields[1])
		if state != "listen" && state != "estab" && state != "unconn" {
			continue
		}
		localAddress, localPort, ok := parseSocketEndpoint(fields[4])
		if !ok {
			continue
		}
		entry := NetworkSocket{Protocol: fields[0], State: state, LocalAddress: localAddress, LocalPort: localPort}
		if remoteAddress, remotePort, remoteOK := parseSocketEndpoint(fields[5]); remoteOK && remotePort > 0 {
			entry.RemoteAddress = &remoteAddress
			entry.RemotePort = &remotePort
		}
		if match := socketProcessPattern.FindStringSubmatch(line); len(match) == 3 {
			process := match[1]
			entry.Process = &process
			if pid, parseErr := strconv.ParseUint(match[2], 10, 64); parseErr == nil {
				entry.PID = &pid
			}
		}
		out = append(out, entry)
	}
	return out
}

func parseSocketEndpoint(value string) (string, int, bool) {
	value = strings.TrimSpace(value)
	host, portRaw, err := net.SplitHostPort(value)
	if err != nil {
		idx := strings.LastIndex(value, ":")
		if idx < 0 {
			return "", 0, false
		}
		host, portRaw = value[:idx], value[idx+1:]
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil || port < 0 || port > 65535 {
		return "", 0, false
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	return host, port, true
}

func collectServices(policy actions.Policy) []ServiceInfo {
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
	type serviceDetails struct {
		PID           *uint64
		Memory        *uint64
		UnitFileState string
	}
	detailsByUnit := make(map[string]serviceDetails, len(units))
	showArgs := []string{"show", "--property=Id,MainPID,MemoryCurrent,UnitFileState"}
	for _, unit := range units {
		showArgs = append(showArgs, unit.Unit)
	}
	if detailsRaw, detailsErr := runFor(20*time.Second, systemctl, showArgs...); detailsErr == nil {
		for _, block := range strings.Split(string(detailsRaw), "\n\n") {
			values := map[string]string{}
			for _, line := range strings.Split(block, "\n") {
				key, value, ok := strings.Cut(line, "=")
				if ok {
					values[key] = value
				}
			}
			id := values["Id"]
			if id == "" {
				continue
			}
			details := serviceDetails{UnitFileState: values["UnitFileState"]}
			if pid, parseErr := strconv.ParseUint(values["MainPID"], 10, 64); parseErr == nil && pid > 0 {
				details.PID = &pid
			}
			if memory, parseErr := strconv.ParseUint(values["MemoryCurrent"], 10, 64); parseErr == nil {
				details.Memory = &memory
			}
			detailsByUnit[id] = details
		}
	}
	out := make([]ServiceInfo, 0, len(units))
	for _, unit := range units {
		details := detailsByUnit[unit.Unit]
		out = append(out, ServiceInfo{
			Name: unit.Unit, Description: unit.Description,
			ActiveState: unit.Active, SubState: unit.Sub,
			UnitFileState: details.UnitFileState, PID: details.PID, MemoryBytes: details.Memory,
			Controllable: policy.Controllable(actions.CategoryService, unit.Unit, 0),
		})
	}
	return out
}

func collectDocker(policy actions.Policy) []DockerContainer {
	docker, err := exec.LookPath("docker")
	if err != nil {
		return nil
	}
	raw, err := runFor(20*time.Second, docker, "ps", "-a", "--format", "{{json .}}")
	if err != nil {
		return nil
	}
	var containers []DockerContainer
	containerIDs := make([]string, 0)
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
		name := strings.Split(item.Names, ",")[0]
		name = strings.TrimSpace(name)
		containers = append(containers, DockerContainer{
			ID: id, Name: name, Image: item.Image, State: state, Status: item.Status, Ports: item.Ports,
			Controllable: policy.Controllable(actions.CategoryContainer, name, 0),
		})
		containerIDs = append(containerIDs, id)
	}
	if len(containerIDs) > 0 {
		args := append([]string{"inspect"}, containerIDs...)
		if inspectedRaw, inspectErr := runFor(20*time.Second, docker, args...); inspectErr == nil {
			var inspected []struct {
				ID     string `json:"Id"`
				Config struct {
					Labels map[string]string `json:"Labels"`
				} `json:"Config"`
				NetworkSettings struct {
					Networks map[string]struct {
						IPAddress string `json:"IPAddress"`
						Gateway   string `json:"Gateway"`
					} `json:"Networks"`
				} `json:"NetworkSettings"`
			}
			if json.Unmarshal(inspectedRaw, &inspected) == nil {
				for index := range containers {
					for _, details := range inspected {
						if !strings.HasPrefix(details.ID, containers[index].ID) {
							continue
						}
						containers[index].Labels = details.Config.Labels
						for name, network := range details.NetworkSettings.Networks {
							var ipAddress, gateway *string
							if network.IPAddress != "" {
								value := network.IPAddress
								ipAddress = &value
							}
							if network.Gateway != "" {
								value := network.Gateway
								gateway = &value
							}
							containers[index].Networks = append(containers[index].Networks, DockerNetwork{Name: name, IPAddress: ipAddress, Gateway: gateway})
						}
						break
					}
				}
			}
		}
	}
	return containers
}

func collectProxmox(policy actions.Policy) []ProxmoxGuest {
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
			Controllable: policy.Controllable(actions.CategoryProxmox, "", r.Vmid),
		})
		guest := &guests[len(guests)-1]
		path := fmt.Sprintf("/nodes/%s/%s/%d/config", r.Node, r.Type, r.Vmid)
		if configRaw, configErr := runFor(5*time.Second, pvesh, "get", path, "--output-format", "json"); configErr == nil {
			var config map[string]any
			if json.Unmarshal(configRaw, &config) == nil {
				if hostname, ok := config["hostname"].(string); ok && hostname != "" {
					guest.Hostname = &hostname
				}
				if smbios, ok := config["smbios1"].(string); ok {
					for _, part := range strings.Split(smbios, ",") {
						if value, found := strings.CutPrefix(part, "uuid="); found && value != "" {
							normalized := strings.ToLower(value)
							guest.UUID = &normalized
						}
					}
				}
			}
		}
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
