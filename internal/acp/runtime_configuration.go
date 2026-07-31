package acp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	speedpkg "github.com/compozy/compozy/internal/speed"
)

// RuntimeConfig identifies mutable ACP settings requested for one prompt boundary.
type RuntimeConfig struct {
	Model           string
	ReasoningEffort string
	Speed           speedpkg.Speed
}

func (c RuntimeConfig) normalize() (RuntimeConfig, error) {
	normalized := RuntimeConfig{
		Model:           strings.TrimSpace(c.Model),
		ReasoningEffort: strings.TrimSpace(c.ReasoningEffort),
		Speed:           c.Speed,
	}
	if normalized.Speed == "" {
		return normalized, nil
	}
	parsed, err := speedpkg.Parse(string(normalized.Speed))
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("acp: validate runtime speed: %w", err)
	}
	normalized.Speed = parsed
	return normalized, nil
}

// ConfigureRuntime applies model, refreshed capabilities, reasoning, and speed in order.
func (d *Driver) ConfigureRuntime(ctx context.Context, process *AgentProcess, config RuntimeConfig) error {
	if d == nil {
		return errors.New("acp: driver is required")
	}
	if ctx == nil {
		return errors.New("acp: context is required")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("acp: configure runtime: %w", err)
	}
	if process == nil {
		return errors.New("acp: agent process is required")
	}
	if process.conn == nil {
		return errProcessConnectionUninitialized
	}
	if strings.TrimSpace(process.SessionID) == "" {
		return errors.New("acp: process session is not initialized")
	}

	normalized, err := config.normalize()
	if err != nil {
		return err
	}

	process.runtimeConfigMu.Lock()
	defer process.runtimeConfigMu.Unlock()

	if _, err := d.applySessionModel(ctx, process, normalized.Model); err != nil {
		return fmt.Errorf("acp: configure runtime model: %w", err)
	}
	if _, err := d.applySessionReasoningEffort(ctx, process, normalized.ReasoningEffort); err != nil {
		return fmt.Errorf("acp: configure runtime reasoning effort: %w", err)
	}
	if _, err := d.applySessionSpeed(ctx, process, normalized.Speed); err != nil {
		return fmt.Errorf("acp: configure runtime speed: %w", err)
	}
	return nil
}
