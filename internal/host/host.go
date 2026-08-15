// internal/host/host.go
package host

import (
	"net"
	"os"
	"runtime"
	"time"

	"github.com/Bissiking/argos-prob/internal/capabilities"
)

type Snapshot struct {
	AgentID      string                    `json:"agent_id"`
	CollectedAt  time.Time                 `json:"collected_at"`
	Hostname     string                    `json:"hostname"`
	OS           string                    `json:"os"`
	Arch         string                    `json:"arch"`
	CPUCount     int                       `json:"cpu_count"`
	Memory       Memory                    `json:"memory"`
	UptimeSecs   uint64                    `json:"uptime_seconds"`
	Interfaces   []NetworkInterface        `json:"interfaces"`
	Capabilities capabilities.Capabilities `json:"capabilities"`
}

type Memory struct {
	Total uint64 `json:"total_bytes"`
	Used  uint64 `json:"used_bytes"`
	Free  uint64 `json:"free_bytes"`
}

type NetworkInterface struct {
	Name      string   `json:"name"`
	MAC       string   `json:"mac,omitempty"`
	Addresses []string `json:"addresses"`
}

func Collect(agentID string, caps capabilities.Capabilities) (Snapshot, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return Snapshot{}, err
	}
	mem, uptime, err := platformStats()
	if err != nil {
		return Snapshot{}, err
	}
	interfaces, err := networkInterfaces()
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		AgentID: agentID, CollectedAt: time.Now().UTC(), Hostname: hostname,
		OS: runtime.GOOS, Arch: runtime.GOARCH, CPUCount: runtime.NumCPU(),
		Memory: mem, UptimeSecs: uptime, Interfaces: interfaces, Capabilities: caps,
	}, nil
}

func networkInterfaces() ([]NetworkInterface, error) {
	all, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	out := make([]NetworkInterface, 0, len(all))
	for _, item := range all {
		addrs, _ := item.Addrs()
		entry := NetworkInterface{Name: item.Name, MAC: item.HardwareAddr.String()}
		for _, addr := range addrs {
			entry.Addresses = append(entry.Addresses, addr.String())
		}
		out = append(out, entry)
	}
	return out, nil
}
