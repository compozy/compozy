package mcpproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	sdk "github.com/echovault/sugardb/sugardb"
)

const (
	mcpKeyPrefix    = "mcps:"
	statusKeyPrefix = "status:"
	mcpIndexKey     = "mcps:index"
)

// SugarDBStorage implements Storage using embedded SugarDB. It is intended
// for standalone mode where no external infrastructure is required.
type SugarDBStorage struct {
	db     *sdk.SugarDB
	prefix string
}

// NewSugarDBStorage creates a new SugarDB-backed storage instance.
func NewSugarDBStorage(ctx context.Context) (*SugarDBStorage, error) {
	cfg := config.FromContext(ctx)
	base := config.SugarDBBaseDir(cfg)
	dataDir := filepath.Join(base, "mcp")

	conf := sdk.DefaultConfig()
	conf.DataDir = dataDir

	db, err := sdk.NewSugarDB(
		sdk.WithConfig(conf),
		sdk.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init sugardb: %w", err)
	}
	return &SugarDBStorage{db: db, prefix: "mcp_proxy"}, nil
}

func (s *SugarDBStorage) Close() error { return nil }

// Keys
func (s *SugarDBStorage) getMCPKey(name string) string { return s.prefix + ":" + mcpKeyPrefix + name }
func (s *SugarDBStorage) getStatusKey(name string) string {
	return s.prefix + ":" + statusKeyPrefix + name
}
func (s *SugarDBStorage) indexKey() string { return s.prefix + ":" + mcpIndexKey }

// SaveMCP stores a definition and maintains an index of MCP names for listing.
func (s *SugarDBStorage) SaveMCP(ctx context.Context, def *MCPDefinition) error {
	if def == nil {
		return fmt.Errorf("definition cannot be nil")
	}
	if err := def.Validate(); err != nil {
		return fmt.Errorf("invalid definition: %w", err)
	}
	def.SetDefaults()

	b, err := json.Marshal(def)
	if err != nil {
		return fmt.Errorf("marshal definition: %w", err)
	}
	// Add to index first to avoid invisible saved defs on partial failures
	if _, err := s.db.SAdd(s.indexKey(), def.Name); err != nil {
		return fmt.Errorf("sugardb SADD index failed: %w", err)
	}
	if _, _, err := s.db.Set(s.getMCPKey(def.Name), string(b), sdk.SETOptions{}); err != nil {
		// best-effort rollback of index entry
		if _, rerr := s.db.SRem(s.indexKey(), def.Name); rerr != nil {
			logger.FromContext(ctx).Warn("index rollback failed after SET error", "name", def.Name, "error", rerr)
		}
		return fmt.Errorf("sugardb SET failed: %w", err)
	}
	return nil
}

// LoadMCP loads a definition by name.
func (s *SugarDBStorage) LoadMCP(_ context.Context, name string) (*MCPDefinition, error) {
	if name == "" {
		return nil, fmt.Errorf("name cannot be empty")
	}
	v, err := s.db.Get(s.getMCPKey(name))
	if err != nil || v == "" {
		return nil, fmt.Errorf("MCP definition '%s' not found", name)
	}
	var def MCPDefinition
	if err := json.Unmarshal([]byte(v), &def); err != nil {
		return nil, fmt.Errorf("unmarshal definition: %w", err)
	}
	def.SetDefaults()
	return &def, nil
}

// DeleteMCP removes a definition, its status, and updates the index.
func (s *SugarDBStorage) DeleteMCP(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	// Try delete definition first
	n, err := s.db.Del(s.getMCPKey(name))
	if err != nil {
		return fmt.Errorf("sugardb DEL failed: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("MCP definition '%s' not found", name)
	}
	// Best-effort cleanups
	if _, err := s.db.Del(s.getStatusKey(name)); err != nil {
		logger.FromContext(ctx).Warn("Failed to delete status during MCP deletion", "name", name, "error", err)
	}
	if _, err := s.db.SRem(s.indexKey(), name); err != nil {
		logger.FromContext(ctx).Warn("Failed to remove MCP from index", "name", name, "error", err)
	}
	return nil
}

// ListMCPs lists definitions using the maintained index + MGET for efficiency.
func (s *SugarDBStorage) ListMCPs(ctx context.Context) ([]*MCPDefinition, error) {
	names, err := s.db.SMembers(s.indexKey())
	if err != nil {
		return nil, fmt.Errorf("sugardb SMEMBERS failed: %w", err)
	}
	if len(names) == 0 {
		return []*MCPDefinition{}, nil
	}
	keys := make([]string, 0, len(names))
	for _, n := range names {
		keys = append(keys, s.getMCPKey(n))
	}
	values, err := s.db.MGet(keys...)
	if err != nil {
		return nil, fmt.Errorf("sugardb MGET failed: %w", err)
	}
	defs := make([]*MCPDefinition, 0, len(values))
	for i, raw := range values {
		if raw == "" {
			logger.FromContext(ctx).Debug("MCP name present in index but key missing", "name", names[i])
			continue
		}
		var def MCPDefinition
		if err := json.Unmarshal([]byte(raw), &def); err != nil {
			logger.FromContext(ctx).Warn("Skipping corrupt MCP definition in storage", "name", names[i], "error", err)
			continue
		}
		def.SetDefaults()
		d := def
		defs = append(defs, &d)
	}
	return defs, nil
}

// SaveStatus persists status for a given MCP.
func (s *SugarDBStorage) SaveStatus(_ context.Context, status *MCPStatus) error {
	if status == nil {
		return fmt.Errorf("status cannot be nil")
	}
	if status.Name == "" {
		return fmt.Errorf("status name cannot be empty")
	}
	b, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("marshal status: %w", err)
	}
	if _, _, err := s.db.Set(s.getStatusKey(status.Name), string(b), sdk.SETOptions{}); err != nil {
		return fmt.Errorf("sugardb SET status failed: %w", err)
	}
	return nil
}

// LoadStatus loads previously saved status or returns a default one if not present.
func (s *SugarDBStorage) LoadStatus(_ context.Context, name string) (*MCPStatus, error) {
	if name == "" {
		return nil, fmt.Errorf("name cannot be empty")
	}
	v, err := s.db.Get(s.getStatusKey(name))
	if err != nil || v == "" {
		return NewMCPStatus(name), nil
	}
	var st MCPStatus
	if err := json.Unmarshal([]byte(v), &st); err != nil {
		return nil, fmt.Errorf("unmarshal status: %w", err)
	}
	return &st, nil
}
