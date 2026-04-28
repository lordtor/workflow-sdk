//go:build otel_nats

// Package workflowotel provides OpenTelemetry integration for workflow services.
// This package is meant to be imported by microservice implementations
// that need tracing and NATS context propagation.
package workflowotel

import _ "github.com/lordtor/workflow-sdk/telemetry"
import _ "github.com/lordtor/workflow-sdk/nats"
