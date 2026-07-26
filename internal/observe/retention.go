package observe

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/store"
)

const (
	defaultRetentionSweepInterval = 24 * time.Hour

	retentionSweepStatusDisabled = "disabled"
	retentionSweepStatusPending  = "pending"
	retentionSweepStatusOK       = "ok"
	retentionSweepStatusError    = "error"
)

// RetentionConfig controls observer retention sweeps.
type RetentionConfig struct {
	Enabled       bool
	RetentionDays int
	SweepInterval time.Duration
}

// RetentionHealth captures the operator-visible state of observability retention.
type RetentionHealth struct {
	Enabled                  bool       `json:"enabled"`
	RetentionDays            int        `json:"retention_days"`
	SweepIntervalSeconds     int64      `json:"sweep_interval_seconds"`
	LastSweepStatus          string     `json:"last_sweep_status"`
	LastSweepAt              *time.Time `json:"last_sweep_at,omitempty"`
	LastCutoffAt             *time.Time `json:"last_cutoff_at,omitempty"`
	LastSweepError           string     `json:"last_sweep_error,omitempty"`
	DeletedEventSummaries    int64      `json:"deleted_event_summaries"`
	DeletedTokenStats        int64      `json:"deleted_token_stats"`
	DeletedTokenUsageDaily   int64      `json:"deleted_token_usage_daily"`
	DeletedPermissionLogRows int64      `json:"deleted_permission_log_rows"`
}

type observabilityRetentionStore interface {
	SweepObservability(ctx context.Context, cutoff time.Time) (store.ObservabilityRetentionSweepResult, error)
}

// RetentionConfigFromObservability maps daemon configuration into observer retention settings.
func RetentionConfigFromObservability(cfg aghconfig.ObservabilityConfig) RetentionConfig {
	return RetentionConfig{
		Enabled:       cfg.Enabled && cfg.RetentionDays > 0,
		RetentionDays: cfg.RetentionDays,
		SweepInterval: defaultRetentionSweepInterval,
	}
}

func normalizeRetentionConfig(cfg RetentionConfig) RetentionConfig {
	if cfg.SweepInterval <= 0 {
		cfg.SweepInterval = defaultRetentionSweepInterval
	}
	if cfg.RetentionDays <= 0 {
		cfg.Enabled = false
	}
	return cfg
}

func (cfg RetentionConfig) disabled() bool {
	return !cfg.Enabled || cfg.RetentionDays <= 0
}

// StartRetention starts the owned retention sweep loop.
func (o *Observer) StartRetention(ctx context.Context) error {
	if o == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("observe: retention context is required")
	}

	if o.retention.disabled() {
		o.setRetentionHealth(o.initialRetentionHealth())
		return nil
	}

	o.retentionMu.Lock()
	if o.retentionCancel != nil {
		o.retentionMu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	o.retentionCancel = cancel
	o.retentionWG.Add(1)
	o.retentionMu.Unlock()

	go o.runRetentionLoop(runCtx)
	return nil
}

// ShutdownRetention stops the owned retention sweep loop and waits for it to exit.
func (o *Observer) ShutdownRetention(ctx context.Context) error {
	if o == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	o.retentionMu.Lock()
	cancel := o.retentionCancel
	o.retentionCancel = nil
	o.retentionMu.Unlock()

	if cancel == nil {
		return nil
	}
	cancel()

	done := make(chan struct{})
	go func() {
		o.retentionWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("observe: shutdown retention: %w", ctx.Err())
	}
}

// SweepRetention runs one deterministic retention sweep using the observer clock.
func (o *Observer) SweepRetention(ctx context.Context) (RetentionHealth, error) {
	if o == nil {
		return RetentionHealth{}, errors.New("observe: observer is required")
	}
	if ctx == nil {
		return RetentionHealth{}, errors.New("observe: retention context is required")
	}

	cfg := normalizeRetentionConfig(o.retention)
	if cfg.disabled() {
		health := o.initialRetentionHealth()
		o.setRetentionHealth(health)
		return health, nil
	}

	retentionStore, ok := o.registry.(observabilityRetentionStore)
	if !ok {
		err := errors.New("observe: retention store is unavailable")
		health := o.retentionHealthForError(cfg, o.now(), time.Time{}, err)
		o.setRetentionHealth(health)
		return health, err
	}

	now := o.now().UTC()
	cutoff := now.AddDate(0, 0, -cfg.RetentionDays)
	result, err := retentionStore.SweepObservability(ctx, cutoff)
	if err != nil {
		health := o.retentionHealthForError(cfg, now, cutoff, err)
		o.setRetentionHealth(health)
		return health, fmt.Errorf("observe: sweep retention: %w", err)
	}

	health := RetentionHealth{
		Enabled:                  true,
		RetentionDays:            cfg.RetentionDays,
		SweepIntervalSeconds:     int64(cfg.SweepInterval.Seconds()),
		LastSweepStatus:          retentionSweepStatusOK,
		LastSweepAt:              timePtr(now),
		LastCutoffAt:             timePtr(result.CutoffAt),
		DeletedEventSummaries:    result.DeletedEventSummaries,
		DeletedTokenStats:        result.DeletedTokenStats,
		DeletedTokenUsageDaily:   result.DeletedTokenUsageDaily,
		DeletedPermissionLogRows: result.DeletedPermissionLogs,
	}
	o.setRetentionHealth(health)
	return health, nil
}

func (o *Observer) runRetentionLoop(ctx context.Context) {
	defer o.retentionWG.Done()

	o.sweepRetentionBestEffort(ctx)

	ticker := time.NewTicker(o.retention.SweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.sweepRetentionBestEffort(ctx)
		}
	}
}

func (o *Observer) sweepRetentionBestEffort(ctx context.Context) {
	health, err := o.SweepRetention(ctx)
	if err == nil {
		o.logger.Debug(
			"observe: retention sweep completed",
			"deleted_event_summaries",
			health.DeletedEventSummaries,
			"deleted_token_stats",
			health.DeletedTokenStats,
			"deleted_token_usage_daily",
			health.DeletedTokenUsageDaily,
			"deleted_permission_log_rows",
			health.DeletedPermissionLogRows,
		)
		return
	}
	if errors.Is(err, context.Canceled) {
		return
	}
	o.logger.Warn("observe: retention sweep failed", slog.String("error", err.Error()))
}

func (o *Observer) initialRetentionHealth() RetentionHealth {
	cfg := normalizeRetentionConfig(o.retention)
	status := retentionSweepStatusPending
	if cfg.disabled() {
		status = retentionSweepStatusDisabled
	}
	return RetentionHealth{
		Enabled:              !cfg.disabled(),
		RetentionDays:        cfg.RetentionDays,
		SweepIntervalSeconds: int64(cfg.SweepInterval.Seconds()),
		LastSweepStatus:      status,
	}
}

func (o *Observer) retentionHealthForError(
	cfg RetentionConfig,
	now time.Time,
	cutoff time.Time,
	err error,
) RetentionHealth {
	health := RetentionHealth{
		Enabled:              !cfg.disabled(),
		RetentionDays:        cfg.RetentionDays,
		SweepIntervalSeconds: int64(cfg.SweepInterval.Seconds()),
		LastSweepStatus:      retentionSweepStatusError,
		LastSweepAt:          timePtr(now),
		LastSweepError:       err.Error(),
	}
	if !cutoff.IsZero() {
		health.LastCutoffAt = timePtr(cutoff)
	}
	return health
}

func (o *Observer) setRetentionHealth(health RetentionHealth) {
	o.retentionMu.Lock()
	defer o.retentionMu.Unlock()
	o.retentionHealth = cloneRetentionHealth(health)
}

func (o *Observer) retentionHealthSnapshot() RetentionHealth {
	if o == nil {
		return RetentionHealth{}
	}
	o.retentionMu.RLock()
	defer o.retentionMu.RUnlock()
	return cloneRetentionHealth(o.retentionHealth)
}

func cloneRetentionHealth(health RetentionHealth) RetentionHealth {
	health.LastSweepAt = cloneHealthTime(health.LastSweepAt)
	health.LastCutoffAt = cloneHealthTime(health.LastCutoffAt)
	return health
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}
