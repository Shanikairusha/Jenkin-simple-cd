package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cd-agent/config"
	"cd-agent/deployer"
)

type mockDeployer struct {
	done    chan struct{}
	lastReq deployer.Request
}

func newMockDeployer() *mockDeployer {
	return &mockDeployer{done: make(chan struct{}, 1)}
}

func (m *mockDeployer) Deploy(req deployer.Request) {
	m.lastReq = req
	m.done <- struct{}{}
}

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

	tests := []struct {
		name           string
		payload        string
		expectedStatus int
		expectDeploy   bool
	}{
		{"Valid Payload", `{"project":"test-proj", "service":"test-svc"}`, http.StatusAccepted, true},
		{"Missing Project", `{"service":"test-svc"}`, http.StatusBadRequest, false},
		{"Project-level with no deploy_command", `{"project":"test-proj"}`, http.StatusBadRequest, false},
		{"Invalid JSON", `{"project":"test-=}!!`, http.StatusBadRequest, false},
		{"Project Not Configured", `{"project":"missing-proj", "service":"test-svc"}`, http.StatusNotFound, false},
		{"Service Not Configured", `{"project":"test-proj", "service":"missing-svc"}`, http.StatusNotFound, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := newMockDeployer()
			handlerToTest := DeployHandler(cfg, mock)

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

			if tc.expectDeploy {
				select {
				case <-mock.done:
				case <-time.After(time.Second):
					t.Errorf("Deploy was not called within timeout for %s", tc.name)
				}
			} else {
				select {
				case <-mock.done:
					t.Errorf("expected Deploy NOT to be called for %s, but it was", tc.name)
				default:
				}
			}
		})
	}
}
