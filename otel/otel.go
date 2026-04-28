// Package otel exports workflow-sdk telemetry and NATS context propagation.
package otel

import (
	"context"
	"github.com/nats-io/nats.go"
	"github.com/lordtor/workflow-sdk/telemetry"
	"github.com/lordtor/workflow-sdk/nats"
)

func InitTracing(ctx context.Context, serviceName string) (func(), error) {
	return telemetry.InitTracing(ctx, serviceName)
}

func InjectNatsTraceContext(ctx context.Context, msg *nats.Msg) {
	nats.InjectNatsTraceContext(ctx, msg)
}

func ExtractNatsTraceContext(ctx context.Context, msg *nats.Msg) context.Context {
	return nats.ExtractNatsTraceContext(ctx, msg)
}
