package vectordb

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/compozy/compozy/pkg/logger"
)

// Manager caches shared vector store instances keyed by configuration ID.
type Manager struct {
	mu     sync.Mutex
	stores map[string]*sharedStoreEntry
}

type sharedStoreEntry struct {
	store     Store
	refs      int
	signature string
}

var defaultManager = NewManager()

// NewManager constructs an empty shared vector store manager.
func NewManager() *Manager {
	return &Manager{stores: make(map[string]*sharedStoreEntry)}
}

// AcquireShared returns a shared vector store instance along with a release function.
func AcquireShared(ctx context.Context, cfg *Config) (Store, func(context.Context) error, error) {
	return defaultManager.AcquireShared(ctx, cfg)
}

// AcquireShared acquires (or creates) a shared store entry keyed by the config ID.
func (m *Manager) AcquireShared(ctx context.Context, cfg *Config) (Store, func(context.Context) error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("vector_db: config is required")
	}
	id := strings.TrimSpace(cfg.ID)
	if id == "" {
		return nil, nil, errMissingID
	}
	signature := signatureKey(cfg)
	m.mu.Lock()
	if entry, ok := m.stores[id]; ok {
		if entry.signature != signature {
			m.mu.Unlock()
			return nil, nil, fmt.Errorf("vector_db %q: configuration mismatch for shared store", id)
		}
		entry.refs++
		store := entry.store
		m.mu.Unlock()
		return store, m.releaseFunc(id, signature), nil
	}
	m.mu.Unlock()

	store, err := instantiateStore(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}

	m.mu.Lock()
	if entry, ok := m.stores[id]; ok {
		if entry.signature != signature {
			m.mu.Unlock()
			if err := store.Close(ctx); err != nil {
				logger.FromContext(ctx).Warn(
					"failed to close redundant vector store",
					"vector_id", id,
					"error", err,
				)
			}
			return nil, nil, fmt.Errorf("vector_db %q: configuration mismatch for shared store", id)
		}
		entry.refs++
		existing := entry.store
		m.mu.Unlock()
		if err := store.Close(ctx); err != nil {
			logger.FromContext(ctx).Warn(
				"failed to close redundant vector store",
				"vector_id", id,
				"error", err,
			)
		}
		return existing, m.releaseFunc(id, signature), nil
	}
	m.stores[id] = &sharedStoreEntry{
		store:     store,
		refs:      1,
		signature: signature,
	}
	m.mu.Unlock()
	return store, m.releaseFunc(id, signature), nil
}

// ResetShared clears all cached stores using the provided context. Intended for tests.
func ResetShared(ctx context.Context) {
	defaultManager.reset(ctx)
}

func (m *Manager) releaseFunc(id string, signature string) func(context.Context) error {
	return func(ctx context.Context) error {
		m.mu.Lock()
		entry, ok := m.stores[id]
		if !ok || entry.signature != signature {
			m.mu.Unlock()
			return nil
		}
		entry.refs--
		if entry.refs > 0 {
			m.mu.Unlock()
			return nil
		}
		delete(m.stores, id)
		store := entry.store
		m.mu.Unlock()
		return store.Close(ctx)
	}
}

func (m *Manager) reset(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, entry := range m.stores {
		_ = entry.store.Close(ctx)
		delete(m.stores, id)
	}
}

func signatureKey(cfg *Config) string {
	builder := strings.Builder{}
	builder.WriteString(string(cfg.Provider))
	builder.WriteString("|")
	builder.WriteString(strings.TrimSpace(cfg.DSN))
	builder.WriteString("|")
	builder.WriteString(strings.TrimSpace(cfg.Path))
	builder.WriteString("|")
	builder.WriteString(strings.TrimSpace(cfg.Table))
	builder.WriteString("|")
	builder.WriteString(strings.TrimSpace(cfg.Collection))
	builder.WriteString("|")
	builder.WriteString(strings.TrimSpace(cfg.Namespace))
	builder.WriteString("|")
	builder.WriteString(strings.TrimSpace(cfg.Index))
	builder.WriteString("|")
	builder.WriteString(strings.TrimSpace(cfg.Metric))
	builder.WriteString("|")
	builder.WriteString(strings.TrimSpace(cfg.Consistency))
	builder.WriteString("|")
	builder.WriteString(fmt.Sprintf("%d", cfg.Dimension))
	builder.WriteString("|")
	builder.WriteString(fmt.Sprintf("%t", cfg.EnsureIndex))
	builder.WriteString("|")
	builder.WriteString(hashStringMap(cfg.Auth))
	builder.WriteString("|")
	builder.WriteString(hashOptionsMap(cfg.Options))
	return builder.String()
}

func hashStringMap(input map[string]string) string {
	if len(input) == 0 {
		return ""
	}
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	builder := strings.Builder{}
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(input[key])
		builder.WriteString(";")
	}
	return builder.String()
}

func hashOptionsMap(input map[string]any) string {
	if len(input) == 0 {
		return ""
	}
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	builder := strings.Builder{}
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(fmt.Sprint(input[key]))
		builder.WriteString(";")
	}
	return builder.String()
}
