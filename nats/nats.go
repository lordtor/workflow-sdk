package nats

import (
	"context"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
)

// InjectNatsTraceContext injects OpenTelemetry context into NATS message headers.
func InjectNatsTraceContext(ctx context.Context, msg *nats.Msg) {
	prop := otel.GetTextMapPropagator()
	prop.Inject(ctx, natsHeaderCarrier{headers: &msg.Header})
}

// ExtractNatsTraceContext extracts OpenTelemetry context from NATS message headers.
func ExtractNatsTraceContext(ctx context.Context, msg *nats.Msg) context.Context {
	prop := otel.GetTextMapPropagator()
	return prop.Extract(ctx, natsHeaderCarrier{headers: &msg.Header})
}

type natsHeaderCarrier struct {
	headers *nats.Header
}

func (c natsHeaderCarrier) Get(key string) string {
	return (*c.headers).Get(key)
}

func (c natsHeaderCarrier) Set(key string, value string) {
	(*c.headers).Set(key, value)
}

func (c natsHeaderCarrier) Keys() []string {
	out := make([]string, 0, len(*c.headers))
	for k := range *c.headers {
		out = append(out, k)
	}
	return out
}

// PublishNatsWithTrace publishes a message to NATS with OpenTelemetry trace context injected into headers.
func PublishNatsWithTrace(ctx context.Context, nc *nats.Conn, subject string, data []byte) error {
	msg := &nats.Msg{
		Subject: subject,
		Data:    data,
		Header:  nats.Header{},
	}

	InjectNatsTraceContext(ctx, msg)

	return nc.PublishMsg(msg)
}

// CreateNatsSubscriber creates a NATS subscriber that extracts OpenTelemetry context from message headers.
func CreateNatsSubscriber(nc *nats.Conn, subject string, handler func(context.Context, *nats.Msg)) (*nats.Subscription, error) {
	return nc.Subscribe(subject, func(msg *nats.Msg) {
		ctx := ExtractNatsTraceContext(context.Background(), msg)
		handler(ctx, msg)
	})
}
