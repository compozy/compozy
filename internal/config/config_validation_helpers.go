package config

import (
	"errors"
	"fmt"

	"math"

	"slices"
	"strings"
)

func validateEnum(path string, value string, allowed ...string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if slices.Contains(allowed, normalized) {
		return normalized, nil
	}
	return "", fmt.Errorf("%s must be one of %s: %q", path, strings.Join(allowed, ", "), value)
}

func validateWeight(path string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("%s must be finite: %v", path, value)
	}
	if value < 0 || value > 1 {
		return fmt.Errorf("%s must be between 0 and 1: %v", path, value)
	}
	return nil
}

func validateWeightSum(path string, sum float64) error {
	if math.Abs(sum-1.0) > 0.000001 {
		return fmt.Errorf("%s must sum to 1.0: %v", path, sum)
	}
	return nil
}

func validateSafePathSegment(path string, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("%s is required", path)
	}
	if trimmed == "." || trimmed == ".." || strings.ContainsAny(trimmed, `/\`) {
		return fmt.Errorf("%s must be a safe single path segment: %q", path, value)
	}
	return nil
}

func normalizeConfigPaths(cfg *Config) error {
	if cfg == nil {
		return errors.New("config is required")
	}

	socket, err := expandUserPath(cfg.Daemon.Socket)
	if err != nil {
		return fmt.Errorf("expand daemon.socket: %w", err)
	}
	cfg.Daemon.Socket = socket

	if strings.TrimSpace(cfg.Memory.GlobalDir) != "" {
		memoryDir, err := expandUserPath(cfg.Memory.GlobalDir)
		if err != nil {
			return fmt.Errorf("expand memory.global_dir: %w", err)
		}
		cfg.Memory.GlobalDir = memoryDir
	}
	if strings.TrimSpace(cfg.Memory.Extractor.InboxPath) != "" {
		inboxPath, err := expandUserPath(cfg.Memory.Extractor.InboxPath)
		if err != nil {
			return fmt.Errorf("expand memory.extractor.inbox_path: %w", err)
		}
		cfg.Memory.Extractor.InboxPath = inboxPath
	}
	if strings.TrimSpace(cfg.Memory.Extractor.DLQPath) != "" {
		dlqPath, err := expandUserPath(cfg.Memory.Extractor.DLQPath)
		if err != nil {
			return fmt.Errorf("expand memory.extractor.dlq_path: %w", err)
		}
		cfg.Memory.Extractor.DLQPath = dlqPath
	}
	if strings.TrimSpace(cfg.Memory.Session.LedgerRoot) != "" {
		ledgerRoot, err := expandUserPath(cfg.Memory.Session.LedgerRoot)
		if err != nil {
			return fmt.Errorf("expand memory.session.ledger_root: %w", err)
		}
		cfg.Memory.Session.LedgerRoot = ledgerRoot
	}

	return nil
}
