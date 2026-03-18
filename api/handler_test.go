package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"cd-agent/config"
)

func TestAuthMiddleware(t *testing.T) {
	cfg := &config.Config{
		APIToken: "secret123",
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	handlerToTest := AuthMiddleware(cfg, nextHandler)

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{"No Header", "", http.StatusUnauthorized},
		{"Invalid Format", "secret123", http.StatusUnauthorized},
		{"Invalid Token", "Bearer wrongsecret", http.StatusUnauthorized},
		{"Valid Token", "Bearer secret123", http.StatusOK},
		{"Case Insensitive Bearer", "bearer secret123", http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest("POST", "/deploy", nil)
			if err != nil {
				t.Fatal(err)
			}

			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			rr := httptest.NewRecorder()
			handlerToTest.ServeHTTP(rr, req)

			if status := rr.Code; status != tc.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tc.expectedStatus)
			}
		})
	}
}

func TestDeployHandler(t *testing.T) {
	cfg := &config.Config{
		Projects: map[string]config.ProjectConfig{
			"test-proj": {
				WorkingDirectory: "/tmp",
				Services: map[string]config.ServiceConfig{
					"test-svc": {
						DeployCommand: "echo test",
					},
				},
			},
		},
	}

	handlerToTest := DeployHandler(cfg)

	tests := []struct {
		name           string
		payload        string
		expectedStatus int
	}{
		{"Valid Payload", `{"project":"test-proj", "service":"test-svc"}`, http.StatusAccepted},
		{"Missing Project", `{"service":"test-svc"}`, http.StatusBadRequest},
		{"Missing Service", `{"project":"test-proj"}`, http.StatusBadRequest},
		{"Invalid JSON", `{"project":"test-=}!!`, http.StatusBadRequest},
		{"Project Not Configured", `{"project":"missing-proj", "service":"test-svc"}`, http.StatusNotFound},
		{"Service Not Configured", `{"project":"test-proj", "service":"missing-svc"}`, http.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest("POST", "/deploy", bytes.NewBuffer([]byte(tc.payload)))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handlerToTest.ServeHTTP(rr, req)

			if status := rr.Code; status != tc.expectedStatus {
				t.Errorf("handler returned wrong status code for %s: got %v want %v",
					tc.name, status, tc.expectedStatus)
			}
		})
	}
}
