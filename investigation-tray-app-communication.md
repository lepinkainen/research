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

## Conclusion

The tray app is **not completely separate** from the main binary — it depends on `sonar` being available and invokes it as a subprocess. However, there is no daemon, no persistent connection, and no shared state beyond the filesystem. The communication is simple and stateless: spawn process → read JSON stdout → update UI.
