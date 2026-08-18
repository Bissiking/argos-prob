// internal/host/stats_windows.go
//go:build windows

package host

import (
	"encoding/json"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

type windowsStats struct {
	TotalVisibleMemorySize uint64 `json:"TotalVisibleMemorySize"`
	FreePhysicalMemory     uint64 `json:"FreePhysicalMemory"`
	LastBootUpTime         string `json:"LastBootUpTime"`
	LocalDateTime          string `json:"LocalDateTime"`
}

func collectPlatform() (platformSnapshot, error) {
	mem, uptime, err := windowsMemoryAndUptime()
	if err != nil {
		return platformSnapshot{}, err
	}
	cpu := CpuSnapshot{Load: [3]float64{}, Cores: runtime.NumCPU()}
	return platformSnapshot{
		OS: "Windows", Kernel: "Windows",
		Arch: archLabel(runtime.GOARCH), Uptime: uptime,
		Memory: mem, CPU: cpu, Storage: []StorageVolume{}, Network: map[string]ifStats{},
	}, nil
}

func windowsMemoryAndUptime() (MemorySnapshot, uint64, error) {
	script := `$os = Get-CimInstance Win32_OperatingSystem | Select-Object TotalVisibleMemorySize,FreePhysicalMemory,LastBootUpTime,LocalDateTime; $os | ConvertTo-Json -Compress`
	raw, err := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return MemorySnapshot{}, 0, err
	}
	var s windowsStats
	if err := json.Unmarshal(raw, &s); err != nil {
		return MemorySnapshot{}, 0, err
	}
	total := s.TotalVisibleMemorySize * 1024
	free := s.FreePhysicalMemory * 1024

	uptimeRaw, err := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", "[int64]((Get-Date)-(Get-CimInstance Win32_OperatingSystem).LastBootUpTime).TotalSeconds").Output()
	if err != nil {
		return MemorySnapshot{}, 0, err
	}
	uptime, _ := strconv.ParseUint(strings.TrimSpace(string(uptimeRaw)), 10, 64)
	used := total - free
	if free > total {
		used = 0
	}
	return MemorySnapshot{Total: total, Used: used}, uptime, nil
}
