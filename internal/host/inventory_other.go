// internal/host/inventory_other.go
//go:build !linux

package host

import "github.com/Bissiking/argos-prob/internal/actions"

func collectServices(_ actions.Policy) []ServiceInfo {
	return nil
}

func collectDocker(_ actions.Policy) []DockerContainer {
	return nil
}

func collectProxmox(_ actions.Policy) []ProxmoxGuest {
	return nil
}
