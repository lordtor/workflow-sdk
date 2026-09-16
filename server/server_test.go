package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func deps(checks ...DependencyCheck) func() []DependencyCheck {
	return func() []DependencyCheck { return checks }
}

func get(t *testing.T, mux *http.ServeMux, path string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("%s: decode body %q: %v", path, rec.Body.String(), err)
	}
	return rec.Code, body
}

// A dependency listed only under readiness must not drag /health down: container health
// checks read /health, and a third-party outage should not look like a broken process.
func TestReadinessDependencyDoesNotAffectHealth(t *testing.T) {
	mux := http.NewServeMux()
	RegisterHealthEndpointsWithReadiness(mux, "svc", time.Now(),
		deps(DependencyCheck{Name: "nats", OK: true}),
		deps(DependencyCheck{Name: "nats", OK: true}, DependencyCheck{Name: "gitlab", OK: false}),
	)

	if code, body := get(t, mux, "/health"); code != http.StatusOK || body["status"] != "healthy" {
		t.Fatalf("/health = %d %v, want 200 healthy", code, body)
	}

	code, body := get(t, mux, "/ready")
	if code != http.StatusServiceUnavailable || body["status"] != "not ready" {
		t.Fatalf("/ready = %d %v, want 503 not ready", code, body)
	}

	checks, ok := body["checks"].(map[string]any)
	if !ok {
		t.Fatalf("/ready body carries no checks: %v", body)
	}
	if checks["gitlab"] != false {
		t.Fatalf("/ready checks = %v, want gitlab false", checks)
	}

	if code, body := get(t, mux, "/live"); code != http.StatusOK || body["status"] != "alive" {
		t.Fatalf("/live = %d %v, want 200 alive", code, body)
	}
}

// A failing health dependency fails both endpoints: the process cannot work without it.
func TestHealthDependencyFailsBoth(t *testing.T) {
	mux := http.NewServeMux()
	RegisterHealthEndpointsWithReadiness(mux, "svc", time.Now(),
		deps(DependencyCheck{Name: "nats", OK: false}),
		deps(DependencyCheck{Name: "nats", OK: false}),
	)

	if code, _ := get(t, mux, "/health"); code != http.StatusServiceUnavailable {
		t.Fatalf("/health = %d, want 503", code)
	}
	if code, _ := get(t, mux, "/ready"); code != http.StatusServiceUnavailable {
		t.Fatalf("/ready = %d, want 503", code)
	}
}

// The single-set form keeps its old behaviour: one dependency list drives both endpoints.
func TestRegisterHealthEndpointsUsesOneSetForBoth(t *testing.T) {
	mux := http.NewServeMux()
	RegisterHealthEndpoints(mux, "svc", time.Now(),
		deps(DependencyCheck{Name: "gitlab", OK: false}))

	if code, _ := get(t, mux, "/health"); code != http.StatusServiceUnavailable {
		t.Fatalf("/health = %d, want 503", code)
	}
	if code, _ := get(t, mux, "/ready"); code != http.StatusServiceUnavailable {
		t.Fatalf("/ready = %d, want 503", code)
	}
}
