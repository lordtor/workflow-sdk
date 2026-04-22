package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type HealthResponse struct {
	Status        string                 `json:"status"`
	ServiceName   string                 `json:"service_name"`
	Timestamp     time.Time              `json:"timestamp"`
	UptimeSeconds int64                  `json:"uptime_seconds"`
	Checks        map[string]interface{} `json:"checks"`
}

type DependencyCheck struct {
	Name string
	OK   bool
}

func RegisterHealthEndpoints(mux *http.ServeMux, serviceName string, startTime time.Time, deps func() []DependencyCheck) {
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		depResults := deps()
		checks := make(map[string]interface{}, len(depResults))
		status := "healthy"
		for _, d := range depResults {
			checks[d.Name] = d.OK
			if !d.OK {
				status = "unhealthy"
			}
		}

		response := HealthResponse{
			Status:        status,
			ServiceName:   serviceName,
			Timestamp:     time.Now(),
			UptimeSeconds: int64(time.Since(startTime).Seconds()),
			Checks:        checks,
		}

		w.Header().Set("Content-Type", "application/json")
		if status == "unhealthy" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(response)
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		depResults := deps()
		allReady := true
		for _, d := range depResults {
			if !d.OK {
				allReady = false
				break
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if allReady {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "not ready"})
		}
	})

	mux.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
	})
}

func StartHTTPServer(addr string, handler http.Handler) error {
	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	log.Printf("Starting HTTP server on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
	return nil
}
