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
	const sigSep = "\x1f" // ASCII Unit Separator (non-printable, collision-safe)
	fields := []string{
		string(cfg.Provider),
		strings.TrimSpace(cfg.DSN),
		strings.TrimSpace(cfg.Path),
		strings.TrimSpace(cfg.Table),
		strings.TrimSpace(cfg.Collection),
		strings.TrimSpace(cfg.Namespace),
		strings.TrimSpace(cfg.Index),
		strings.TrimSpace(cfg.Metric),
		strings.TrimSpace(cfg.Consistency),
		fmt.Sprintf("%d", cfg.Dimension),
		fmt.Sprintf("%t", cfg.EnsureIndex),
		hashStringMap(cfg.Auth),
		hashOptionsMap(cfg.Options),
	}
	fields = append(fields, pgVectorSignature(cfg.PGVector)...)
	return strings.Join(fields, sigSep)
}

func pgVectorSignature(opts *PGVectorOptions) []string {
	if opts == nil {
		return nil
	}
	index := opts.Index
	pool := opts.Pool
	search := opts.Search
	return []string{
		strings.TrimSpace(index.Type),
		fmt.Sprintf("%d", index.Lists),
		fmt.Sprintf("%d", index.Probes),
		fmt.Sprintf("%d", index.M),
		fmt.Sprintf("%d", index.EFConstruction),
		fmt.Sprintf("%d", index.EFSearch),
		fmt.Sprintf("%d", pool.MinConns),
		fmt.Sprintf("%d", pool.MaxConns),
		pool.MaxConnLifetime.String(),
		pool.MaxConnIdleTime.String(),
		pool.HealthCheckPeriod.String(),
		fmt.Sprintf("%d", search.Probes),
		fmt.Sprintf("%d", search.EFSearch),
	}
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
