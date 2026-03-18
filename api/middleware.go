package api

import (
	"log"
	"net/http"
	"strings"

	"cd-agent/config"
)

// AuthMiddleware wraps an http.Handler to ensure valid Bearer token authentication.
func AuthMiddleware(cfg *config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Unauthorized: missing Authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, "Unauthorized: invalid Authorization format", http.StatusUnauthorized)
			return
		}

		token := parts[1]
		if token != cfg.APIToken {
			log.Printf("Failed authentication attempt from %s", r.RemoteAddr)
			http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
