# Argos Prob

**Argos Prob** is the lightweight, cross-platform host agent for Argos.

The agent is designed to keep working even when optional components such as Docker, Proxmox or a service manager are unavailable. The Argos server API is **not required yet**: the current development version can initialize itself and expose a local inventory through its CLI.

## Supported targets

The architecture targets:

- Windows 10 / 11
- Windows Server
- Linux, including Debian and Ubuntu
- Proxmox VE hosts
- macOS Intel
- macOS Apple Silicon

Argos Prob itself does not require Docker.

## Quick start

```bash
go build -o argos-prob ./cmd/argos-prob
./argos-prob init
./argos-prob doctor
./argos-prob status
```

On Windows:

```powershell
go build -o argos-prob.exe ./cmd/argos-prob
.\argos-prob.exe init
.\argos-prob.exe doctor
.\argos-prob.exe status
```

## Commands

| Command | Purpose |
| --- | --- |
| `init` | Creates a local agent identity and configuration |
| `status` | Prints the current host inventory as JSON |
| `doctor` | Checks the local installation and capabilities |
| `version` | Prints the agent version |

## Configuration

Default paths:

- Linux as root: `/etc/argos-prob/config.json`
- Linux/macOS as a regular user: the OS user configuration directory
- Windows: `%ProgramData%\\ArgosProb\\config.json`

For development, override the path with `ARGOS_PROB_CONFIG`.

The configuration already contains optional `endpoint` and `token` fields so the future Argos API can be connected without redesigning the agent.

## Safety principles

- No arbitrary remote shell execution.
- Docker and Proxmox are optional capabilities, not hard dependencies.
- A missing provider must not prevent core host inventory from working.
- Remote actions will use an explicit allowlist.
- Read-only inventory comes before remote administration.
- Failures must be isolated per provider.

## Current development scope

Version `0.1.0-dev` provides:

- persistent random agent identity
- OS / architecture / hostname detection
- CPU count
- RAM usage
- uptime
- network interfaces and addresses
- Docker capability detection
- systemd detection on Linux
- Proxmox detection on Linux
- Windows Services capability flag
- launchd detection on macOS
- local diagnostics
- JSON inventory output

Next steps are native service installation, richer disk/service inventory, provider isolation, local structured logs and the future authenticated transport to Argos.
