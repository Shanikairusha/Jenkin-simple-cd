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

# Run with custom port or config path (env vars PORT / CONFIG_PATH override these flags)
./cd-agent -port 9090 -config /etc/cd-agent/config.yaml
PORT=9090 CONFIG_PATH=/etc/cd-agent/config.yaml ./cd-agent
```

The web/ SPA is embedded into the binary at build time (`//go:embed`), so the dashboard is served at `http://<host>:<port>/` with no extra files to deploy.

On the Linux server: copy `cd-agent-linux`, `config.yaml`, `start.sh`, `stop.sh`. Use `./start.sh` / `./stop.sh` to manage the daemon (PID file at `cd-agent.pid`, logs at `cd-agent.log`).

---

## Architecture

The agent is a single HTTP server exposing the deploy webhook (`POST /api/v1/deploy`), a set of read-only UI API endpoints (`GET /api/v1/ui/...`), and an embedded static SPA served at `/`.

**Package layout:**

```
main.go          – flag/env parsing (-port/PORT, -config/CONFIG_PATH), slog setup, store + deployer wiring,
                   route registration, embedded web/ SPA via //go:embed
config/          – YAML loader; validates api_token and that every project has working_directory
api/             – HTTP layer only: auth middleware, deploy handler, and UI API handlers (ui_handler.go)
deployer/        – deployment orchestration (image pull → gdown → tar load → .env → deploy command)
executor/        – thin wrappers around os/exec
store/           – in-memory, thread-safe deployment history (records + logs) consumed by the UI
web/             – static single-page app (index.html), embedded into the binary
```

**Configuration precedence (main.go):** flags `-port`/`-config` set defaults, then env vars `PORT` / `CONFIG_PATH` override them when set (`getenvOrDefault`).

**Routes** (every `/api/...` route is wrapped in `AuthMiddleware`; `/` is unauthenticated so the login page can load before a token is supplied):

| Method & path | Handler | Purpose |
|---------------|---------|---------|
| `POST /api/v1/deploy` | `DeployHandler` | Webhook that triggers a deployment |
| `GET /api/v1/ui/health` | `UIHealthHandler` | Agent health + uptime since process start |
| `GET /api/v1/ui/deployments` | `UIDeploymentsHandler` | All deployment records + aggregate stats |
| `GET /api/v1/ui/deployments/{id}` | `UIDeploymentDetailHandler` | One record including its log lines |
| `GET /api/v1/ui/projects` | `UIProjectsHandler` | Configured projects and their service names |
| `GET /` | `http.FileServer` | Embedded SPA (`web/index.html`) |

**Request flow:**

1. `AuthMiddleware` validates the `Bearer` token against `config.yaml:api_token`.
2. `DeployHandler` parses the JSON payload, resolves `workDir` and `deployCommand` from config (service-level overrides project-level), fires `go d.Deploy(...)`, immediately returns `202 Accepted`.
3. `deployer.DefaultDeployer.Deploy` first adds a `running` `store.DeployRecord`, then runs these steps sequentially (each step is skipped if its field is empty), appending each step's output to the record's logs, and finally marks the record `success`/`failed`:
   - `image` set → `executor.PullImage` → `docker pull <image>`
   - `gdrive_file_id` set → `executor.DownloadGdown` → `gdown <file_id> -O <tar_path>`
   - `tar_path` set → `executor.LoadTarImage` → `docker load -i <tar_path>`
   - `image` + `service` both set → `updateEnvFile` → writes `SERVICE_IMAGE=<image>` to `<workDir>/.env`
   - always → `executor.RunShellCommand` → `sh -c "<deploy_command>"`

The store passed to `deployer.New` is nil-safe: if `nil`, `Deploy` runs identically but records nothing.

---

## Deployment Methods

### Method 1 — Registry Pull
Send `image` in the payload. Agent runs `docker pull` then the deploy command.

```json
{"project":"pd-qa","service":"pd-discovery-service","image":"sysadminaffiniti/affiniti_dms_ms_discovery_server:pd-revamp-latest"}
```

### Method 2 — Google Drive tar (gdown)
Send `gdrive_file_id` + `tar_path` + `image`. Agent downloads the tar via `gdown`, loads it with `docker load`, updates `.env`, then runs the deploy command. Requires `gdown` installed on the server (`pip install gdown`).

```json
{"project":"pd-qa","service":"pd-ms-auth-service","image":"registry/auth:1.0.0-45","gdrive_file_id":"1A2b3C4d5E6f7G8h9I0jKLmnoPqRst","tar_path":"/tmp/pd-ms-auth-service.tar"}
```

- `image` is required even for gdown/tar deployments so the `.env` version record is updated. The pull step may fail if the server can't reach the private registry — this is non-fatal; the tar provides the actual image.
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

## Deployment History (store package)

`store/store.go` keeps every deployment run in memory (lost on restart — there is no persistence). It is wired in `main.go` via `store.New()` and passed to both the deployer (writes) and the UI handlers (reads).

**Types & functions:**

| Symbol | Signature | Purpose |
|--------|-----------|---------|
| `store.New` | `() *Store` | Create an empty store. |
| `Store.Add` | `(r *DeployRecord)` | Append a record (also indexes it by ID). |
| `Store.Get` | `(id string) *DeployRecord` | Live pointer by ID, or `nil`. |
| `Store.List` | `() []DeployRecord` | Snapshots of all records, **newest first**. |
| `store.GenerateID` | `() string` | Random UUID v4 used as the record ID. |
| `DeployRecord.AppendLog` | `(line string)` | Append one log line (mutex-guarded). |
| `DeployRecord.Complete` | `(success bool, endTime time.Time)` | Set terminal status + end time. |
| `DeployRecord.Snapshot` | `() DeployRecord` | Lock-free copy safe to serialize. |

**Status constants:** `StatusRunning` / `StatusSuccess` / `StatusFailed`.

Concurrency: each `DeployRecord` has its own `sync.Mutex` (guards `Logs`, `Status`, `EndTime`); the `Store` has a `sync.RWMutex` over its slice + index. Always serialize via `Snapshot()` rather than reading fields directly while a deploy goroutine may still be writing.

---

## UI API Handlers (api/ui_handler.go)

Read-only JSON endpoints backing the SPA. All are registered behind `AuthMiddleware` in `main.go`.

| Handler | Constructor | Returns |
|---------|-------------|---------|
| `UIHealthHandler` | `(startTime time.Time) http.Handler` | `{status, uptime_seconds, started_at}` |
| `UIDeploymentsHandler` | `(s *store.Store) http.Handler` | `{deployments[], total, stats{total,success,failed,running,success_rate,avg_duration_seconds}}` |
| `UIDeploymentDetailHandler` | `(s *store.Store) http.Handler` | One record incl. `logs[]`; `404 {"error":"not found"}` for unknown `{id}` |
| `UIProjectsHandler` | `(cfg *config.Config) http.Handler` | `{projects:[{name, services[]}]}`, both sorted alphabetically |

`success_rate` is computed over *finished* deploys only (success / (success+failed)), rounded to one decimal; `avg_duration_seconds` averages only completed records.

---

## Version Tracking (.env file)

`deployer.updateEnvFile(workDir, serviceName, image)` (in `deployer/deployer.go`) maintains `<workDir>/.env`. It is called after tar load and before the deploy command whenever both `req.Image` and `req.Service` are non-empty.

**Key derivation:** `strings.ToUpper(strings.ReplaceAll(serviceName, "-", "_")) + "_IMAGE"`
- `pd-ms-auth-service` → `PD_MS_AUTH_SERVICE_IMAGE`
- `pd-discovery-service` → `PD_DISCOVERY_SERVICE_IMAGE`

Docker Compose reads `.env` automatically. The compose file uses `${VAR:-default}` so the default tag applies if no `.env` entry exists yet (e.g. first deploy or manual `docker compose up`).

The write is atomic: data is written to a temp file in the same directory, then renamed over `.env`, preventing partial writes if the process is interrupted.

**Rollback:** edit the relevant line in `.env`, then `docker compose up -d --no-deps <service>`.

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
