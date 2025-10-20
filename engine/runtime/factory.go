package runtime

import (
	"context"
	"fmt"

	appconfig "github.com/compozy/compozy/pkg/config"
)

// Factory creates Runtime instances based on configuration.
// This allows for flexible runtime selection and easier testing.
type Factory interface {
	// CreateRuntime creates a new Runtime instance for the given configuration.
	// The runtime type is determined by the config.RuntimeType field.
	CreateRuntime(ctx context.Context, config *Config) (Runtime, error)
	// CreateRuntimeFromAppConfig creates a new Runtime instance from unified app config.
	// This method uses the new direct configuration mapping approach.
	CreateRuntimeFromAppConfig(ctx context.Context, appConfig *appconfig.RuntimeConfig) (Runtime, error)
}

// DefaultFactory is the default implementation of the Factory interface
type DefaultFactory struct {
	projectRoot string
}

// NewDefaultFactory creates a new DefaultFactory with the specified project root.
// The project root is used to resolve relative paths in runtime configurations.
func NewDefaultFactory(projectRoot string) Factory {
	return &DefaultFactory{
		projectRoot: projectRoot,
	}
}

// CreateRuntime creates the appropriate runtime based on the configuration.
// It supports "bun" and "node" runtime types, defaulting to "bun" if not specified.
func (f *DefaultFactory) CreateRuntime(ctx context.Context, config *Config) (Runtime, error) {
	if config == nil {
		return nil, fmt.Errorf("runtime config must not be nil")
	}
	runtimeType := config.RuntimeType
	if runtimeType == "" {
		runtimeType = RuntimeTypeBun
	}
	if !IsValidRuntimeType(runtimeType) {
		return nil, fmt.Errorf("unsupported runtime type: %s (supported types: %v)", runtimeType, SupportedRuntimeTypes)
	}
	switch runtimeType {
	case RuntimeTypeBun:
		return NewBunManager(ctx, f.projectRoot, config)
	case RuntimeTypeNode:
		// TODO: Implement NewNodeManager in a future task
		return nil, fmt.Errorf("node.js runtime not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported runtime type: %s", runtimeType)
	}
}

// CreateRuntimeFromAppConfig creates a runtime using unified app configuration
func (f *DefaultFactory) CreateRuntimeFromAppConfig(
	ctx context.Context,
	appConfig *appconfig.RuntimeConfig,
) (Runtime, error) {
	if appConfig == nil {
		return nil, fmt.Errorf("runtime app config must not be nil")
	}
	// TODO: Add runtime type to appconfig.RuntimeConfig when needed
	return NewBunManagerFromConfig(ctx, f.projectRoot, appConfig)
}
