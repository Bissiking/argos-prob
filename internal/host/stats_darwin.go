// internal/host/stats_darwin.go
//go:build darwin

package host

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

func collectPlatform() (platformSnapshot, error) {
	osName := "Darwin"
	kernel, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return platformSnapshot{}, err
	}
	mem, err := darwinMemory()
	if err != nil {
		return platformSnapshot{}, err
	}
	uptime, err := darwinUptime()
	if err != nil {
		return platformSnapshot{}, err
	}
	cpu, err := darwinCPU()
	if err != nil {
		return platformSnapshot{}, err
	}
	storage, err := darwinStorage()
	if err != nil {
		return platformSnapshot{}, err
	}
	return platformSnapshot{
		OS: osName, Kernel: strings.TrimSpace(string(kernel)),
		Arch: archLabel(runtime.GOARCH), Uptime: uptime,
		Memory: mem, CPU: cpu, Storage: storage, Network: map[string]ifStats{},
	}, nil
}

func darwinUptime() (uint64, error) {
	boot, err := exec.Command("sysctl", "-n", "kern.boottime").Output()
	if err != nil {
		return 0, err
	}
	var sec uint64
	if _, err := fmt.Sscanf(string(boot), "{ sec = %d,", &sec); err != nil {
		return 0, fmt.Errorf("parse kern.boottime: %w", err)
	}
	now, err := commandUint("date", "+%s")
	if err != nil {
		return 0, err
	}
	return now - sec, nil
}

func darwinMemory() (MemorySnapshot, error) {
	total, err := commandUint("sysctl", "-n", "hw.memsize")
	if err != nil {
		return MemorySnapshot{}, err
	}
	free, err := vmStatFreeBytes()
	if err != nil {
		return MemorySnapshot{}, err
	}
	if free > total {
		free = total
	}
	swap := MemorySnapshot{
		Total: total, Used: total - free,
		SwapTotal: 0, SwapUsed: 0,
	}
	raw, err := exec.Command("sysctl", "-n", "vm.swapusage").Output()
	if err == nil {
		// Format: total = 2048.00M  used = 512.00M  free = 1536.00M ...
		fields := strings.Fields(string(raw))
		for i := 0; i+3 < len(fields); i++ {
			if fields[i] == "total" && fields[i+1] == "=" {
				if mb, perr := strconv.ParseFloat(strings.TrimSuffix(fields[i+2], "M"), 64); perr == nil {
					swap.SwapTotal = uint64(mb * 1024 * 1024)
				}
			}
			if fields[i] == "used" && fields[i+1] == "=" {
				if mb, perr := strconv.ParseFloat(strings.TrimSuffix(fields[i+2], "M"), 64); perr == nil {
					swap.SwapUsed = uint64(mb * 1024 * 1024)
				}
			}
		}
	}
	return swap, nil
}

func darwinCPU() (CpuSnapshot, error) {
	var load [3]float64
	raw, err := exec.Command("sysctl", "-n", "vm.loadavg").Output()
	if err == nil {
		fields := strings.Split(strings.Trim(string(raw), "{ }\n"), " ")
		for i := 0; i < 3 && i < len(fields); i++ {
			if v, perr := strconv.ParseFloat(strings.TrimSpace(fields[i]), 64); perr == nil {
				load[i] = v
			}
		}
	}
	cores := runtime.NumCPU()
	usage := 0.0
	if cores > 0 {
		// Approximation faute de host_statistics (pas de cgo).
		usage = load[0] / float64(cores) * 100
		if usage < 0 {
			usage = 0
		}
		if usage > 100 {
			usage = 100
		}
	}
	return CpuSnapshot{Usage: usage, Load: load, Cores: cores}, nil
}

func darwinStorage() ([]StorageVolume, error) {
	raw, err := exec.Command("df", "-kP").Output()
	if err != nil {
		return nil, err
	}
	out := make([]StorageVolume, 0, 4)
	for _, line := range strings.Split(string(raw), "\n")[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		device := fields[0]
		if strings.HasPrefix(device, "devfs") || strings.HasPrefix(device, "map") {
			continue
		}
		mount := strings.Join(fields[5:], " ")
		blocks, err1 := strconv.ParseUint(fields[1], 10, 64)
		used, err2 := strconv.ParseUint(fields[2], 10, 64)
		available, err3 := strconv.ParseUint(fields[3], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		size := blocks * 1024
		if size == 0 {
			continue
		}
		out = append(out, StorageVolume{
			Device: device, Mount: mount, Filesystem: "apfs", Size: size,
			Used: used * 1024, Available: available * 1024,
			Usage: float64(used*1024) / float64(size) * 100,
		})
	}
	return out, nil
}

func commandUint(name string, args ...string) (uint64, error) {
	raw, err := exec.Command(name, args...).Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
}

func vmStatFreeBytes() (uint64, error) {
	raw, err := exec.Command("vm_stat").Output()
	if err != nil {
		return 0, err
	}
	var pageSize uint64 = 4096
	var freePages uint64
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, "page size of") {
			fmt.Sscanf(line, "Mach Virtual Memory Statistics: (page size of %d bytes)", &pageSize)
		}
		if strings.HasPrefix(line, "Pages free:") || strings.HasPrefix(line, "Pages speculative:") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				v, _ := strconv.ParseUint(strings.TrimSuffix(fields[2], "."), 10, 64)
				freePages += v
			}
		}
	}
	return freePages * pageSize, nil
}
