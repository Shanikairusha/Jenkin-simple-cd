package main

import (
	"log"
	"net/http"

	"cd-agent/api"
	"cd-agent/config"
)

func main() {
	log.Println("Starting CD Agent...")

	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("Failed to start: %v", err)
	}

	log.Println("Config loaded successfully. Found", len(cfg.Projects), "projects.")

	mux := http.NewServeMux()
	mux.Handle("/api/v1/deploy", api.AuthMiddleware(cfg, api.DeployHandler(cfg)))

	serverAddr := ":8080"
	log.Printf("Listening for webhooks on %s...", serverAddr)
	if err := http.ListenAndServe(serverAddr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
