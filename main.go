package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"

	"cd-agent/api"
	"cd-agent/config"
	"cd-agent/deployer"
)

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

	d := deployer.New()

	mux := http.NewServeMux()
	mux.Handle("/api/v1/deploy", api.AuthMiddleware(cfg, api.DeployHandler(cfg, d)))

	addr := ":" + *port
	slog.Info("listening for webhooks", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
