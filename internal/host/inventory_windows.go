//go:build windows

package host

import (
	"encoding/json"

	"github.com/Bissiking/argos-prob/internal/actions"
)

func collectServices(policy actions.Policy) []ServiceInfo {
	raw, err := runPowerShell(`$services = @(Get-CimInstance Win32_Service | Select-Object Name,DisplayName,State,StartMode,ProcessId); ConvertTo-Json -InputObject $services -Compress`)
	if err != nil {
		return nil
	}
	var services []windowsService
	if err := json.Unmarshal(raw, &services); err != nil {
		return nil
	}
	return windowsServiceInfo(services, policy)
}

func collectDocker(_ actions.Policy) []DockerContainer { return nil }
func collectProxmox(_ actions.Policy) []ProxmoxGuest   { return nil }
func collectVirtualizationIdentity() *VirtualizationIdentity {
	return &VirtualizationIdentity{Kind: "unknown"}
}
func collectSockets() []NetworkSocket { return nil }
