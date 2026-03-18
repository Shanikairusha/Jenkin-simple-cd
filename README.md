# Simple Continuous Deployment (CD) Agent

A lightweight webhook agent written in Go that listens for deployment notifications (e.g., from Jenkins) and executes local Docker and shell commands to deploy microservices on your servers.

## Getting Started

1. **Build the binary**:
   - For local testing: `go build -o cd-agent main.go`
   - **For Linux Servers (from Windows)**: Run the included script:
     ```powershell
     .\build-linux.ps1
     ```
     This will generate a `cd-agent-linux` executable.

2. **Configure the agent**:
   Create a `config.yaml` next to your binary. See [`config.yaml.example`] for a starting point.

3. **Deploy to Server**:
   Copy `cd-agent-linux`, `config.yaml`, `start.sh`, and `stop.sh` to your Linux server.

4. **Run the agent**:
   Make the scripts executable on the server if they aren't already:
   ```bash
   chmod +x cd-agent-linux start.sh stop.sh
   ```
   
   To run it in the background:
   ```bash
   ./start.sh
   ```
   *(This uses `nohup` to run the agent in the background, writes logs to `cd-agent.log`, and saves the process ID in `cd-agent.pid`)*

   To stop the agent:
   ```bash
   ./stop.sh
   ```

## Configuration Structure

The agent expects a `config.yaml` file in the same directory:

```yaml
api_token: "your-super-secret-token"
projects:
  my-project:
    working_directory: "/opt/deployments/my-project"
    services:
      auth-service:
        deploy_command:
          - docker-compose
          - up
          - "-d"
          - auth-service
      standalone-service:
        working_directory: "/opt/deployments/my-project/standalone-service"
        deploy_command:
          - ./deploy.sh
```

- **api_token**: Required to authorize Jenkins webhooks.
- **projects.working_directory**: Base path for `docker-compose.yaml` files.
- **services.deploy_command**: The array of command arguments to restart the service (e.g., `docker-compose up -d auth-service`).

## Calling from Jenkins

A Jenkins pipeline can call this webhook using `curl` as the final step of a deployment:

```groovy
pipeline {
    agent any
    stages {
        stage('Deploy') {
            steps {
                sh '''
                curl -X POST http://<SERVER_IP>:8080/api/v1/deploy \
                  -H "Authorization: Bearer your-super-secret-token" \
                  -H "Content-Type: application/json" \
                  -d '{
                        "project": "my-project",
                        "service": "auth-service",
                        "image": "myregistry.com/auth:latest",
                        "tar_path": "/opt/docker/images/auth.tar",
                        "gdrive_file_id": "1A2b3C4d5E6f7G8h9I0jKLmnoPqRst"
                      }'
                '''
            }
        }
    }
}
```

### Payload Options

- `"image"`: If provided, the agent explicitly runs `docker pull <image>` before executing your deployment.
- `"tar_path"`: If you are using `rclone` to mount direct `.tar` files to the server, provide the absolute path here. The agent will execute `docker load -i <tar_path>`.
- `"gdrive_file_id"`: If you want the server to directly download a `.tar` from Google Drive before loading, provide the Google Drive File ID here AND provide a local `"tar_path"` indicating where it should be saved. *Requires the `gdown` Python package to be installed on your server (`pip install gdown`).*
