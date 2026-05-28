package api

import (
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"time"

	"cd-agent/config"
	"cd-agent/store"
)

// UIHealthHandler returns agent health and uptime.
func UIHealthHandler(startTime time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":          "healthy",
			"uptime_seconds":  int64(time.Since(startTime).Seconds()),
			"started_at":      startTime.UTC(),
		})
	})
}

// UIDeploymentsHandler returns all deployment records with aggregate stats.
func UIDeploymentsHandler(s *store.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		records := s.List()

		type summary struct {
			ID              string     `json:"id"`
			Project         string     `json:"project"`
			Service         string     `json:"service"`
			Image           string     `json:"image"`
			Status          string     `json:"status"`
			StartTime       time.Time  `json:"start_time"`
			EndTime         *time.Time `json:"end_time"`
			DurationSeconds int64      `json:"duration_seconds"`
		}

		type stats struct {
			Total              int     `json:"total"`
			Success            int     `json:"success"`
			Failed             int     `json:"failed"`
			Running            int     `json:"running"`
			SuccessRate        float64 `json:"success_rate"`
			AvgDurationSeconds int64   `json:"avg_duration_seconds"`
		}

		deploys := make([]summary, 0, len(records))
		var totalSuccess, totalFailed, totalRunning int
		var totalDuration int64
		var completedCount int

		for _, rec := range records {
			s := summary{
				ID:        rec.ID,
				Project:   rec.Project,
				Service:   rec.Service,
				Image:     rec.Image,
				Status:    rec.Status,
				StartTime: rec.StartTime,
			}
			if !rec.EndTime.IsZero() {
				t := rec.EndTime
				s.EndTime = &t
				s.DurationSeconds = int64(rec.EndTime.Sub(rec.StartTime).Seconds())
				totalDuration += s.DurationSeconds
				completedCount++
			}
			switch rec.Status {
			case store.StatusSuccess:
				totalSuccess++
			case store.StatusFailed:
				totalFailed++
			case store.StatusRunning:
				totalRunning++
			}
			deploys = append(deploys, s)
		}

		finished := totalSuccess + totalFailed
		var successRate float64
		if finished > 0 {
			successRate = math.Round(float64(totalSuccess)/float64(finished)*1000) / 10
		}
		var avgDuration int64
		if completedCount > 0 {
			avgDuration = totalDuration / int64(completedCount)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"deployments": deploys,
			"total":       len(deploys),
			"stats": stats{
				Total:              len(deploys),
				Success:            totalSuccess,
				Failed:             totalFailed,
				Running:            totalRunning,
				SuccessRate:        successRate,
				AvgDurationSeconds: avgDuration,
			},
		})
	})
}

// UIDeploymentDetailHandler returns a single deployment record including logs.
func UIDeploymentDetailHandler(s *store.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		rec := s.Get(id)
		if rec == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
			return
		}

		snap := rec.Snapshot()

		type detail struct {
			ID              string     `json:"id"`
			Project         string     `json:"project"`
			Service         string     `json:"service"`
			Image           string     `json:"image"`
			Status          string     `json:"status"`
			StartTime       time.Time  `json:"start_time"`
			EndTime         *time.Time `json:"end_time"`
			DurationSeconds int64      `json:"duration_seconds"`
			Logs            []string   `json:"logs"`
		}

		d := detail{
			ID:        snap.ID,
			Project:   snap.Project,
			Service:   snap.Service,
			Image:     snap.Image,
			Status:    snap.Status,
			StartTime: snap.StartTime,
			Logs:      snap.Logs,
		}
		if !snap.EndTime.IsZero() {
			t := snap.EndTime
			d.EndTime = &t
			d.DurationSeconds = int64(snap.EndTime.Sub(snap.StartTime).Seconds())
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(d)
	})
}

// UIProjectsHandler returns the list of configured projects and their services.
func UIProjectsHandler(cfg *config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type projectInfo struct {
			Name     string   `json:"name"`
			Services []string `json:"services"`
		}

		projects := make([]projectInfo, 0, len(cfg.Projects))
		for name, proj := range cfg.Projects {
			services := make([]string, 0, len(proj.Services))
			for svc := range proj.Services {
				services = append(services, svc)
			}
			sort.Strings(services)
			projects = append(projects, projectInfo{Name: name, Services: services})
		}
		sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"projects": projects})
	})
}
