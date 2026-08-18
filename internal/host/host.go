// internal/host/host.go
package host

import (
	"net"
	"os"
	"strings"
	"time"

	"github.com/Bissiking/argos-prob/internal/capabilities"
)

// Snapshot is the host inventory as expected by the Argos master
// (packages/contracts AgentSnapshot). Keys follow the master camelCase.
type Snapshot struct {
	CollectedAt  time.Time          `json:"collectedAt"`
	AgentID      string             `json:"agentId,omitempty"`
	Hostname     string             `json:"hostname"`
	IP           *string            `json:"ip"`
	OS           string             `json:"os"`
	Kernel       string             `json:"kernel"`
	Architecture string             `json:"architecture"`
	Uptime       uint64             `json:"uptime"`
	Capabilities Capabilities       `json:"capabilities"`
	CPU          CpuSnapshot        `json:"cpu"`
	Memory       MemorySnapshot     `json:"memory"`
	Storage      []StorageVolume    `json:"storage"`
	Network      []NetworkInterface `json:"network"`
	Services     []ServiceInfo      `json:"services"`
	Docker       []DockerContainer  `json:"docker"`
	Proxmox      []ProxmoxGuest     `json:"proxmox"`
}

type Capabilities struct {
	Systemd bool `json:"systemd"`
	Docker  bool `json:"docker"`
	Proxmox bool `json:"proxmox"`
}

type CpuSnapshot struct {
	Usage float64    `json:"usage"`
	Load  [3]float64 `json:"load"`
	Cores int        `json:"cores"`
}

type MemorySnapshot struct {
	Total     uint64 `json:"total"`
	Used      uint64 `json:"used"`
	SwapTotal uint64 `json:"swapTotal"`
	SwapUsed  uint64 `json:"swapUsed"`
}

type StorageVolume struct {
	Device     string  `json:"device"`
	Mount      string  `json:"mount"`
	Filesystem string  `json:"filesystem"`
	Size       uint64  `json:"size"`
	Used       uint64  `json:"used"`
	Available  uint64  `json:"available"`
	Usage      float64 `json:"usage"`
}

type NetworkInterface struct {
	Name      string  `json:"name"`
	IPv4      *string `json:"ipv4"`
	IPv6      *string `json:"ipv6"`
	MAC       string  `json:"mac"`
	State     string  `json:"state"`
	SpeedMbps uint64  `json:"speedMbps"`
	RxBytes   uint64  `json:"rxBytes"`
	TxBytes   uint64  `json:"txBytes"`
}

type ServiceInfo struct {
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	ActiveState   string  `json:"activeState"`
	SubState      string  `json:"subState"`
	UnitFileState string  `json:"unitFileState"`
	PID           *uint64 `json:"pid"`
	MemoryBytes   *uint64 `json:"memoryBytes"`
	Controllable  bool    `json:"controllable"`
}

type DockerContainer struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Image  string `json:"image"`
	State  string `json:"state"`
	Status string `json:"status"`
	Ports  string `json:"ports"`
}

type ProxmoxGuest struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	Node      string  `json:"node"`
	Status    string  `json:"status"`
	CPU       float64 `json:"cpu"`
	MaxMemory uint64  `json:"maxMemory"`
	Memory    uint64  `json:"memory"`
	MaxDisk   uint64  `json:"maxDisk"`
	Disk      uint64  `json:"disk"`
	Uptime    uint64  `json:"uptime"`
	Template  bool    `json:"template"`
}

// ifStats carries the per-interface counters the platform layer can provide.
// A nil/absent entry for an interface name means "counters unknown".
type ifStats struct {
	State     string
	SpeedMbps uint64
	RxBytes   uint64
	TxBytes   uint64
}

// platformSnapshot holds every field that depends on the operating system.
type platformSnapshot struct {
	OS      string
	Kernel  string
	Arch    string
	Uptime  uint64
	Memory  MemorySnapshot
	CPU     CpuSnapshot
	Storage []StorageVolume
	Network map[string]ifStats
}

func Collect(agentID string, caps capabilities.Capabilities) (Snapshot, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return Snapshot{}, err
	}
	p, err := collectPlatform()
	if err != nil {
		return Snapshot{}, err
	}
	ifaces := networkInterfaces(p.Network)
	ip := primaryIPv4(ifaces)
	services := collectServices()
	if services == nil {
		services = []ServiceInfo{}
	}
	docker := collectDocker()
	if docker == nil {
		docker = []DockerContainer{}
	}
	proxmox := collectProxmox()
	if proxmox == nil {
		proxmox = []ProxmoxGuest{}
	}
	return Snapshot{
		CollectedAt:  time.Now().UTC(),
		AgentID:      agentID,
		Hostname:     hostname,
		IP:           ip,
		OS:           p.OS,
		Kernel:       p.Kernel,
		Architecture: p.Arch,
		Uptime:       p.Uptime,
		Capabilities: Capabilities{Systemd: caps.Systemd, Docker: caps.Docker, Proxmox: caps.Proxmox},
		CPU:          p.CPU,
		Memory:       p.Memory,
		Storage:      p.Storage,
		Network:      ifaces,
		Services:     services,
		Docker:       docker,
		Proxmox:      proxmox,
	}, nil
}

func networkInterfaces(counters map[string]ifStats) []NetworkInterface {
	all, err := net.Interfaces()
	if err != nil {
		return nil
	}
	out := make([]NetworkInterface, 0, len(all))
	for _, item := range all {
		entry := NetworkInterface{Name: item.Name, MAC: item.HardwareAddr.String()}
		if st, ok := counters[item.Name]; ok {
			entry.State = st.State
			entry.SpeedMbps = st.SpeedMbps
			entry.RxBytes = st.RxBytes
			entry.TxBytes = st.TxBytes
		} else if item.Flags&net.FlagUp != 0 {
			entry.State = "up"
		} else {
			entry.State = "down"
		}
		addrs, _ := item.Addrs()
		for _, addr := range addrs {
			raw := addr.String()
			hostPart := strings.Split(raw, "/")[0]
			if ip := net.ParseIP(hostPart); ip != nil {
				if ip.To4() != nil {
					if entry.IPv4 == nil {
						v := hostPart
						entry.IPv4 = &v
					}
				} else if entry.IPv6 == nil {
					v := hostPart
					entry.IPv6 = &v
				}
			}
		}
		out = append(out, entry)
	}
	return out
}

func primaryIPv4(ifaces []NetworkInterface) *string {
	for _, item := range ifaces {
		if item.IPv4 == nil || item.State == "down" {
			continue
		}
		name := item.Name
		if name == "lo" || name == "lo0" || strings.HasPrefix(name, "docker") ||
			strings.HasPrefix(name, "veth") || strings.HasPrefix(name, "br-") ||
			strings.HasPrefix(name, "virbr") || strings.HasPrefix(name, "tun") ||
			strings.HasPrefix(name, "utun") || strings.HasPrefix(name, "tailscale") ||
			strings.HasPrefix(name, "tap") {
			continue
		}
		ip := *item.IPv4
		if !net.ParseIP(ip).IsLoopback() {
			return &ip
		}
	}
	return nil
}

func archLabel(goarch string) string {
	switch goarch {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	default:
		return goarch
	}
}
