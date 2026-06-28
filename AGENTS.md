# Project: spawn-flowise

## Overview
`spawn-flowise` is a cross-platform CLI tool written in Go designed to orchestrate the deployment and management of multiple, isolated [FlowiseAI](https://flowiseai.com/) instances using Docker or Podman. It creates self-contained environments with their own data volumes, networks, and port mappings, optimized for high-density hosting and private network deployments.

## Technical Stack
- **Language**: Go (1.22+)
- **Containerization**: Docker, Docker Compose (supports Podman & podman-compose)
- **Configuration**: `.env` files, `docker-compose.yml`
- **Networking**: `0.0.0.0` binding for all interfaces (Private Network compatible)

## Key Components

### 1. CLI Entry Point (`main.go`)
The root entry point that dispatches commands to the `cmd` package and handles robust flag parsing for positional and named arguments.

### 2. Modular Architecture
The application follows a strict modular design for maintainability and portability:
- **`cmd/`**: Command implementations (Spawn, Stop, Hold, Unhold, Cleanup).
- **`internal/container/`**: Abstraction layer for Docker/Podman engines, including cross-platform socket management.
- **`internal/system/`**: Platform-specific resource discovery (RAM, Port availability) using Go build tags.
- **`internal/lock/`**: Portable file-based concurrency control.
- **`internal/config/`**: Centralized naming conventions (`InstanceInfo`), defaults, and environment file operations.
- **`internal/utils/`**: Shared helpers, filesystem scanning, and secure `sudo` wrappers.

### 3. Service Template (`docker-compose.yml`)
Acts as a dynamic template using environment variable substitution for isolation:
- **Service Name**: `flowise-instance-NN`.
- **Ports**: Maps unique host ports to internal ports, binding to `0.0.0.0`.
- **Volumes**: Persistent mapping of host directories to container storage.
- **Healthchecks**: Configured to probe `0.0.0.0` for compatibility.

## Workflow
1.  **Initialization**: Run `./flowise-spawn check` to validate engine reachability and host resources.
2.  **Spawning**:
    - Users run `./flowise-spawn spawn <N>`.
    - Instance numbering is **0-based**, so `spawn N` creates instances `00` through `N-1` (e.g., `spawn 2` creates `flowise-instance-00` and `flowise-instance-01`).
    - CLI performs memory checks using a **1024 MB per-instance reservation** (no hard container memory limit is currently enforced).
    - Instances are round-robin assigned to a pool of Docker networks (`flowise-default-XX`).
    - Sequential spawning (30s delay) prevents host resource exhaustion.
3.  **Stop**:
    - `stop all` halts all `flowise-instance-NN` containers, removes them, and tears down their `flowise-default-XX` networks so subsequent `spawn` commands start cleanly.
    - `stop <N>` halts only `flowise-instance-<N-1>` (instance numbering is 0-based), removes that container, and attempts to remove its network.
4.  **Persistence**: Data is stored in `~/.flowiseNN`, ensuring state survives restarts.
5.  **Hold/Unhold**:
    - **Hold**: Gracefully stops instances and renames data directories to `~/.bkpflowiseNN` using secure move operations.
    - **Unhold**: Restores directories to allow respawning.
6.  **Cleanup**:
    - Full system reset: stops and removes all containers and their `flowise-default-XX` networks, then archives all active instance data (`~/.flowiseNN`) into a single gzip-compressed tar archive at `~/flowise_backup/flowise_backup_<timestamp>.tar.gz` using `sudo tar`.
    - After archiving, the active data directories are securely removed.
    - Held data directories (`~/.bkpflowiseNN`) are intentionally preserved so they can later be restored with `unhold`.

## Security & Reliability
- **Symlink Protection**: Validates all paths before destructive or move operations.
- **Context Timeouts**: Critical system operations (like backups) use `context.WithTimeout` to prevent hanging.
- **Command Injection Prevention**: Uses argument separators (`--`) in all shell invocations.
- **Portability**: Uses platform-specific implementations for Linux, macOS, and Windows.
