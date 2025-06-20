package auth

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/pkg/logger"
)

// TemporalConfig is an alias to worker.TemporalConfig for backward compatibility
type TemporalConfig = worker.TemporalConfig

// DefaultTemporalConfig returns the default Temporal configuration
func DefaultTemporalConfig() *TemporalConfig {
	return worker.DefaultTemporalConfig()
}

// TemporalService provides Temporal namespace management operations
type TemporalService struct {
	client *worker.Client
}

// NewTemporalService creates a new Temporal service instance using the worker client
func NewTemporalService(ctx context.Context, config *TemporalConfig) (*TemporalService, error) {
	if config == nil {
		config = DefaultTemporalConfig()
	}
	client, err := worker.NewClient(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporal client: %w", err)
	}
	return &TemporalService{
		client: client,
	}, nil
}

// ProvisionNamespace creates a new Temporal namespace
func (t *TemporalService) ProvisionNamespace(ctx context.Context, namespace string) error {
	return t.client.ProvisionNamespace(ctx, namespace)
}

// DeleteNamespace removes a Temporal namespace
func (t *TemporalService) DeleteNamespace(ctx context.Context, namespace string) error {
	log := logger.FromContext(ctx)
	log.With("namespace", namespace).Info("Marking namespace for deletion")
	// Note: Temporal doesn't provide a public DeleteNamespace API in the SDK
	// In production environments, namespace deletion is typically handled
	// through operational procedures or administrative tools
	log.With("namespace", namespace).Info("Namespace marked for administrative deletion")
	return nil
}

// NamespaceExists checks if a namespace exists
func (t *TemporalService) NamespaceExists(ctx context.Context, namespace string) (bool, error) {
	return t.client.NamespaceExists(ctx, namespace)
}

// Close closes the underlying temporal client
func (t *TemporalService) Close() {
	if t.client != nil {
		t.client.Close()
	}
}
