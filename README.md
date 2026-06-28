# spawn-flowise

`spawn-flowise` is a cross-platform CLI tool written in Go that orchestrates the deployment and management of multiple, isolated [FlowiseAI](https://flowiseai.com/) instances using Docker or Podman. Each instance gets its own data volume, container, host port, and shared network — making it easy to run many Flowise environments on a single host.

---

## Overview

FlowiseAI is a powerful drag-and-drop LLM workflow builder. When you need to host several independent Flowise environments — for different teams, customers, or experiments — `spawn-flowise` automates the repetitive work:

- Assigns a unique host port to every instance.
- Creates isolated data directories under your home folder.
- Distributes instances across a pool of Docker/Podman networks.
- Provides lifecycle commands for start, stop, hold, unhold, and cleanup.
- Uses a single `docker-compose.yml` template as the basis for every instance.

Instances are numbered **0-based**. Running `spawn-flowise spawn 3` creates `flowise-instance-00`, `flowise-instance-01`, and `flowise-instance-02`.

---

## Features

- **Engine agnostic** — works with Docker or Podman (`docker compose` / `podman compose`).
- **Isolated instances** — each instance has its own container, volume, port, and environment file.
- **High-density hosting** — round-robin assignment across a pool of networks (`flowise-default-01` … `flowise-default-04`).
- **Cross-platform** — builds and runs on Linux, macOS, and Windows.
- **Sequential spawning** — 30-second delays between instances to avoid resource exhaustion.
- **Hold / unhold** — temporarily park instance data without deleting it.
- **Safe cleanup** — archives active data to a timestamped `.tar.gz` before removal.
- **Security minded** — validates paths before destructive operations, refuses symlinks, and uses argument separators in shell invocations.

---

## Requirements

- **Go 1.22+** (only if building from source).
- **Docker** or **Podman** installed and available in `PATH`.
- **Docker Compose** or **Podman Compose** plugin installed.
- A user account with permission to run the container engine.
- **RAM**: `spawn-flowise` checks that roughly **1 GB of RAM per requested instance** is available. This is a spawn-time reservation, not a hard container memory limit.
- **Ports**: host ports starting at `3001` must be free for each instance you spawn.

---

## Installation

### Build from source

```bash
git clone https://github.com/spawn-flowise/spawn-flowise.git
cd spawn-flowise
go build -o flowise-spawn .
```

### Use the prebuilt binary

Download the `flowise-spawn` binary for your platform and place it in a directory on your `PATH`, then make it executable:

```bash
chmod +x flowise-spawn
```

---

## Quick Start

1. **Check that your environment is ready:**

   ```bash
   ./flowise-spawn check
   ```

2. **Spawn two isolated Flowise instances:**

   ```bash
   ./flowise-spawn spawn 2
   ```

   This creates `flowise-instance-00` on port `3001` and `flowise-instance-01` on port `3002`.

3. **Wait for the instances to be healthy**, then open your browser:

   - http://localhost:3001
   - http://localhost:3002

4. **Stop all instances when you are done:**

   ```bash
   ./flowise-spawn stop all
   ```

---

## Command Reference

All commands share the global flag `-engine` to select the container engine:

```bash
./flowise-spawn -engine podman check
```

If omitted, `docker` is used.

| Command | Description |
|--------|-------------|
| `check` | Validate engine reachability, total RAM, and base port availability. |
| `spawn <N>` | Create and start `N` isolated Flowise instances (0-based numbering). |
| `stop all` | Stop and remove all `flowise-instance-NN` containers and their networks. |
| `stop <N>` | Stop and remove `flowise-instance-<N-1>` only. |
| `hold` | Stop instances and rename data directories from `~/.flowiseNN` to `~/.bkpflowiseNN`. |
| `unhold` | Restore held data directories from `~/.bkpflowiseNN` back to `~/.flowiseNN`. |
| `cleanup` | Remove all containers and networks, archive active data to `~/flowise_backup/`, then delete it. Held data is preserved. |

---

## How-to Guides

### Spawn multiple instances

```bash
./flowise-spawn spawn 5
```

This creates instances `00` through `04` on host ports `3001`–`3005`. A 30-second pause is inserted between each instance, and a final 30-second stabilization wait occurs at the end.

### Stop and restart cleanly

`stop all` removes all containers and networks but leaves data intact:

```bash
./flowise-spawn stop all
```

To stop a single instance, pass its 1-based index (`stop <N>` stops `flowise-instance-<N-1>`):

```bash
# Stop only flowise-instance-00
./flowise-spawn stop 1
```

Later, you can spawn again. Existing data directories are reused, so state survives the stop.

### Hold and unhold data

Use `hold` to park instance data without deleting it. This is useful for maintenance or freeing ports:

```bash
./flowise-spawn hold
```

Data is moved from `~/.flowiseNN` to `~/.bkpflowiseNN`. To restore:

```bash
./flowise-spawn unhold
```

### Clean up and archive data

`cleanup` performs a full reset:

1. Removes all `flowise-instance-NN` containers.
2. Removes all `flowise-default-XX` networks.
3. Archives active `~/.flowiseNN` directories into `~/flowise_backup/flowise_backup_<timestamp>.tar.gz`.
4. Deletes the archived data directories.
5. Leaves held directories (`~/.bkpflowiseNN`) untouched.

```bash
./flowise-spawn cleanup
```

### Use Podman instead of Docker

Either set the engine flag per command:

```bash
./flowise-spawn -engine podman check
./flowise-spawn -engine podman spawn 2
```

Or create an alias/script that always passes `-engine podman`.

---

## Configuration

`spawn-flowise` looks for a `docker-compose.yml` file in the current working directory and uses it as a template for every instance. The following substitutions are applied automatically:

- Service name `flowise` → `flowise-instance-NN`.
- Port mapping is rewritten to bind on `0.0.0.0` for private-network compatibility.

Each instance receives its own environment file at `~/.flowise-spawn/env/flowise-instance-NN.env`. Key variables include:

| Variable | Default / Value | Purpose |
|----------|-----------------|---------|
| `PORT` | `3000` | Internal container port. |
| `HOST_PORT` | `3001 + N` | External host port mapped to the instance. |
| `CONTAINER_NAME` | `flowise-instance-NN` | Container name. |
| `HOST_PATH` | `~/.flowiseNN` | Host data directory. |
| `CONTAINER_PATH` | `/root/.flowise` | Container data directory. |
| `NETWORK_NAME` | `flowise-default-XX` | Docker/Podman network name. |

You can customize the base `docker-compose.yml` template, but keep the `${...}` placeholders so the CLI can substitute instance-specific values.

### Hard-coded defaults

The following values are baked into the CLI:

- Base host port: `3001`
- Internal container port: `3000`
- Network pool size: `4` networks
- Spawn delay: `30` seconds
- Spawn memory reservation: `1024` MB per instance
- Default engine: `docker`
- Container image: `flowiseai/flowise:latest`

> **Security note:** The default JWT and token secrets in the generated `.env` files are intended for local/testing use. For production deployments, generate your own secrets and update the environment files before spawning.

---

## Architecture & Data Persistence

### Directory layout

| Path | Purpose |
|------|---------|
| `~/.flowiseNN` | Active data directory for instance `NN`. |
| `~/.bkpflowiseNN` | Held data directory for instance `NN`. |
| `~/.flowise-spawn/env/` | Generated per-instance `.env` files. |
| `~/.flowise-spawn/compose/` | Generated per-instance compose files. |
| `~/flowise_backup/` | Compressed archives produced by `cleanup`. |

### Networking

Instances are distributed across four networks to keep each network from growing too large:

- `flowise-default-01`
- `flowise-default-02`
- `flowise-default-03`
- `flowise-default-04`

Instance `N` is assigned to network `(N % 4) + 1`.

### Port numbering

Instance `N` listens externally on port `3001 + N`. For example:

| Instance | Host port |
|----------|-----------|
| `flowise-instance-00` | `3001` |
| `flowise-instance-01` | `3002` |
| `flowise-instance-02` | `3003` |

All ports bind to `0.0.0.0` so the instances are reachable from other hosts on the same private network.

---

## Security Considerations

- **Symlink protection**: The CLI refuses to operate on paths that are symlinks before move, archive, or remove operations.
- **Argument separators**: Shell invocations use `--` to separate options from operands, reducing command-injection risk.
- **Sudo for destructive cleanup**: `cleanup` uses `sudo tar` and `sudo rm -rf` to preserve permissions and remove potentially root-owned container data. Ensure you understand the implications before running it.
- **Default secrets**: Replace the built-in JWT secrets, session secret, and token hash secret before any production use.

---

## Troubleshooting

### `check` fails with “base port 3001 is already in use”

Another service is listening on port `3001`. Either stop that service or choose a different base port by editing the source constant `BasePort` and rebuilding, or by adjusting the compose template if you are not relying on the CLI’s port numbering.

### `spawn` fails partway through

- Check that all ports from `3001` to `3001 + N - 1` are free.
- Verify the engine is running (`docker version` or `podman version`).
- Ensure you have enough RAM: `spawn` requires ~1 GB per instance.

### Containers are unhealthy

Flowise can take a minute or more to start on slower hosts. The compose healthcheck waits 30 seconds and retries 5 times. If containers remain unhealthy, inspect logs:

```bash
docker logs flowise-instance-00
```

### Permission denied during cleanup

`cleanup` uses `sudo` for archive and removal operations. Make sure your user can run `sudo` non-interactively, or run the cleanup manually.

---

## Development

### Project layout

```
spawn-flowise/
├── main.go                       # CLI entry point
├── cmd/                          # Command implementations
├── internal/config/              # Naming conventions, defaults, .env handling
├── internal/container/           # Docker/Podman abstraction
├── internal/lock/                # File-based concurrency lock
├── internal/system/              # RAM and port checks (platform-specific)
├── internal/utils/               # Filesystem helpers and secure sudo wrappers
└── docker-compose.yml            # Base compose template
```

### Run tests

```bash
go test ./...
```

### Build

```bash
go build -o flowise-spawn .
```

---

## License

This project is licensed under the [Apache License 2.0](LICENSE).

Copyright 2026 Vijay Menon.
