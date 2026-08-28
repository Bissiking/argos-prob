package host

import (
	"math"
	"testing"

	"github.com/Bissiking/argos-prob/internal/actions"
	"github.com/Bissiking/argos-prob/internal/capabilities"
)

func TestSnapshotCapabilitiesExposeWindowsServicesToCurrentMaster(t *testing.T) {
	got := snapshotCapabilities(capabilities.Capabilities{OS: "windows", WindowsServices: true})
	if !got.WindowsServices {
		t.Fatal("la capacité Windows Services doit être publiée explicitement")
	}
	if !got.Systemd {
		t.Fatal("le master actuel utilise systemd comme capacité générique pour afficher les services")
	}
}

func TestWindowsPlatformValues(t *testing.T) {
	mem, uptime, cpu, disks := windowsPlatformValues(windowsSnapshotPayload{
		TotalVisibleMemorySize: 16 * 1024 * 1024,
		FreePhysicalMemory:     4 * 1024 * 1024,
		UptimeSeconds:          3600,
		CPUUsage:               37.5,
		Disks: []windowsDisk{
			{DeviceID: "C:", FileSystem: "NTFS", Size: 1000, FreeSpace: 250},
			{DeviceID: "Z:", FileSystem: "NTFS", Size: 0},
		},
	})
	if mem.Total != 16*1024*1024*1024 || mem.Used != 12*1024*1024*1024 {
		t.Fatalf("mémoire Windows inattendue: %+v", mem)
	}
	if uptime != 3600 || cpu.Usage != 37.5 || cpu.Cores < 1 {
		t.Fatalf("CPU/uptime Windows inattendus: uptime=%d cpu=%+v", uptime, cpu)
	}
	if len(disks) != 1 || disks[0].Mount != `C:\` || disks[0].Used != 750 || math.Abs(disks[0].Usage-75) > 0.001 {
		t.Fatalf("disques Windows inattendus: %+v", disks)
	}
}

func TestWindowsServiceInfo(t *testing.T) {
	services := windowsServiceInfo([]windowsService{
		{Name: "Spooler", DisplayName: "Print Spooler", State: "Running", StartMode: "Auto", ProcessID: 4242},
		{Name: "WSearch", DisplayName: "Windows Search", State: "Stopped", StartMode: "Disabled"},
	}, actions.Policy{Services: []string{"Spool*"}})
	if len(services) != 2 {
		t.Fatalf("nombre de services inattendu: %d", len(services))
	}
	if services[0].ActiveState != "active" || !services[0].Controllable || services[0].PID == nil || *services[0].PID != 4242 {
		t.Fatalf("service actif inattendu: %+v", services[0])
	}
	if services[1].ActiveState != "inactive" || services[1].Controllable || services[1].PID != nil {
		t.Fatalf("service arrêté inattendu: %+v", services[1])
	}
}

func TestSnapshotCapabilitiesPreserveOptionalProviders(t *testing.T) {
	got := snapshotCapabilities(capabilities.Capabilities{Systemd: true, Docker: true, Proxmox: true})
	if !got.Systemd || !got.Docker || !got.Proxmox || got.WindowsServices {
		t.Fatalf("capacités inattendues: %+v", got)
	}
}
