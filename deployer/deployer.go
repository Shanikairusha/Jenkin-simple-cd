package deployer

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cd-agent/executor"
	"cd-agent/store"
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
type DefaultDeployer struct {
	store *store.Store // nil-safe: if nil, no records are written
}

func New(s *store.Store) *DefaultDeployer {
	return &DefaultDeployer{store: s}
}

func (d *DefaultDeployer) Deploy(req Request) {
	log := slog.With("project", req.Project, "service", req.Service)

	var rec *store.DeployRecord
	if d.store != nil {
		rec = &store.DeployRecord{
			ID:        store.GenerateID(),
			Project:   req.Project,
			Service:   req.Service,
			Image:     req.Image,
			Status:    store.StatusRunning,
			StartTime: time.Now(),
		}
		d.store.Add(rec)
	}

	appendLog := func(line string) {
		if rec != nil {
			rec.AppendLog(line)
		}
	}

	success := true

	// If the deployment is delivered via tar/gdrive, skip registry pull
	// because the image may not exist in the registry yet.
	if req.Image != "" && req.GdriveFileID == "" && req.TarPath == "" {
		out, err := executor.PullImage(req.WorkDir, req.Image)
		if err != nil {
			log.Error("failed to pull image", "error", err)
			appendLog(fmt.Sprintf("PULL ERROR: %v", err))
			if out != "" {
				appendLog(out)
			}
			success = false
		} else {
			appendLog("PULL OK: " + req.Image)
			if out != "" {
				appendLog(out)
			}
		}
	}

	if req.GdriveFileID != "" {
		tarPath := req.TarPath
		if tarPath == "" {
			tarPath = "/tmp/gdrive-download.tar"
		}
		out, err := executor.DownloadGdown(req.WorkDir, req.GdriveFileID, tarPath)
		if err != nil {
			log.Error("failed to download from Google Drive", "error", err)
			appendLog(fmt.Sprintf("GDOWN ERROR: %v", err))
			if out != "" {
				appendLog(out)
			}
			success = false
		} else {
			appendLog("GDOWN OK: " + tarPath)
			if out != "" {
				appendLog(out)
			}
		}
		req.TarPath = tarPath
	}

	if req.TarPath != "" {
		out, err := executor.LoadTarImage(req.WorkDir, req.TarPath)
		if err != nil {
			log.Error("failed to load tar image", "error", err)
			appendLog(fmt.Sprintf("LOAD ERROR: %v", err))
			if out != "" {
				appendLog(out)
			}
			success = false
		} else {
			appendLog("LOAD OK: " + req.TarPath)
			if out != "" {
				appendLog(out)
			}
		}
	}

	// Record the deployed image in .env so Docker Compose uses the versioned tag
	// and rollback is possible by editing .env and re-running docker compose up.
	if req.Image != "" && req.Service != "" {
		if err := updateEnvFile(req.WorkDir, req.Service, req.Image); err != nil {
			log.Error("failed to update .env file", "error", err)
			appendLog(fmt.Sprintf("ENV UPDATE ERROR: %v", err))
			success = false
		} else {
			log.Info("updated .env with image", "image", req.Image)
			appendLog("ENV UPDATE OK: " + req.Image)
		}
	}

	log.Info("starting deploy command")
	out, err := executor.RunShellCommand(req.WorkDir, req.DeployCommand)
	if err != nil {
		log.Error("deployment failed", "error", err)
		appendLog(fmt.Sprintf("DEPLOY ERROR: %v", err))
		if out != "" {
			appendLog(out)
		}
		success = false
	} else {
		log.Info("deployment succeeded")
		appendLog("DEPLOY OK")
		if out != "" {
			appendLog(out)
		}
	}

	if rec != nil {
		rec.Complete(success, time.Now())
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
