package host

import (
	"runtime"
	"strings"

	"github.com/Bissiking/argos-prob/internal/actions"
)

type windowsSnapshotPayload struct {
	TotalVisibleMemorySize uint64        `json:"TotalVisibleMemorySize"`
	FreePhysicalMemory     uint64        `json:"FreePhysicalMemory"`
	UptimeSeconds          uint64        `json:"UptimeSeconds"`
	Version                string        `json:"Version"`
	CPUUsage               float64       `json:"CPUUsage"`
	Disks                  []windowsDisk `json:"Disks"`
}

type windowsDisk struct {
	DeviceID   string `json:"DeviceID"`
	FileSystem string `json:"FileSystem"`
	Size       uint64 `json:"Size"`
	FreeSpace  uint64 `json:"FreeSpace"`
}

type windowsService struct {
	Name        string `json:"Name"`
	DisplayName string `json:"DisplayName"`
	State       string `json:"State"`
	StartMode   string `json:"StartMode"`
	ProcessID   uint64 `json:"ProcessId"`
}

func windowsPlatformValues(s windowsSnapshotPayload) (MemorySnapshot, uint64, CpuSnapshot, []StorageVolume) {
	total := s.TotalVisibleMemorySize * 1024
	free := s.FreePhysicalMemory * 1024
	used := total - free
	if free > total {
		used = 0
	}
	usage := s.CPUUsage
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}
	storage := make([]StorageVolume, 0, len(s.Disks))
	for _, disk := range s.Disks {
		if disk.Size == 0 {
			continue
		}
		available := disk.FreeSpace
		if available > disk.Size {
			available = disk.Size
		}
		usedBytes := disk.Size - available
		storage = append(storage, StorageVolume{
			Device: disk.DeviceID, Mount: disk.DeviceID + `\`, Filesystem: disk.FileSystem,
			Size: disk.Size, Used: usedBytes, Available: available,
			Usage: float64(usedBytes) / float64(disk.Size) * 100,
		})
	}
	return MemorySnapshot{Total: total, Used: used}, s.UptimeSeconds,
		CpuSnapshot{Usage: usage, Load: [3]float64{}, Cores: runtime.NumCPU()}, storage
}

func windowsServiceInfo(services []windowsService, policy actions.Policy) []ServiceInfo {
	out := make([]ServiceInfo, 0, len(services))
	for _, service := range services {
		state := strings.ToLower(service.State)
		active := "inactive"
		switch state {
		case "running", "paused":
			active = "active"
		case "start pending", "continue pending":
			active = "activating"
		case "stop pending", "pause pending":
			active = "deactivating"
		}
		var pid *uint64
		if service.ProcessID > 0 {
			value := service.ProcessID
			pid = &value
		}
		out = append(out, ServiceInfo{
			Name: service.Name, Description: service.DisplayName,
			ActiveState: active, SubState: state, UnitFileState: strings.ToLower(service.StartMode), PID: pid,
			Controllable: policy.Controllable(actions.CategoryService, service.Name, 0),
		})
	}
	return out
}
