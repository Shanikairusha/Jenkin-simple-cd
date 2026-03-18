# Simple Continuous Deployment (CD) Agent

A lightweight webhook agent written in Go that listens for deployment notifications (e.g., from Jenkins) and executes local Docker and shell commands to deploy microservices on your servers.

## Getting Started

1. **Build the binary**:
   ```bash
   go build -o cd-agent main.go
   ```

2. **Configure the agent**:
   Create a `config.yaml` next to your binary. See [`config.yaml.example`] for a starting point.

3. **Run the agent**:
   ```bash
   ./cd-agent
   ```
   *Optionally, set this up as a `systemd` service or run it in `tmux`/background.*

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
                        "image": "myregistry.com/auth:latest"
                      }'
                '''
            }
        }
    }
}
```

*Note: If `"image"` is provided, the agent will aggressively run `docker pull <image>` before executing the `deploy_command`.*
