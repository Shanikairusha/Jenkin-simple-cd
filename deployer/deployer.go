package deployer

import (
	"log/slog"

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

	log.Info("starting deploy command")
	if err := executor.RunShellCommand(req.WorkDir, req.DeployCommand); err != nil {
		log.Error("deployment failed", "error", err)
	} else {
		log.Info("deployment succeeded")
	}
}
