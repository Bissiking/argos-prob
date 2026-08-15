// internal/host/stats_windows.go
//go:build windows

package host

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

type windowsStats struct {
	TotalVisibleMemorySize uint64 `json:"TotalVisibleMemorySize"`
	FreePhysicalMemory     uint64 `json:"FreePhysicalMemory"`
	LastBootUpTime         string `json:"LastBootUpTime"`
	LocalDateTime          string `json:"LocalDateTime"`
}

func platformStats() (Memory, uint64, error) {
	script := `$os = Get-CimInstance Win32_OperatingSystem | Select-Object TotalVisibleMemorySize,FreePhysicalMemory,LastBootUpTime,LocalDateTime; $os | ConvertTo-Json -Compress`
	raw, err := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return Memory{}, 0, err
	}
	var s windowsStats
	if err := json.Unmarshal(raw, &s); err != nil {
		return Memory{}, 0, err
	}
	total := s.TotalVisibleMemorySize * 1024
	free := s.FreePhysicalMemory * 1024

	uptimeRaw, err := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", "[int64]((Get-Date)-(Get-CimInstance Win32_OperatingSystem).LastBootUpTime).TotalSeconds").Output()
	if err != nil {
		return Memory{}, 0, err
	}
	var uptime uint64
	if _, err := fmt.Sscanf(string(uptimeRaw), "%d", &uptime); err != nil {
		return Memory{}, 0, err
	}
	return Memory{Total: total, Used: total - free, Free: free}, uptime, nil
}
