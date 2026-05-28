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

The agent supports three image delivery methods. They are **not mutually exclusive** — all three are checked in order per request:

```
1. image         →  docker pull <image>
2. gdrive_file_id + tar_path  →  gdown <file_id> -O <tar_path>  then  docker load -i <tar_path>
3. tar_path only →  docker load -i <tar_path>   (tar already on server, e.g. via rclone)
4. deploy_command (always runs last)
```

### Method 1 — Registry Pull

CI pushes the image to a registry; the agent pulls it on the server.

**Webhook payload:**
```json
{
  "project": "pd-qa",
  "service": "pd-discovery-service",
  "image": "sysadminaffiniti/affiniti_dms_ms_discovery_server:pd-revamp-latest"
}
```

**What the agent does:**
1. `docker pull sysadminaffiniti/affiniti_dms_ms_discovery_server:pd-revamp-latest`
2. Runs `deploy_command` from config

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
  "gdrive_file_id": "1A2b3C4d5E6f7G8h9I0jKLmnoPqRst",
  "tar_path": "/tmp/pd-ms-auth-service.tar"
}
```

**What the agent does:**
1. `gdown 1A2b3C4d5E6f7G8h9I0jKLmnoPqRst -O /tmp/pd-ms-auth-service.tar`
2. `docker load -i /tmp/pd-ms-auth-service.tar`
3. Runs `deploy_command` from config

> `tar_path` is where the file is saved on the server. If omitted, it defaults to `/tmp/gdrive-download.tar`.

**GitLab CI — save and upload the tar:**
```yaml
deploy:
  stage: deploy
  script:
    # Build and save image as tar
    - docker build -t pd-ms-auth-service:latest .
    - docker save pd-ms-auth-service:latest -o auth-service.tar
    # Upload to Google Drive using gdrive or rclone, capture the file ID
    - GDRIVE_FILE_ID=$(gdrive files upload auth-service.tar --parent <FOLDER_ID> --json | jq -r '.id')
    # Trigger the CD agent
    - |
      curl -s -X POST http://<VM_IP>:8080/api/v1/deploy \
        -H "Authorization: Bearer your-super-secret-token" \
        -H "Content-Type: application/json" \
        -d "{
              \"project\": \"pd-qa\",
              \"service\": \"pd-ms-auth-service\",
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

## Calling from GitLab CI

Full `.gitlab-ci.yml` example covering all three methods:

```yaml
variables:
  CD_AGENT_URL: "http://<VM_IP>:8080/api/v1/deploy"
  CD_TOKEN: "your-super-secret-token"

# Method 1: registry pull
deploy-discovery:
  stage: deploy
  script:
    - |
      curl -sf -X POST "$CD_AGENT_URL" \
        -H "Authorization: Bearer $CD_TOKEN" \
        -H "Content-Type: application/json" \
        -d '{"project":"pd-qa","service":"pd-discovery-service","image":"sysadminaffiniti/affiniti_dms_ms_discovery_server:pd-revamp-latest"}'

# Method 2: gdown tar from Google Drive
deploy-auth:
  stage: deploy
  script:
    - docker build -t pd-ms-auth-service:latest .
    - docker save pd-ms-auth-service:latest -o auth.tar
    - GDRIVE_ID=$(gdrive files upload auth.tar --parent $GDRIVE_FOLDER_ID --json | jq -r '.id')
    - |
      curl -sf -X POST "$CD_AGENT_URL" \
        -H "Authorization: Bearer $CD_TOKEN" \
        -H "Content-Type: application/json" \
        -d "{\"project\":\"pd-qa\",\"service\":\"pd-ms-auth-service\",\"gdrive_file_id\":\"$GDRIVE_ID\",\"tar_path\":\"/tmp/pd-ms-auth-service.tar\"}"

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
