package settings

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"fmt"

	"slices"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/config/lifecycle"
)

func (s *service) ensureActiveConfigState(ctx context.Context) (activeSnapshot, error) {
	s.activeConfig.mu.Lock()
	defer s.activeConfig.mu.Unlock()
	if s.activeConfig.initialized {
		return activeSnapshot{
			hash:       s.activeConfig.hash,
			generation: s.activeConfig.generation,
			config:     cloneActiveConfig(&s.activeConfig.config),
		}, nil
	}

	hash, cfg, err := s.currentDesiredConfigHash()
	if err != nil {
		return activeSnapshot{}, err
	}
	generation := int64(0)
	if s.applyRecords != nil {
		latest, latestErr := s.applyRecords.LatestAppliedRecord(ctx)
		if latestErr != nil {
			return activeSnapshot{}, latestErr
		}
		if latest != nil {
			generation = latest.Generation
			// A daemon boot makes the current file active even when the last applied record predates restart.
			if strings.TrimSpace(latest.ActiveHash) != "" && latest.ActiveHash != hash {
				generation++
			}
		}
	}
	s.activeConfig.initialized = true
	s.activeConfig.hash = hash
	s.activeConfig.generation = generation
	s.activeConfig.config = cfg
	return activeSnapshot{hash: hash, generation: generation, config: cloneActiveConfig(&cfg)}, nil
}

type activeSnapshot struct {
	hash       string
	generation int64
	config     aghconfig.Config
}

func (s *service) advanceActiveConfig(cfg *aghconfig.Config, hash string, generation int64) {
	s.activeConfig.mu.Lock()
	defer s.activeConfig.mu.Unlock()
	s.activeConfig.initialized = true
	s.activeConfig.config = cloneActiveConfig(cfg)
	s.activeConfig.hash = hash
	s.activeConfig.generation = generation
}

func (s *service) currentDesiredConfigHash() (string, aghconfig.Config, error) {
	cfg, err := aghconfig.LoadForHome(s.homePaths)
	if err != nil {
		return "", aghconfig.Config{}, fmt.Errorf("settings: load desired config: %w", err)
	}
	hash, err := hashConfigSnapshot(&cfg)
	if err != nil {
		return "", aghconfig.Config{}, err
	}
	return hash, cfg, nil
}

func hashConfigSnapshot(cfg *aghconfig.Config) (string, error) {
	bytes, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("settings: marshal config snapshot: %w", err)
	}
	sum := sha256.Sum256(bytes)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func mutationResultHasNoChanges(result MutationResult) bool {
	return slices.Contains(result.Warnings, sectionsNoChangesValue)
}

func mutationLifecycle(result MutationResult) lifecycle.Lifecycle {
	if result.Lifecycle != "" {
		return result.Lifecycle
	}
	if result.DiffClass != "" {
		return lifecycle.Lifecycle(result.DiffClass)
	}
	return lifecycle.RestartRequired
}

func skippedReason(skipped bool) string {
	if !skipped {
		return ""
	}
	return configApplyNoChangesReason
}

func skippedReloadResult(state *activeSnapshot, desiredHash string) ApplyResult {
	return ApplyResult{
		Record: ApplyRecord{
			DesiredHash: desiredHash,
			ActiveHash:  state.hash,
			Generation:  state.generation,
			Lifecycle:   lifecycle.Live,
			DiffClass:   lifecycle.DiffClassLive,
			Status:      lifecycle.StatusApplied,
			NextAction:  lifecycle.NextActionNone,
		},
		Applied:       true,
		NextAction:    lifecycle.NextActionNone,
		Skipped:       true,
		SkippedReason: configApplyNoChangesReason,
	}
}

func newRuntimeApplyPlan(
	state *activeSnapshot,
	desiredHash string,
	configLifecycle lifecycle.Lifecycle,
	noChanges bool,
) runtimeApplyPlan {
	plan := runtimeApplyPlan{
		status:     lifecycle.StatusApplied,
		activeHash: desiredHash,
		generation: state.generation,
		applied:    true,
	}
	if !noChanges {
		plan.generation = state.generation + 1
	}
	if configLifecycle == lifecycle.RestartRequired {
		plan.status = lifecycle.StatusBlocked
		plan.activeHash = state.hash
		plan.generation = state.generation
		plan.applied = false
	}
	return plan
}
