// internal/host/stats_linux.go
//go:build linux

package host

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func platformStats() (Memory, uint64, error) {
	mem, err := linuxMemory()
	if err != nil {
		return Memory{}, 0, err
	}
	raw, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return Memory{}, 0, err
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return Memory{}, 0, fmt.Errorf("invalid /proc/uptime")
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return Memory{}, 0, err
	}
	return mem, uint64(seconds), nil
}

func linuxMemory() (Memory, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return Memory{}, err
	}
	defer f.Close()

	values := map[string]uint64{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		v, err := strconv.ParseUint(fields[1], 10, 64)
		if err == nil {
			values[key] = v * 1024
		}
	}
	if err := scanner.Err(); err != nil {
		return Memory{}, err
	}
	total := values["MemTotal"]
	free := values["MemAvailable"]
	if total == 0 {
		return Memory{}, fmt.Errorf("MemTotal missing from /proc/meminfo")
	}
	return Memory{Total: total, Used: total - free, Free: free}, nil
}
