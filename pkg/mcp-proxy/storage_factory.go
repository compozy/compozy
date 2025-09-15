package mcpproxy

import (
	"context"
	"fmt"
	"sync"
)

// StorageType represents the type of storage backend
type StorageType string

const (
	StorageTypeRedis   StorageType = "redis"
	StorageTypeMemory  StorageType = "memory"
	StorageTypeSugarDB StorageType = "sugardb"
)

// StorageConfig holds configuration for storage backends
type StorageConfig struct {
	Type  StorageType  `json:"type"`
	Redis *RedisConfig `json:"redis,omitempty"`
}

// DefaultStorageConfig returns a default storage configuration
func DefaultStorageConfig() *StorageConfig {
	return &StorageConfig{
		Type:  StorageTypeRedis,
		Redis: DefaultRedisConfig(),
	}
}

// NewStorage creates a new storage instance based on configuration
func NewStorage(ctx context.Context, config *StorageConfig) (Storage, error) {
	if config == nil {
		config = DefaultStorageConfig()
	}

	switch config.Type {
	case StorageTypeRedis:
		return NewRedisStorage(config.Redis)
	case StorageTypeMemory:
		return NewMemoryStorage(), nil
	case StorageTypeSugarDB:
		return NewSugarDBStorage(ctx)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", config.Type)
	}
}

// DefaultStorageConfigForMode returns a default storage configuration suitable
// for the provided application mode without mutating the legacy behavior of
// DefaultStorageConfig() used by existing tests.
//
// - standalone: SugarDB (embedded, no external infra)
// - distributed: Redis (external)
func DefaultStorageConfigForMode(mode string) *StorageConfig {
	if mode == "standalone" {
		return &StorageConfig{Type: StorageTypeSugarDB}
	}
	return DefaultStorageConfig()
}

// MemoryStorage is a simple in-memory storage implementation for testing
type MemoryStorage struct {
	mcps     map[string]*MCPDefinition
	statuses map[string]*MCPStatus
	mu       sync.RWMutex // Protects both maps
}

// NewMemoryStorage creates a new memory-based storage instance
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		mcps:     make(map[string]*MCPDefinition),
		statuses: make(map[string]*MCPStatus),
	}
}

// SaveMCP saves an MCP definition in memory
func (m *MemoryStorage) SaveMCP(_ context.Context, def *MCPDefinition) error {
	if def == nil {
		return fmt.Errorf("definition cannot be nil")
	}

	if err := def.Validate(); err != nil {
		return fmt.Errorf("invalid definition: %w", err)
	}

	def.SetDefaults()

	// Clone to prevent external modifications
	clone, cerr := def.Clone()
	if cerr != nil {
		return fmt.Errorf("failed to clone definition: %w", cerr)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.mcps[clone.Name] = clone
	return nil
}

// LoadMCP loads an MCP definition from memory
func (m *MemoryStorage) LoadMCP(_ context.Context, name string) (*MCPDefinition, error) {
	if name == "" {
		return nil, fmt.Errorf("name cannot be empty")
	}

	m.mu.RLock()
	def, exists := m.mcps[name]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("MCP definition '%s' not found", name)
	}

	// Return a clone to prevent external modifications
	clone, cerr := def.Clone()
	if cerr != nil {
		return nil, fmt.Errorf("failed to clone definition: %w", cerr)
	}

	return clone, nil
}

// DeleteMCP deletes an MCP definition from memory
func (m *MemoryStorage) DeleteMCP(_ context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.mcps[name]
	if !exists {
		return fmt.Errorf("MCP definition '%s' not found", name)
	}

	delete(m.mcps, name)
	delete(m.statuses, name) // Also remove status
	return nil
}

// ListMCPs lists all MCP definitions in memory
func (m *MemoryStorage) ListMCPs(_ context.Context) ([]*MCPDefinition, error) {
	var definitions []*MCPDefinition

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, def := range m.mcps {
		// Return clones to prevent external modifications
		if clone, err := def.Clone(); err == nil && clone != nil {
			definitions = append(definitions, clone)
		}
	}

	return definitions, nil
}

// SaveStatus saves an MCP status in memory
func (m *MemoryStorage) SaveStatus(_ context.Context, status *MCPStatus) error {
	if status == nil {
		return fmt.Errorf("status cannot be nil")
	}

	if status.Name == "" {
		return fmt.Errorf("status name cannot be empty")
	}

	// Create a copy to prevent external modifications
	statusCopy := status.SafeCopy()

	m.mu.Lock()
	defer m.mu.Unlock()
	m.statuses[status.Name] = statusCopy
	return nil
}

// LoadStatus loads an MCP status from memory
func (m *MemoryStorage) LoadStatus(_ context.Context, name string) (*MCPStatus, error) {
	if name == "" {
		return nil, fmt.Errorf("name cannot be empty")
	}

	m.mu.RLock()
	status, exists := m.statuses[name]
	m.mu.RUnlock()

	if !exists {
		// Return default status if not found
		return NewMCPStatus(name), nil
	}

	// Return a copy to prevent external modifications
	return status.SafeCopy(), nil
}

// Close closes the memory storage (no-op)
func (m *MemoryStorage) Close() error {
	return nil
}
