// internal/host/inventory_other.go
//go:build !linux

package host

func collectServices() []ServiceInfo {
	return nil
}

func collectDocker() []DockerContainer {
	return nil
}

func collectProxmox() []ProxmoxGuest {
	return nil
}
