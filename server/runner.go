package server

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lordtor/workflow-sdk/registry"
	"github.com/rs/zerolog"
)

// Consumer defines the interface for a NATS consumer that the runner can manage.
type Consumer interface {
	Start(ctx context.Context) error
	Close() error
	IsConnected() bool
	Ready() <-chan struct{}
}

// ManagedServer defines the interface for the HTTP server.
type ManagedServer interface {
	Start(startTime time.Time, natsCheck func() bool, depCheck func() bool)
	Stop() error
}

// RunConfig defines the configuration for the service runner.
type RunConfig struct {
	ServiceName string
	ServiceType string
	Endpoint    string
	Metadata    map[string]string

	NATSURL        string
	NATSPrefix     string
	NATSQueueGroup string

	RegistryURL       string
	HeartbeatInterval time.Duration

	ServerHost string
	ServerPort int

	Consumer        Consumer
	Server          ManagedServer
	DependencyCheck func() bool
}

// Run is the universal entry point for workflow services.
func Run(ctx context.Context, cfg RunConfig) error {
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("service", cfg.ServiceName).Logger()
	logger.Info().Msgf("Starting %s...", cfg.ServiceName)

	// 1. NATS Consumer Startup
	go func() {
		if cfg.Consumer == nil {
			return
		}
		if err := cfg.Consumer.Start(ctx); err != nil && err != context.Canceled {
			logger.Error().Err(err).Msg("NATS consumer error")
		}
	}()

	if cfg.Consumer != nil {
		select {
		case <-cfg.Consumer.Ready():
			logger.Info().Msg("NATS consumer ready")
		case <-time.After(5 * time.Second):
			logger.Warn().Msg("Consumer readiness timeout, continuing anyway")
		}
	}

	// 2. Registry Setup
	registryClient := registry.NewClient(
		cfg.RegistryURL,
		cfg.ServiceName,
		cfg.ServiceType,
		cfg.Endpoint,
		cfg.Metadata,
	)
	if err := registryClient.Register(ctx); err != nil {
		logger.Warn().Err(err).Msg("Failed to register with engine - service can still run")
	}
	go registryClient.StartHeartbeat(ctx, cfg.HeartbeatInterval)
	defer registryClient.Stop()

	// 3. HTTP Server Startup
	startTime := time.Now()
	if cfg.Server != nil {
		go cfg.Server.Start(
			startTime,
			func() bool { return cfg.Consumer != nil && cfg.Consumer.IsConnected() },
			cfg.DependencyCheck,
		)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
	case <-ctx.Done():
		logger.Info().Msg("Context cancelled")
	}

	logger.Info().Msg("Shutting down...")

	if cfg.Consumer != nil {
		cfg.Consumer.Close()
	}

	if cfg.Server != nil {
		if err := cfg.Server.Stop(); err != nil {
			logger.Warn().Err(err).Msg("Failed to stop HTTP server")
		}
	}

	if err := registryClient.Unregister(context.Background()); err != nil {
		logger.Warn().Err(err).Msg("Failed to unregister")
	}

	return nil
}
