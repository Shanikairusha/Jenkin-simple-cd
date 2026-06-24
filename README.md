# Simple CD Agent

A lightweight webhook agent written in Go that listens for deployment webhooks (GitLab CI, Jenkins, or any HTTP client) and executes Docker commands to deploy microservices on your server.

---

## Quick Start

### 1. Build

**Linux binary (from Windows):**
```powershell
.\build-linux.ps1        # produces cd-agent-linux
```

**Local test build:**
```bash
go build -o cd-agent .
```

**Docker image:**
```bash
docker build -t cd-agent .
```

### 2. Configure

Create `config.yaml` next to the binary (see [Configuration](#configuration) below).

### 3. Deploy to Linux server

Copy these four files to your server (e.g. `/opt/cd-agent/`):
```
cd-agent-linux
config.yaml
start.sh
stop.sh
```

```bash
chmod +x cd-agent-linux start.sh stop.sh
./start.sh          # starts daemon, logs → cd-agent.log, PID → cd-agent.pid
./stop.sh           # stops the daemon
```

The agent listens on port `8080` by default. Override with the `-port` / `-config` flags or the `PORT` / `CONFIG_PATH` environment variables (env vars take precedence):

```bash
./cd-agent-linux -port 9090 -config /etc/cd-agent/config.yaml
PORT=9090 CONFIG_PATH=/etc/cd-agent/config.yaml ./cd-agent-linux
```

---

## Configuration

`config.yaml` structure:

```yaml
api_token: "your-super-secret-token"

projects:
  <project-name>:
    working_directory: "/absolute/path/to/docker-compose-dir"
    deploy_command: "docker compose up -d"   # optional project-level fallback
    services:
      <service-name>:
        working_directory: "/override/path"  # optional, falls back to project level
        deploy_command: "docker compose up -d --no-deps <service-name>"
```

- `api_token` — Bearer token that all webhook callers must send.
- `working_directory` — Directory where `docker compose` commands run.
- `deploy_command` — Shell command executed after any image pull/load steps. Always sourced from this file (never from the webhook payload).
- Omitting `service` in the payload triggers the project-level `deploy_command`.

---

## Deployment Methods

The agent supports three image delivery methods. They are **not mutually exclusive** — all steps are checked in order per request:

```
1. image         →  docker pull <image>
2. gdrive_file_id + tar_path  →  gdown <file_id> -O <tar_path>  then  docker load -i <tar_path>
3. tar_path only →  docker load -i <tar_path>   (tar already on server, e.g. via rclone)
4. .env updated  →  <workDir>/.env: SERVICE_IMAGE=<image>  (version tracking, only if image + service both set)
5. deploy_command (always runs last)
```

### Method 1 — Registry Pull

CI pushes the image to a registry; the agent pulls it on the server.

**Webhook payload:**
```json
{
  "project": "pd-qa",
  "service": "pd-discovery-service",
  "image": "sysadminaffiniti/affiniti_dms_ms_discovery_server:1.0.0-45"
}
```

**What the agent does:**
1. `docker pull sysadminaffiniti/affiniti_dms_ms_discovery_server:1.0.0-45`
2. Writes `PD_DISCOVERY_SERVICE_IMAGE=sysadminaffiniti/affiniti_dms_ms_discovery_server:1.0.0-45` to `<workDir>/.env`
3. Runs `deploy_command` from config

---

### Method 2 — Google Drive tar download (gdown)

Use this when images come from a private source or you want to avoid registry credentials on the deploy VM. CI saves the image as a `.tar`, uploads it to Google Drive, then tells the agent where to fetch it.

**Requires `gdown` on the server:**
```bash
pip install gdown
```

**Webhook payload:**
```json
{
  "project": "pd-qa",
  "service": "pd-ms-auth-service",
  "image": "affiniti-crm.duckdns.org:8080/sysadminaffiniti/affiniti_dms_ms_auth_service:1.0.0-45",
  "gdrive_file_id": "1A2b3C4d5E6f7G8h9I0jKLmnoPqRst",
  "tar_path": "/tmp/pd-ms-auth-service.tar"
}
```

**What the agent does:**
1. `docker pull ...` — may fail if server can't reach the private registry; error is logged and execution continues
2. `gdown 1A2b3C4d5E6f7G8h9I0jKLmnoPqRst -O /tmp/pd-ms-auth-service.tar`
3. `docker load -i /tmp/pd-ms-auth-service.tar`
4. Writes `PD_MS_AUTH_SERVICE_IMAGE=affiniti-crm.duckdns.org:8080/.../auth:1.0.0-45` to `<workDir>/.env`
5. Runs `deploy_command` from config

> Always include `image` in the payload even for gdown/tar deployments — it's used to update the `.env` version record for rollback. The actual image comes from the tar; the pull failure is harmless.

> `tar_path` is where the tar is saved on the server. Defaults to `/tmp/gdrive-download.tar` if omitted.

**GitLab CI — save and upload the tar:**
```yaml
deploy:
  stage: deploy
  script:
    # Build with a versioned tag using CI pipeline number
    - IMAGE="affiniti-crm.duckdns.org:8080/sysadminaffiniti/affiniti_dms_ms_auth_service:1.0.0-${CI_PIPELINE_IID}"
    - docker build -t "$IMAGE" .
    - docker save "$IMAGE" -o auth-service.tar
    # Upload to Google Drive, capture the file ID
    - GDRIVE_FILE_ID=$(gdrive files upload auth-service.tar --parent <FOLDER_ID> --json | jq -r '.id')
    # Trigger the CD agent
    - |
      curl -sf -X POST http://<VM_IP>:8080/api/v1/deploy \
        -H "Authorization: Bearer your-super-secret-token" \
        -H "Content-Type: application/json" \
        -d "{
              \"project\": \"pd-qa\",
              \"service\": \"pd-ms-auth-service\",
              \"image\": \"$IMAGE\",
              \"gdrive_file_id\": \"$GDRIVE_FILE_ID\",
              \"tar_path\": \"/tmp/pd-ms-auth-service.tar\"
            }"
```

---

### Method 3 — Local tar (pre-placed by rclone or other tool)

The tar is already on the server (e.g. synced via rclone). Just pass `tar_path` without `gdrive_file_id`.

**Webhook payload:**
```json
{
  "project": "pd-qa",
  "service": "pd-ms-auth-service",
  "tar_path": "/opt/images/pd-ms-auth-service.tar"
}
```

**What the agent does:**
1. `docker load -i /opt/images/pd-ms-auth-service.tar`
2. Runs `deploy_command` from config

---

## Version Tracking & Rollback

Every time the agent deploys a service with an `image` field in the payload, it writes a `KEY=VALUE` line to `<working_directory>/.env`. Docker Compose reads this file automatically, so the correct versioned image is used on every subsequent `docker compose up`.

**Key naming convention:** service name → uppercase + underscores + `_IMAGE`
- `pd-ms-auth-service` → `PD_MS_AUTH_SERVICE_IMAGE`
- `pd-discovery-service` → `PD_DISCOVERY_SERVICE_IMAGE`

**Example `.env` after several deployments:**
```
PD_DISCOVERY_SERVICE_IMAGE=sysadminaffiniti/affiniti_dms_ms_discovery_server:1.0.0-45
PD_MS_AUTH_SERVICE_IMAGE=affiniti-crm.duckdns.org:8080/sysadminaffiniti/affiniti_dms_ms_auth_service:1.2.1-89
PD_CRM_REACT_FRONTEND_IMAGE=affiniti-crm.duckdns.org:8080/sysadminaffiniti/affiniti_crm_react_front_end:2.0.0-102
```

**The `pd-qa-docker-compose.yml` uses `${VAR:-default}` substitution:**
```yaml
image: ${PD_MS_AUTH_SERVICE_IMAGE:-affiniti-crm.duckdns.org:8080/sysadminaffiniti/affiniti_dms_ms_auth_service:pd-revamp-latest}
```
If the `.env` entry is missing (first deploy), the default tag is used.

### Rolling back a service

```bash
# On the VM, in the docker-compose directory
nano .env
# Change: PD_MS_AUTH_SERVICE_IMAGE=...auth:1.2.1-89
# To:     PD_MS_AUTH_SERVICE_IMAGE=...auth:1.1.0-72

docker compose up -d --no-deps pd-ms-auth-service
# Docker Compose detects the image change and recreates the container
```

---

## Web Dashboard

The agent serves a single-page dashboard at `http://<VM_IP>:8080/`. The SPA is embedded into the binary (`//go:embed web`), so nothing extra needs to be copied to the server. The page is served unauthenticated (so it can load a login form), but every data endpoint it calls requires the same `Bearer` token as the webhook.

### UI API endpoints

All require `Authorization: Bearer <api_token>` and return JSON.

| Method & path | Returns |
|---------------|---------|
| `GET /api/v1/ui/health` | `{ "status": "healthy", "uptime_seconds": 1234, "started_at": "..." }` |
| `GET /api/v1/ui/deployments` | All deployment records (newest first) plus aggregate `stats` (total, success, failed, running, `success_rate`, `avg_duration_seconds`) |
| `GET /api/v1/ui/deployments/{id}` | A single deployment record including its captured `logs` (`404` if the ID is unknown) |
| `GET /api/v1/ui/projects` | Configured projects and their service names, alphabetically sorted |

**Example:**
```bash
curl -s http://<VM_IP>:8080/api/v1/ui/deployments \
  -H "Authorization: Bearer your-super-secret-token" | jq
```

> Deployment history is kept **in memory only** — it is cleared whenever the agent restarts. Each `POST /api/v1/deploy` creates one record that starts as `running`, accumulates per-step logs (pull/gdown/load/env/deploy), and ends as `success` or `failed`.

---

## Calling from GitLab CI

Full `.gitlab-ci.yml` example covering all three methods:

```yaml
variables:
  CD_AGENT_URL: "http://<VM_IP>:8080/api/v1/deploy"
  CD_TOKEN: "your-super-secret-token"

# Method 1: registry pull with versioned tag
deploy-discovery:
  stage: deploy
  script:
    - IMAGE="sysadminaffiniti/affiniti_dms_ms_discovery_server:1.0.0-${CI_PIPELINE_IID}"
    - docker build -t "$IMAGE" . && docker push "$IMAGE"
    - |
      curl -sf -X POST "$CD_AGENT_URL" \
        -H "Authorization: Bearer $CD_TOKEN" \
        -H "Content-Type: application/json" \
        -d "{\"project\":\"pd-qa\",\"service\":\"pd-discovery-service\",\"image\":\"$IMAGE\"}"

# Method 2: gdown tar from Google Drive with versioned tag
deploy-auth:
  stage: deploy
  script:
    - IMAGE="affiniti-crm.duckdns.org:8080/sysadminaffiniti/affiniti_dms_ms_auth_service:1.0.0-${CI_PIPELINE_IID}"
    - docker build -t "$IMAGE" .
    - docker save "$IMAGE" -o auth.tar
    - GDRIVE_ID=$(gdrive files upload auth.tar --parent $GDRIVE_FOLDER_ID --json | jq -r '.id')
    - |
      curl -sf -X POST "$CD_AGENT_URL" \
        -H "Authorization: Bearer $CD_TOKEN" \
        -H "Content-Type: application/json" \
        -d "{\"project\":\"pd-qa\",\"service\":\"pd-ms-auth-service\",\"image\":\"$IMAGE\",\"gdrive_file_id\":\"$GDRIVE_ID\",\"tar_path\":\"/tmp/pd-ms-auth-service.tar\"}"

# Deploy all services at once (project-level, no service key)
deploy-all:
  stage: deploy
  script:
    - |
      curl -sf -X POST "$CD_AGENT_URL" \
        -H "Authorization: Bearer $CD_TOKEN" \
        -H "Content-Type: application/json" \
        -d '{"project":"pd-qa"}'
```

---

## Calling from Jenkins

```groovy
pipeline {
    agent any
    environment {
        CD_TOKEN = credentials('cd-agent-token')
    }
    stages {
        stage('Deploy') {
            steps {
                sh '''
                curl -sf -X POST http://<VM_IP>:8080/api/v1/deploy \
                  -H "Authorization: Bearer $CD_TOKEN" \
                  -H "Content-Type: application/json" \
                  -d '{"project":"pd-qa","service":"pd-ms-auth-service","gdrive_file_id":"<FILE_ID>","tar_path":"/tmp/pd-ms-auth-service.tar"}'
                '''
            }
        }
    }
}
```

---

## Payload Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `project` | string | Yes | Must match a key under `projects:` in config.yaml |
| `service` | string | No | Must match a key under `services:`. Omit to run project-level `deploy_command` |
| `image` | string | No | Docker image to pull before deploying |
| `gdrive_file_id` | string | No | Google Drive file ID; agent downloads via `gdown` to `tar_path` |
| `tar_path` | string | No | Absolute path on server to load with `docker load`. If used with `gdrive_file_id`, this is the download destination |

---

## Server Prerequisites

| Tool | Required for | Install |
|------|-------------|---------|
| `docker` | All methods | — |
| `docker compose` | All methods | — |
| `gdown` | Method 2 (Google Drive) | `pip install gdown` |
