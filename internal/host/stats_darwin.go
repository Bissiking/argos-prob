// internal/host/stats_darwin.go
//go:build darwin

package host

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func platformStats() (Memory, uint64, error) {
	total, err := commandUint("sysctl", "-n", "hw.memsize")
	if err != nil {
		return Memory{}, 0, err
	}
	pagesFree, err := vmStatFreeBytes()
	if err != nil {
		return Memory{}, 0, err
	}
	boot, err := exec.Command("sysctl", "-n", "kern.boottime").Output()
	if err != nil {
		return Memory{}, 0, err
	}
	var sec uint64
	if _, err := fmt.Sscanf(string(boot), "{ sec = %d,", &sec); err != nil {
		return Memory{}, 0, fmt.Errorf("parse kern.boottime: %w", err)
	}
	now, err := commandUint("date", "+%s")
	if err != nil {
		return Memory{}, 0, err
	}
	free := pagesFree
	if free > total {
		free = total
	}
	return Memory{Total: total, Used: total - free, Free: free}, now - sec, nil
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
