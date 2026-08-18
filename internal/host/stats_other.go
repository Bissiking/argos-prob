// internal/host/stats_other.go
//go:build !linux && !darwin && !windows

package host

import "fmt"

func collectPlatform() (platformSnapshot, error) {
	return platformSnapshot{}, fmt.Errorf("platform metrics are not implemented for this operating system")
}
