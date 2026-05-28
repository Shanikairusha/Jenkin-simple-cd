package main

import (
	"embed"
	"flag"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"time"

	"cd-agent/api"
	"cd-agent/config"
	"cd-agent/deployer"
	"cd-agent/store"
)

//go:embed web
var webFS embed.FS

func getenvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	port := flag.String("port", "8080", "port to listen on")
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	*port = getenvOrDefault("PORT", *port)
	*configPath = getenvOrDefault("CONFIG_PATH", *configPath)

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	slog.Info("starting CD Agent")

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("config loaded", "projects", len(cfg.Projects))

	s := store.New()
	startTime := time.Now()
	d := deployer.New(s)

	mux := http.NewServeMux()

	// Existing webhook endpoint
	mux.Handle("POST /api/v1/deploy", api.AuthMiddleware(cfg, api.DeployHandler(cfg, d)))

	// UI API endpoints (all behind auth)
	mux.Handle("GET /api/v1/ui/health", api.AuthMiddleware(cfg, api.UIHealthHandler(startTime)))
	mux.Handle("GET /api/v1/ui/deployments", api.AuthMiddleware(cfg, api.UIDeploymentsHandler(s)))
	mux.Handle("GET /api/v1/ui/deployments/{id}", api.AuthMiddleware(cfg, api.UIDeploymentDetailHandler(s)))
	mux.Handle("GET /api/v1/ui/projects", api.AuthMiddleware(cfg, api.UIProjectsHandler(cfg)))

	// Static SPA — unauthenticated so login page loads without a token
	webContent, _ := fs.Sub(webFS, "web")
	mux.Handle("GET /", http.FileServer(http.FS(webContent)))

	addr := ":" + *port
	slog.Info("listening for webhooks", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
