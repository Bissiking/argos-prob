// internal/host/stats_linux.go
//go:build linux

package host

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

var (
	prevCPUIdle  uint64
	prevCPUTotal uint64
	cpuInit      bool
)

func collectPlatform() (platformSnapshot, error) {
	osName, _ := osReleaseName()
	kernel, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return platformSnapshot{}, err
	}
	uptime, err := linuxUptime()
	if err != nil {
		return platformSnapshot{}, err
	}
	mem, err := linuxMemory()
	if err != nil {
		return platformSnapshot{}, err
	}
	cpu, err := linuxCPU()
	if err != nil {
		return platformSnapshot{}, err
	}
	storage, err := linuxStorage()
	if err != nil {
		return platformSnapshot{}, err
	}
	network, err := linuxNetwork()
	if err != nil {
		return platformSnapshot{}, err
	}
	return platformSnapshot{
		OS: osName, Kernel: strings.TrimSpace(string(kernel)),
		Arch: archLabel(runtime.GOARCH), Uptime: uptime,
		Memory: mem, CPU: cpu, Storage: storage, Network: network,
	}, nil
}

func osReleaseName() (string, error) {
	raw, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "Linux", nil
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			name := strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
			if name != "" {
				return name, nil
			}
		}
	}
	return "Linux", nil
}

func linuxUptime() (uint64, error) {
	raw, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return 0, fmt.Errorf("invalid /proc/uptime")
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}
	return uint64(seconds), nil
}

func linuxMemory() (MemorySnapshot, error) {
	values := map[string]uint64{}
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return MemorySnapshot{}, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		if v, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
			values[key] = v * 1024
		}
	}
	total := values["MemTotal"]
	available := values["MemAvailable"]
	swapTotal := values["SwapTotal"]
	swapUsed := values["SwapTotal"] - values["SwapFree"]
	used := total - available
	return MemorySnapshot{Total: total, Used: used, SwapTotal: swapTotal, SwapUsed: swapUsed}, nil
}

func linuxCPU() (CpuSnapshot, error) {
	var load [3]float64
	raw, err := os.ReadFile("/proc/loadavg")
	if err == nil {
		fields := strings.Fields(string(raw))
		for i := 0; i < 3 && i < len(fields); i++ {
			if v, err := strconv.ParseFloat(fields[i], 64); err == nil {
				load[i] = v
			}
		}
	}
	usage, err := cpuUsagePercent()
	if err != nil && !cpuInit {
		return CpuSnapshot{}, err
	}
	return CpuSnapshot{Usage: usage, Load: load, Cores: runtime.NumCPU()}, nil
}

func cpuUsagePercent() (float64, error) {
	raw, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}
	var total, idle uint64
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)[1:]
		if len(fields) < 8 {
			continue
		}
		for i, field := range fields {
			v, _ := strconv.ParseUint(field, 10, 64)
			if i == 3 || i == 4 { // idle + iowait
				idle += v
			}
			total += v
		}
		break
	}
	if total == 0 {
		return 0, fmt.Errorf("empty /proc/stat")
	}
	if !cpuInit {
		cpuInit = true
		prevCPUIdle, prevCPUTotal = idle, total
		return 0, nil
	}
	dIdle := idle - prevCPUIdle
	dTotal := total - prevCPUTotal
	prevCPUIdle, prevCPUTotal = idle, total
	if dTotal == 0 {
		return 0, nil
	}
	usage := (1 - float64(dIdle)/float64(dTotal)) * 100
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}
	return usage, nil
}

var realFilesystems = map[string]bool{
	"ext2": true, "ext3": true, "ext4": true, "xfs": true, "btrfs": true,
	"zfs": true, "reiserfs": true, "jfs": true, "f2fs": true,
	"vfat": true, "exfat": true, "ntfs": true, "iso9660": true,
}

func linuxStorage() ([]StorageVolume, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	seen := map[string]bool{}
	out := make([]StorageVolume, 0)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		device, mount, fstype := fields[0], unescapeMount(fields[1]), fields[2]
		if !realFilesystems[fstype] || seen[mount] {
			continue
		}
		seen[mount] = true
		var st syscall.Statfs_t
		if err := syscall.Statfs(mount, &st); err != nil {
			continue
		}
		block := uint64(st.Bsize)
		if st.Frsize > 0 {
			block = uint64(st.Frsize)
		}
		size := st.Blocks * block
		if size == 0 {
			continue
		}
		used := (st.Blocks - st.Bfree) * block
		available := st.Bavail * block
		out = append(out, StorageVolume{
			Device: device, Mount: mount, Filesystem: fstype,
			Size: size, Used: used, Available: available,
			Usage: float64(used) / float64(size) * 100,
		})
	}
	if err := scanner.Err(); err != nil {
		return out, err
	}
	return out, nil
}

func unescapeMount(raw string) string {
	r := strings.NewReplacer("\\040", " ", "\\011", "\t", "\\012", "\n", "\\134", "\\")
	return r.Replace(raw)
}

func linuxNetwork() (map[string]ifStats, error) {
	counters := map[string]ifStats{}
	raw, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(raw), "\n")
	for _, line := range lines {
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		name := strings.TrimSpace(line[:idx])
		fields := strings.Fields(line[idx+1:])
		if len(fields) < 16 {
			continue
		}
		rx, _ := strconv.ParseUint(fields[0], 10, 64)
		tx, _ := strconv.ParseUint(fields[8], 10, 64)
		counters[name] = ifStats{RxBytes: rx, TxBytes: tx}
	}
	entries, err := os.ReadDir("/sys/class/net")
	if err == nil {
		for _, entry := range entries {
			name := entry.Name()
			st := counters[name]
			if state, err := os.ReadFile("/sys/class/net/" + name + "/operstate"); err == nil {
				st.State = strings.TrimSpace(string(state))
			}
			if speed, err := os.ReadFile("/sys/class/net/" + name + "/speed"); err == nil {
				value, perr := strconv.ParseUint(strings.TrimSpace(string(speed)), 10, 64)
				if perr == nil && value > 1 && value != ^uint64(0) {
					st.SpeedMbps = value
				}
			}
			counters[name] = st
		}
	}
	return counters, nil
}
