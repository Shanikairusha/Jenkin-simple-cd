package api

import (
	"encoding/json"
	"log"
	"net/http"

	"cd-agent/config"
	"cd-agent/executor"
)

// DeployPayload represents the expected JSON body from Jenkins.
type DeployPayload struct {
	Project string `json:"project"`
	Service string `json:"service"`
	Image   string `json:"image,omitempty"`
}

// DeployHandler returns an http.Handler that processes the deployment webhooks.
func DeployHandler(cfg *config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload DeployPayload
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&payload); err != nil {
			http.Error(w, "Bad Request: invalid JSON payload", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		if payload.Project == "" || payload.Service == "" {
			http.Error(w, "Bad Request: project and service are required", http.StatusBadRequest)
			return
		}

		log.Printf("Received deployment request: Project=%s, Service=%s, Image=%s", payload.Project, payload.Service, payload.Image)

		// Find the project configuration
		projectConfig, ok := cfg.Projects[payload.Project]
		if !ok {
			log.Printf("Project not found in config: %s", payload.Project)
			http.Error(w, "Not Found: project not configured", http.StatusNotFound)
			return
		}

		// If service is omitted, we assume project-level execution
		var workDir, deployCommand string

		if payload.Service == "" {
			// Project-level execution
			workDir = projectConfig.WorkingDirectory
			deployCommand = projectConfig.DeployCommand
			if deployCommand == "" {
				log.Printf("No deploy_command configured for project %s", payload.Project)
				http.Error(w, "Failed: no deployment command for project", http.StatusBadRequest)
				return
			}
		} else {
			// Service-level execution
			serviceConfig, ok := projectConfig.Services[payload.Service]
			if !ok {
				log.Printf("Service not found in project %s: %s", payload.Project, payload.Service)
				http.Error(w, "Not Found: service not configured for this project", http.StatusNotFound)
				return
			}

			workDir = serviceConfig.WorkingDirectory
			if workDir == "" {
				workDir = projectConfig.WorkingDirectory
			}
			deployCommand = serviceConfig.DeployCommand

			if deployCommand == "" {
				log.Printf("No deploy_command configured for service %s in project %s", payload.Service, payload.Project)
				http.Error(w, "Failed: no deployment command for service", http.StatusBadRequest)
				return
			}
		}

		if workDir == "" {
			log.Printf("No working directory configured for deployment of %s", payload.Project)
			http.Error(w, "Internal Server Error: no working directory configured", http.StatusInternalServerError)
			return
		}

		// Execute deployment asynchronously so we don't block the webhook response
		go func() {
			if payload.Image != "" {
				if err := executor.PullImage(workDir, payload.Image); err != nil {
					log.Printf("Failed to pull image for %s/%s: %v", payload.Project, payload.Service, err)
					// Optionally, we could choose to abort deployment, but let's try to proceed
				}
			}

			log.Printf("Starting deployment for %s/%s", payload.Project, payload.Service)
			// Pass the string securely to the executor, which will split it or pass it to bash
			if err := executor.RunShellCommand(workDir, deployCommand); err != nil {
				log.Printf("Deployment failed for %s/%s: %v", payload.Project, payload.Service, err)
			} else {
				log.Printf("Deployment successful for %s/%s", payload.Project, payload.Service)
			}
		}()

		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("Deployment initiated\n"))
	})
}
