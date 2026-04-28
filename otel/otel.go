// Package otel exports workflow-sdk telemetry and NATS context propagation.
package otel

import (
	"context"

	natslib "github.com/nats-io/nats.go"
	"github.com/lordtor/workflow-sdk/nats"
	"github.com/lordtor/workflow-sdk/telemetry"
)

func InitTracing(ctx context.Context, serviceName string) (func(), error) {
	return telemetry.InitTracing(ctx, serviceName)
}

func InjectNatsTraceContext(ctx context.Context, msg *natslib.Msg) {
	nats.InjectNatsTraceContext(ctx, msg)
}

func ExtractNatsTraceContext(ctx context.Context, msg *natslib.Msg) context.Context {
	return nats.ExtractNatsTraceContext(ctx, msg)
}
