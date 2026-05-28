# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

All commands run from the `Jenkin-simple-cd/` directory.

```bash
# Build for local testing
go build -o cd-agent .

# Cross-compile for Linux (from Windows)
.\build-linux.ps1                  # produces cd-agent-linux

# Run all tests
go test ./...

# Run a single test
go test ./api/ -run TestDeployHandler
go test ./api/ -run TestAuthMiddleware

# Run with custom port or config path
./cd-agent -port 9090 -config /etc/cd-agent/config.yaml
```

On the Linux server: copy `cd-agent-linux`, `config.yaml`, `start.sh`, `stop.sh`. Use `./start.sh` / `./stop.sh` to manage the daemon (PID file at `cd-agent.pid`, logs at `cd-agent.log`).

---

## Architecture

The agent is a single HTTP server exposing one authenticated endpoint: `POST /api/v1/deploy`.

**Package layout:**

```
main.go          – flag parsing (-port, -config), slog setup, HTTP server wiring
config/          – YAML loader; validates api_token and that every project has working_directory
api/             – HTTP layer only (auth middleware + request handler)
deployer/        – deployment orchestration (image pull → gdown → tar load → deploy command)
executor/        – thin wrappers around os/exec
```

**Request flow:**

1. `AuthMiddleware` validates the `Bearer` token against `config.yaml:api_token`.
2. `DeployHandler` parses the JSON payload, resolves `workDir` and `deployCommand` from config (service-level overrides project-level), fires `go d.Deploy(...)`, immediately returns `202 Accepted`.
3. `deployer.DefaultDeployer.Deploy` runs these steps sequentially (each step is skipped if its field is empty):
   - `image` set → `executor.PullImage` → `docker pull <image>`
   - `gdrive_file_id` set → `executor.DownloadGdown` → `gdown <file_id> -O <tar_path>`
   - `tar_path` set → `executor.LoadTarImage` → `docker load -i <tar_path>`
   - always → `executor.RunShellCommand` → `sh -c "<deploy_command>"`

---

## Deployment Methods

### Method 1 — Registry Pull
Send `image` in the payload. Agent runs `docker pull` then the deploy command.

```json
{"project":"pd-qa","service":"pd-discovery-service","image":"sysadminaffiniti/affiniti_dms_ms_discovery_server:pd-revamp-latest"}
```

### Method 2 — Google Drive tar (gdown)
Send `gdrive_file_id` + `tar_path`. Agent downloads the tar via `gdown`, loads it with `docker load`, then runs the deploy command. Requires `gdown` installed on the server (`pip install gdown`).

```json
{"project":"pd-qa","service":"pd-ms-auth-service","gdrive_file_id":"1A2b3C4d5E6f7G8h9I0jKLmnoPqRst","tar_path":"/tmp/pd-ms-auth-service.tar"}
```

- `tar_path` is the save location on the server. Defaults to `/tmp/gdrive-download.tar` if omitted.

### Method 3 — Local tar (pre-placed)
Send only `tar_path`. Agent runs `docker load -i <tar_path>` then the deploy command. Use when the tar is already on the server (e.g. via rclone).

```json
{"project":"pd-qa","service":"pd-ms-auth-service","tar_path":"/opt/images/pd-ms-auth-service.tar"}
```

### Project-level deploy (no service)
Omit `service` to trigger the project-level `deploy_command` (e.g. `docker compose up -d` for all services).

```json
{"project":"pd-qa"}
```

---

## Executor Functions

These are the four functions in `executor/executor.go`. All use `exec.Command` directly (no shell interpolation) except `RunShellCommand`.

| Function | Signature | What it runs |
|----------|-----------|--------------|
| `RunCommand` | `(workDir string, cmdArgs []string) error` | Direct exec, no shell. Used for untrusted payload args. |
| `RunShellCommand` | `(workDir string, command string) error` | `sh -c "<command>"`. Only for trusted config values. |
| `PullImage` | `(workDir, image string) error` | `docker pull <image>` |
| `LoadTarImage` | `(workDir, tarPath string) error` | `docker load -i <tarPath>` |
| `DownloadGdown` | `(workDir, fileID, outPath string) error` | `gdown <fileID> -O <outPath>` |

**Command injection boundary:**
- `RunCommand` / `PullImage` / `LoadTarImage` / `DownloadGdown` — safe to call with HTTP payload data; args pass directly to `exec.Command`.
- `RunShellCommand` — **only** for `deploy_command` from `config.yaml`. Never pass payload data here.

---

## Configuration

`config.yaml` is gitignored (contains live API token and server paths). Full YAML structure:

```yaml
api_token: "..."
projects:
  <project-name>:
    working_directory: "/absolute/path"
    deploy_command: "docker compose up -d"       # optional project-level default
    services:
      <service-name>:
        working_directory: "/override/path"      # optional, falls back to project
        deploy_command: "docker compose up -d --no-deps <service-name>"
```

Omitting `service` in the payload triggers project-level execution (requires `deploy_command` at the project level).

---

## Testability

`api.DeployHandler` accepts a `deployer.Deployer` interface, so tests inject a `mockDeployer` without spawning real processes. The mock uses a buffered channel (`done chan struct{}`) to synchronize with the async goroutine in tests.
