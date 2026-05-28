package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"cd-agent/config"
	"cd-agent/deployer"
)

// DeployPayload represents the expected JSON body from Jenkins.
type DeployPayload struct {
	Project      string `json:"project"`
	Service      string `json:"service"`
	Image        string `json:"image,omitempty"`
	TarPath      string `json:"tar_path,omitempty"`       // Local path to a .tar image (e.g. from rclone)
	GdriveFileID string `json:"gdrive_file_id,omitempty"` // Google Drive File ID to download
}

// DeployHandler returns an http.Handler that processes deployment webhooks.
func DeployHandler(cfg *config.Config, d deployer.Deployer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload DeployPayload
		decoder := json.NewDecoder(r.Body)
		defer r.Body.Close()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(w, "Bad Request: invalid JSON payload", http.StatusBadRequest)
			return
		}

		if payload.Project == "" {
			http.Error(w, "Bad Request: 'project' is required", http.StatusBadRequest)
			return
		}

		slog.Info("received deployment request",
			"project", payload.Project,
			"service", payload.Service,
			"image", payload.Image,
		)

		projectConfig, ok := cfg.Projects[payload.Project]
		if !ok {
			slog.Warn("project not found in config", "project", payload.Project)
			http.Error(w, "Not Found: project not configured", http.StatusNotFound)
			return
		}

		var workDir, deployCommand string

		if payload.Service == "" {
			// Project-level execution
			workDir = projectConfig.WorkingDirectory
			deployCommand = projectConfig.DeployCommand
			if deployCommand == "" {
				slog.Warn("no deploy_command for project", "project", payload.Project)
				http.Error(w, "Failed: no deployment command for project", http.StatusBadRequest)
				return
			}
		} else {
			// Service-level execution
			serviceConfig, ok := projectConfig.Services[payload.Service]
			if !ok {
				slog.Warn("service not found in project", "project", payload.Project, "service", payload.Service)
				http.Error(w, "Not Found: service not configured for this project", http.StatusNotFound)
				return
			}

			workDir = serviceConfig.WorkingDirectory
			if workDir == "" {
				workDir = projectConfig.WorkingDirectory
			}
			deployCommand = serviceConfig.DeployCommand

			if deployCommand == "" {
				slog.Warn("no deploy_command for service", "project", payload.Project, "service", payload.Service)
				http.Error(w, "Failed: no deployment command for service", http.StatusBadRequest)
				return
			}
		}

		if workDir == "" {
			slog.Error("no working directory configured", "project", payload.Project)
			http.Error(w, "Internal Server Error: no working directory configured", http.StatusInternalServerError)
			return
		}

		go d.Deploy(deployer.Request{
			Project:       payload.Project,
			Service:       payload.Service,
			WorkDir:       workDir,
			DeployCommand: deployCommand,
			Image:         payload.Image,
			TarPath:       payload.TarPath,
			GdriveFileID:  payload.GdriveFileID,
		})

		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("Deployment initiated\n"))
	})
}
