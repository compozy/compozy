package sugardb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	sdk "github.com/echovault/sugardb/sugardb"
)

// Server wraps an embedded SugarDB instance for standalone mode.
type Server struct{ db *sdk.SugarDB }

// NewEmbedded creates a new embedded SugarDB instance.
func NewEmbedded(ctx context.Context) (*Server, error) {
	log := logger.FromContext(ctx)

	base := config.SugarDBBaseDir(config.FromContext(ctx))
	dataDir := filepath.Join(base, "cache")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create sugardb cache dir: %w", err)
	}

	conf := sdk.DefaultConfig()
	// Note: DataDir controls AOF/WAL/Snapshot location per SugarDB docs.
	conf.DataDir = dataDir

	db, err := sdk.NewSugarDB(
		sdk.WithConfig(conf),
		sdk.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("sugardb init failed: %w", err)
	}
	log.With("cache_driver", "sugardb", "data_dir", dataDir).Info("SugarDB embedded initialized")
	return &Server{db: db}, nil
}

func (s *Server) DB() *sdk.SugarDB { return s.db }

// HealthCheck performs a minimal R/W/TTL cycle.
func (s *Server) HealthCheck(_ context.Context) error {
	k := "health:check"
	if _, _, err := s.db.Set(k, "ok", sdk.SETOptions{ExpireOpt: sdk.SETPX, ExpireTime: 500}); err != nil {
		return err
	}
	v, err := s.db.Get(k)
	if err != nil {
		return err
	}
	if v != "ok" {
		return fmt.Errorf("health value mismatch: %q", v)
	}
	if ok, err := s.db.PExpire(k, 50); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("pexpire failed")
	}
	time.Sleep(60 * time.Millisecond)
	v, err = s.db.Get(k)
	if err != nil {
		return err
	}
	if v != "" {
		return fmt.Errorf("expected key to be expired")
	}
	if _, err := s.db.Del(k); err != nil {
		return err
	}
	return nil
}

// defaultDBBasePath resolves the base path for embedded SugarDB persistence.
// If cfg is nil or empty, defaults to ~/.compozy.
// base path resolution centralized in pkg/config.SugarDBBaseDir
