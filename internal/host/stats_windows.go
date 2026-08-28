// internal/host/stats_windows.go
//go:build windows

package host

import (
	"encoding/json"
	"runtime"
)

func collectPlatform() (platformSnapshot, error) {
	payload, err := collectWindowsSnapshot()
	if err != nil {
		return platformSnapshot{}, err
	}
	mem, uptime, cpu, storage := windowsPlatformValues(payload)
	return platformSnapshot{
		OS: "Windows", Kernel: payload.Version,
		Arch: archLabel(runtime.GOARCH), Uptime: uptime,
		Memory: mem, CPU: cpu, Storage: storage, Network: map[string]ifStats{},
	}, nil
}

func collectWindowsSnapshot() (windowsSnapshotPayload, error) {
	script := `$ErrorActionPreference = 'Stop'
$os = Get-CimInstance Win32_OperatingSystem
$cpuUsage = 0
try {
  $cpuUsage = [double](Get-CimInstance Win32_PerfFormattedData_PerfOS_Processor -Filter "Name='_Total'").PercentProcessorTime
} catch {
  $values = @(Get-CimInstance Win32_Processor | ForEach-Object { [double]$_.LoadPercentage })
  if ($values.Count -gt 0) { $cpuUsage = [double](($values | Measure-Object -Average).Average) }
}
$disks = @()
try { $disks = @(Get-CimInstance Win32_LogicalDisk -Filter "DriveType=3" | Select-Object DeviceID,FileSystem,Size,FreeSpace) } catch {}
[pscustomobject]@{
  TotalVisibleMemorySize = [uint64]$os.TotalVisibleMemorySize
  FreePhysicalMemory = [uint64]$os.FreePhysicalMemory
  UptimeSeconds = [uint64]((Get-Date) - $os.LastBootUpTime).TotalSeconds
  Version = [string]$os.Version
  CPUUsage = $cpuUsage
  Disks = $disks
} | ConvertTo-Json -Compress -Depth 4`
	raw, err := runPowerShell(script)
	if err != nil {
		return windowsSnapshotPayload{}, err
	}
	var payload windowsSnapshotPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return windowsSnapshotPayload{}, err
	}
	return payload, nil
}
