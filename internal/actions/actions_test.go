package actions

import "testing"

func TestPolicyAllows(t *testing.T) {
	policy := Policy{
		Services:   []string{"nginx.service", "postgresql@*.service", "backup-*.service"},
		Containers: []string{"nextcloud*", "prometheus"},
		VMs:        []int{100, 104},
	}
	cases := []struct {
		name string
		cmd  Command
		want bool
	}{
		{"service exact", Command{Category: CategoryService, Target: "nginx.service"}, true},
		{"service glob instantiated", Command{Category: CategoryService, Target: "postgresql@14-main.service"}, true},
		{"service glob étoile", Command{Category: CategoryService, Target: "backup-nightly.service"}, true},
		{"service non listé", Command{Category: CategoryService, Target: "sshd.service"}, false},
		{"conteneur glob", Command{Category: CategoryContainer, Target: "nextcloud-db"}, true},
		{"conteneur exact", Command{Category: CategoryContainer, Target: "prometheus"}, true},
		{"conteneur refusé", Command{Category: CategoryContainer, Target: "jellyfin"}, false},
		{"vm autorisée", Command{Category: CategoryProxmox, VMID: 104}, true},
		{"vm non autorisée", Command{Category: CategoryProxmox, VMID: 101}, false},
	}
	for _, tc := range cases {
		if got := policy.Allows(tc.cmd); got != tc.want {
			t.Errorf("%s: Allows(%+v) = %v, want %v", tc.name, tc.cmd, got, tc.want)
		}
	}
	var empty Policy
	if empty.Allows(Command{Category: CategoryService, Target: "nginx.service"}) {
		t.Fatal("une politique vide doit tout refuser")
	}
}

func TestCommandValidate(t *testing.T) {
	valid := []Command{
		{Category: CategoryService, Target: "nginx.service", Action: "restart"},
		{Category: CategoryService, Target: "postgresql@14-main.service", Action: "stop"},
		{Category: CategoryContainer, Target: "nextcloud-db", Action: "start"},
		{Category: CategoryProxmox, Kind: KindQEMU, VMID: 100, Action: "shutdown"},
		{Category: CategoryProxmox, Kind: KindLXC, VMID: 104, Action: "reboot"},
	}
	for _, cmd := range valid {
		if err := cmd.Validate(); err != nil {
			t.Errorf("Validate(%+v) inattendu: %v", cmd, err)
		}
	}
	invalid := []Command{
		{Category: CategoryService, Target: "nginx; rm -rf /", Action: "restart"},
		{Category: CategoryService, Target: "nginx.service", Action: "purge"},
		{Category: CategoryContainer, Target: "../../etc", Action: "start"},
		{Category: CategoryProxmox, Kind: "hv", VMID: 100, Action: "start"},
		{Category: CategoryProxmox, Kind: KindQEMU, VMID: -3, Action: "start"},
		{Category: "inconnue", Target: "x", Action: "start"},
	}
	for _, cmd := range invalid {
		if err := cmd.Validate(); err == nil {
			t.Errorf("Validate(%+v) aurait dû échouer", cmd)
		}
	}
}

func TestValidNameAndUnit(t *testing.T) {
	for _, name := range []string{"a", "1abc_.-"} {
		if !validName(name) {
			t.Errorf("validName(%q) aurait dû passer", name)
		}
	}
	for _, name := range []string{"with space", "-", ""} {
		if validName(name) {
			t.Errorf("validName(%q) aurait dû échouer", name)
		}
	}
	for _, unit := range []string{"nginx.service", "postgresql@14.service"} {
		if !validUnit(unit) {
			t.Errorf("validUnit(%q) aurait dû passer", unit)
		}
	}
	for _, unit := range []string{"a/b", "unit;x", ""} {
		if validUnit(unit) {
			t.Errorf("validUnit(%q) aurait dû échouer", unit)
		}
	}
}
