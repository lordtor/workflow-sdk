package telemetry

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.32.0"
)

const version = "1.0.0"

func InitTracing(ctx context.Context, serviceName string) (func(), error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://tmpo:4318"
	}

	exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpoint(endpoint))
	if err != nil {
		return nil, fmt.Errorf("creating OTLP trace exporter: %w", err)
	}

	ratio := 1.0
	if r := os.Getenv("OTEL_TRACES_SAMPLER_ARG"); r != "" {
		if _, err := fmt.Sscanf(r, "%f", &ratio); err != nil {
			return nil, fmt.Errorf("parsing OTEL_TRACES_SAMPLER_ARG: %w", err)
		}
	}

	deploymentEnv := os.Getenv("ENVIRONMENT")
	if deploymentEnv == "" {
		deploymentEnv = "development"
	}

	clusterName := os.Getenv("CLUSTER_NAME")
	if clusterName == "" {
		clusterName = "docker-desktop"
	}

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(version),
		semconv.DeploymentEnvironmentName(deploymentEnv),
		semconv.K8SClusterName(clusterName),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
		sdktrace.WithBatcher(exporter),
	)

	otel.SetTracerProvider(tp)

	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(propagator)

	cleanup := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(shutdownCtx); err != nil {
			fmt.Printf("TracerProvider shutdown: %v\n", err)
		}
	}

	return cleanup, nil
}
