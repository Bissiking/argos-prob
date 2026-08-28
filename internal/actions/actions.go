// internal/actions/actions.go
package actions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"
)

// Categories of remote operations exposed by the agent.
const (
	CategoryService   = "service"
	CategoryContainer = "container"
	CategoryProxmox   = "proxmox"
)

// Proxmox guest kinds.
const (
	KindQEMU = "qemu"
	KindLXC  = "lxc"
)

var actionsByCategory = map[string]map[string]bool{
	CategoryService:   {"start": true, "stop": true, "restart": true},
	CategoryContainer: {"start": true, "stop": true, "restart": true},
	CategoryProxmox:   {"start": true, "stop": true, "reboot": true, "shutdown": true},
}

// Policy is the explicit allowlist the master UI advertises: only services,
// containers and virtual machines listed here can be controlled remotely.
// An empty Policy denies everything, so a freshly installed agent stays
// read-only until its operator allows specific targets.
type Policy struct {
	Services   []string
	Containers []string
	VMs        []int
}

// Command is a typed, remote operation. It never carries free-form shell
// input: both the action and the non-proxmox target are validated against
// strict patterns, then passed as fixed argv to a known binary.
type Command struct {
	Category string // service | container | proxmox
	Target   string // unit name, container name (or empty for proxmox)
	Action   string // start | stop | restart, + reboot | shutdown for proxmox
	Kind     string // qemu | lxc for proxmox
	VMID     int    // Proxmox guest id (valid when Category == proxmox)
}

// Allows reports whether the policy explicitly authorizes the command.
func (p Policy) Allows(cmd Command) bool {
	switch cmd.Category {
	case CategoryService:
		return matchAny(cmd.Target, p.Services)
	case CategoryContainer:
		return matchAny(cmd.Target, p.Containers)
	case CategoryProxmox:
		for _, id := range p.VMs {
			if id == cmd.VMID {
				return true
			}
		}
	}
	return false
}

// Controllable mirrors Allows for snapshot flags.
func (p Policy) Controllable(category string, target string, vmid int) bool {
	return p.Allows(Command{Category: category, Target: target, VMID: vmid})
}

func (c Command) Validate() error {
	allowed, ok := actionsByCategory[c.Category]
	if !ok {
		return fmt.Errorf("catégorie inconnue: %q", c.Category)
	}
	if !allowed[c.Action] {
		return fmt.Errorf("action %q non autorisée pour %s", c.Action, c.Category)
	}
	switch c.Category {
	case CategoryService:
		if !validUnit(c.Target) {
			return fmt.Errorf("nom d'unité invalide: %q", c.Target)
		}
	case CategoryContainer:
		if !validName(c.Target) {
			return fmt.Errorf("nom de conteneur invalide: %q", c.Target)
		}
	case CategoryProxmox:
		if c.Kind != KindQEMU && c.Kind != KindLXC {
			return fmt.Errorf("type de machine invalide: %q", c.Kind)
		}
		if c.VMID <= 0 {
			return fmt.Errorf("identifiant de machine invalide: %d", c.VMID)
		}
	}
	return nil
}

// Execute runs the command with a fixed argv (no shell) and returns the
// combined stdout/stderr. The returned error embeds the program's stderr.
func Execute(cmd Command) (string, error) {
	if err := cmd.Validate(); err != nil {
		return "", err
	}
	bin, args, err := cmd.argv()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	process := exec.CommandContext(ctx, bin, args...)
	var buf bytes.Buffer
	process.Stdout = &buf
	process.Stderr = &buf
	if err := process.Run(); err != nil {
		return strings.TrimSpace(buf.String()), fmt.Errorf("%s %s: %w: %s", bin, strings.Join(args, " "), err, strings.TrimSpace(buf.String()))
	}
	return strings.TrimSpace(buf.String()), nil
}

func (c Command) argv() (string, []string, error) {
	switch c.Category {
	case CategoryService:
		return serviceArgv(c.Action, c.Target)
	case CategoryContainer:
		bin, err := exec.LookPath("docker")
		if err != nil {
			return "", nil, errors.New("docker introuvable : contrôles de conteneurs indisponibles sur cet hôte")
		}
		return bin, []string{c.Action, c.Target}, nil
	case CategoryProxmox:
		name := "qm"
		if c.Kind == KindLXC {
			name = "pct"
		}
		bin, err := exec.LookPath(name)
		if err != nil {
			return "", nil, fmt.Errorf("%s introuvable : contrôles Proxmox indisponibles sur cet hôte", name)
		}
		return bin, []string{c.Action, strconv.Itoa(c.VMID)}, nil
	default:
		return "", nil, fmt.Errorf("catégorie inconnue: %q", c.Category)
	}
}

func matchGlob(pattern, target string) (bool, error) {
	return path.Match(pattern, target)
}

func matchAny(target string, patterns []string) bool {
	for _, pattern := range patterns {
		if ok, err := matchGlob(pattern, target); err == nil && ok {
			return true
		}
	}
	return false
}

func validUnit(name string) bool {
	if name == "" || len(name) > 256 {
		return false
	}
	for _, r := range name {
		if !('a' <= r && r <= 'z') && !('A' <= r && r <= 'Z') && !('0' <= r && r <= '9') &&
			r != '.' && r != '_' && r != '@' && r != '-' && r != '+' {
			return false
		}
	}
	return true
}

func validName(name string) bool {
	if name == "" || len(name) > 128 {
		return false
	}
	for i, r := range name {
		if !('a' <= r && r <= 'z') && !('A' <= r && r <= 'Z') && !('0' <= r && r <= '9') &&
			r != '.' && r != '_' && r != '-' {
			return false
		}
		if i == 0 && !('a' <= r && r <= 'z') && !('A' <= r && r <= 'Z') && !('0' <= r && r <= '9') && r != '_' {
			return false
		}
	}
	return true
}
