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

// RegisterHealthEndpoints wires /health, /ready and /live, checking the same
// dependencies for health and readiness. Prefer RegisterHealthEndpointsWithReadiness
// when a dependency belongs to only one of the two.
func RegisterHealthEndpoints(mux *http.ServeMux, serviceName string, startTime time.Time, deps func() []DependencyCheck) {
	RegisterHealthEndpointsWithReadiness(mux, serviceName, startTime, deps, deps)
}

// RegisterHealthEndpointsWithReadiness wires /health, /ready and /live with separate
// dependency sets.
//
// health reports whether the process itself is in working order — the infrastructure
// it owns, such as its broker connection. It drives container health checks, so a
// third-party API belongs in readiness instead: an outage there would otherwise mark
// a perfectly functional service as broken.
//
// readiness reports whether the service can serve traffic right now, third-party
// dependencies included. Callers that route work should read this one.
//
// /live answers 200 for as long as the process is running and takes no dependencies.
func RegisterHealthEndpointsWithReadiness(mux *http.ServeMux, serviceName string, startTime time.Time, health, readiness func() []DependencyCheck) {
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		checks, ok := evaluate(health)

		status := "healthy"
		if !ok {
			status = "unhealthy"
		}

		response := HealthResponse{
			Status:        status,
			ServiceName:   serviceName,
			Timestamp:     time.Now(),
			UptimeSeconds: int64(time.Since(startTime).Seconds()),
			Checks:        checks,
		}

		w.Header().Set("Content-Type", "application/json")
		if !ok {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(response)
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		checks, ok := evaluate(readiness)

		status := "ready"
		if !ok {
			status = "not ready"
		}

		w.Header().Set("Content-Type", "application/json")
		if !ok {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		// Checks travel with the answer: "not ready" on its own says nothing about which
		// dependency is at fault.
		json.NewEncoder(w).Encode(map[string]interface{}{"status": status, "checks": checks})
	})

	mux.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
	})
}

// evaluate runs a dependency set and reports the results alongside whether all passed.
// A nil set is vacuously satisfied.
func evaluate(deps func() []DependencyCheck) (map[string]interface{}, bool) {
	if deps == nil {
		return map[string]interface{}{}, true
	}

	results := deps()
	checks := make(map[string]interface{}, len(results))
	ok := true
	for _, d := range results {
		checks[d.Name] = d.OK
		if !d.OK {
			ok = false
		}
	}
	return checks, ok
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
