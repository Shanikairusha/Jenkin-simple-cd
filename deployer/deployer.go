package deployer

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"cd-agent/executor"
)

// Request holds the resolved, validated parameters for a single deployment run.
type Request struct {
	Project       string
	Service       string
	WorkDir       string
	DeployCommand string
	Image         string
	TarPath       string
	GdriveFileID  string
}

// Deployer runs a full deployment sequence for a resolved request.
type Deployer interface {
	Deploy(req Request)
}

// DefaultDeployer is the production implementation backed by the executor package.
type DefaultDeployer struct{}

func New() *DefaultDeployer {
	return &DefaultDeployer{}
}

func (d *DefaultDeployer) Deploy(req Request) {
	log := slog.With("project", req.Project, "service", req.Service)

	if req.Image != "" {
		if err := executor.PullImage(req.WorkDir, req.Image); err != nil {
			log.Error("failed to pull image", "error", err)
		}
	}

	if req.GdriveFileID != "" {
		tarPath := req.TarPath
		if tarPath == "" {
			tarPath = "/tmp/gdrive-download.tar"
		}
		if err := executor.DownloadGdown(req.WorkDir, req.GdriveFileID, tarPath); err != nil {
			log.Error("failed to download from Google Drive", "error", err)
		}
		req.TarPath = tarPath
	}

	if req.TarPath != "" {
		if err := executor.LoadTarImage(req.WorkDir, req.TarPath); err != nil {
			log.Error("failed to load tar image", "error", err)
		}
	}

	// Record the deployed image in .env so Docker Compose uses the versioned tag
	// and rollback is possible by editing .env and re-running docker compose up.
	if req.Image != "" && req.Service != "" {
		if err := updateEnvFile(req.WorkDir, req.Service, req.Image); err != nil {
			log.Error("failed to update .env file", "error", err)
		} else {
			log.Info("updated .env with image", "image", req.Image)
		}
	}

	log.Info("starting deploy command")
	if err := executor.RunShellCommand(req.WorkDir, req.DeployCommand); err != nil {
		log.Error("deployment failed", "error", err)
	} else {
		log.Info("deployment succeeded")
	}
}

// updateEnvFile writes or updates a KEY=VALUE line in <workDir>/.env.
// The key is derived from the service name: "pd-ms-auth-service" → "PD_MS_AUTH_SERVICE_IMAGE".
// The file is written atomically via a temp file to avoid partial writes.
func updateEnvFile(workDir, serviceName, image string) error {
	key := strings.ToUpper(strings.ReplaceAll(serviceName, "-", "_")) + "_IMAGE"
	envPath := filepath.Join(workDir, ".env")
	newLine := fmt.Sprintf("%s=%s", key, image)

	// Read existing content, or start fresh if the file doesn't exist yet.
	raw, err := os.ReadFile(envPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read .env: %w", err)
	}

	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	// Filter out a single empty string that results from splitting an empty file.
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}

	updated := false
	prefix := key + "="
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			lines[i] = newLine
			updated = true
			break
		}
	}
	if !updated {
		lines = append(lines, newLine)
	}

	content := strings.Join(lines, "\n") + "\n"

	// Write atomically: temp file in the same directory, then rename.
	tmp, err := os.CreateTemp(workDir, ".env.tmp.*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, envPath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp to .env: %w", err)
	}

	return nil
}
