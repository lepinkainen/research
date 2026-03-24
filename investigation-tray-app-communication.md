# Investigation: Sonar Tray App Communication

**Repository:** https://github.com/RasKrebs/sonar
**Date:** 2026-03-24

## Overview

Sonar is a CLI tool (written in Go) for discovering and managing TCP ports listening on localhost. It includes a native macOS menu bar app (written in Swift) called `sonar-tray`.

## How the Tray App Communicates with the Main Binary

The tray app and main binary communicate via **subprocess invocation with JSON over stdout**. There are no sockets, HTTP servers, gRPC, XPC, named pipes, shared memory, or any other IPC mechanism.

### Mechanism: Subprocess + JSON stdout

Every interaction spawns a fresh `sonar` process — there is no persistent connection or daemon.

#### 1. Go launches Swift

Running `sonar tray` (Go CLI command in `internal/cmd/tray.go`) finds and launches the `sonar-tray` Swift binary. It can optionally detach the tray into its own session (`Setsid: true`) so it survives the parent process exiting.

#### 2. Swift polls Go every 2 seconds

The Swift tray app (`tray/SonarTray.swift`) runs a repeating timer that calls:

```swift
runSonar(["list", "--json", "--stats"])
```

This spawns a new `sonar` process, captures stdout via a `Pipe`, and parses the JSON into `SonarPort` structs using `JSONDecoder`.

The core helper function:

```swift
private func runSonar(_ args: [String]) -> String {
    let proc = Process()
    let pipe = Pipe()
    proc.executableURL = URL(fileURLWithPath: sonarPath)
    proc.arguments = args
    proc.standardOutput = pipe
    proc.standardError = FileHandle.nullDevice
    // ...
    try proc.run()
    proc.waitUntilExit()
    // read stdout from pipe
}
```

#### 3. Actions also go through subprocess calls

When a user clicks "Kill Process" in the menu bar:

```swift
runSonar(["kill", String(port.port)])
```

### JSON Contract

The Swift `SonarPort` Decodable struct mirrors the Go `JSONPort` struct (`internal/display/json.go`). Fields include: port, pid, process name, command, user, bind address, CPU %, memory RSS, thread count, uptime, state, connections, Docker container/image/compose info, etc.

### Binary Discovery

The tray app looks for the `sonar` binary:
1. Next to itself (same directory as `sonar-tray`)
2. In `$PATH`

## Architecture Diagram

```
┌─────────────────────┐     subprocess + JSON stdout      ┌──────────────┐
│  sonar-tray (Swift)  │ ────────────────────────────────→ │  sonar (Go)  │
│  macOS menu bar app  │ ←──────────────────────────────── │  CLI tool    │
│  polls every 2s      │  "list --json --stats" / "kill N" │              │
└─────────────────────┘                                    └──────────────┘
```

## Key Files

| File | Purpose |
|------|---------|
| `tray/SonarTray.swift` | macOS menu bar app (Swift) |
| `internal/tray/tray.go` | Go wrapper that launches the Swift tray binary |
| `internal/cmd/tray.go` | CLI `tray` subcommand |
| `internal/cmd/list.go` | Port scanning command |
| `internal/cmd/kill.go` | Process termination command |
| `internal/display/json.go` | JSON output format (the contract) |
| `internal/ports/scan.go` | Port scanning logic (lsof/ss/netstat) |
| `internal/ports/model.go` | Port data structures |

## Efficiency Analysis

The subprocess+JSON approach is simple but **quite wasteful**. Each 2-second poll cycle with `--stats` spawns roughly 20 subprocesses/socket calls and takes an estimated 5-9 seconds on macOS (with 5 ports and 3 Docker containers) — meaning the tray can't even keep up with its own 2-second timer.

### Cost Per Poll Cycle (macOS, 5 ports, 3 Docker containers)

| Step | Subprocesses | Est. Time |
|------|-------------|-----------|
| Go binary startup | 1 (the sonar process) | ~10ms |
| Port scan (`lsof`) | 1 | ~150ms |
| Docker container list (`docker ps`) | 1 | ~300ms |
| Docker stats (socket API, stats + inspect per container) | 6 socket calls | **2-5s** |
| Command lookup (`ps`) | 1 | ~100ms |
| Process stats (`ps`) | 1 | ~300ms |
| Thread count (`ps -M` per port, macOS only) | 5 | ~500ms |
| Connection count (`lsof` per port) | 5 | ~1.5s |
| JSON encode | 0 | ~2ms |
| **Total** | **~20** | **~5-9s** |

### Specific Inefficiencies

1. **Per-port connection counting** (`internal/ports/enrich.go`) — spawns a separate `lsof`/`ss` for each port instead of batching all ports into one call
2. **Per-port thread counting on macOS** — spawns `ps -M` per PID, while Linux already batches this into a single `ps` call using `nlwp`
3. **Docker stats for ALL containers** (`internal/docker/docker.go`) — fetches stats even for containers without listening ports
4. **Docker API CPU sampling** — takes ~1s per container due to how Docker measures CPU usage
5. **Full process startup each time** — Go runtime init, Cobra framework setup, all thrown away after each 2-second cycle

## Improvement Recommendations

### Quick Wins (keep subprocess architecture)

- **Drop `--stats` from the tray poll** — brings each cycle from ~5-9s down to ~500ms. Fetch stats only on-demand when a user expands a port's submenu.
- **Batch connection counting** — one `lsof -iTCP -sTCP:ESTABLISHED -n -P` for all ports, then filter in code
- **Batch thread counting on macOS** — one `ps -M -p PID1,PID2,...` instead of N separate calls

### Medium Effort (architectural changes)

- **Daemon mode with a Unix socket** — run `sonar daemon` that keeps a long-lived process scanning in the background and serves results over a Unix domain socket. The tray app connects once and reads JSON on demand. Eliminates process startup cost and allows caching/incremental updates.
- **Tiered refresh rates** — scan ports every 2s, but refresh stats (CPU, memory, connections) every 10-30s since they change slowly
- **Cache Docker state** — Docker container mappings rarely change; cache them for 30s+ and only refresh stats periodically

### Higher Effort (more sophisticated)

- **Event-driven port detection** — on Linux, use `netlink` sockets to get notified of new listening ports instead of polling. On macOS, `Network.framework` or `proc_info` syscalls could reduce scanning cost.
- **Embed the tray UI in Go** — eliminate the Swift/Go boundary entirely using a Go system tray library (e.g., `fyne`, `systray`), trading native macOS look-and-feel for simplicity.

### Recommended Best Improvement

The **daemon mode with a Unix socket** offers the best return on effort. It would:
- Eliminate ~20 subprocess spawns per cycle
- Allow smart caching (port list: 2s, stats: 10s, Docker mapping: 30s)
- Enable push-based updates instead of polling
- Let the tray connect once and stay connected

```
┌─────────────────────┐     Unix socket (persistent)       ┌──────────────────┐
│  sonar-tray (Swift)  │ ←────────────────────────────────→ │  sonar daemon    │
│  macOS menu bar app  │   JSON messages over socket        │  (Go, long-lived)│
│  connects once       │                                    │  caches results  │
└─────────────────────┘                                     └──────────────────┘
```

## Conclusion

The tray app is **not completely separate** from the main binary — it depends on `sonar` being available and invokes it as a subprocess. However, there is no daemon, no persistent connection, and no shared state beyond the filesystem. The communication is simple and stateless: spawn process → read JSON stdout → update UI.

While this approach is easy to understand and implement, it is inefficient at scale. The single biggest improvement would be introducing a daemon mode with a Unix socket, which would eliminate the per-cycle subprocess overhead and enable intelligent caching of expensive operations like Docker stats.
