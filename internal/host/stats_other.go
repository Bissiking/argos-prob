// internal/host/stats_other.go
//go:build !linux && !darwin && !windows

package host

import "fmt"

func platformStats() (Memory, uint64, error) {
	return Memory{}, 0, fmt.Errorf("platform metrics are not implemented for this operating system")
}
