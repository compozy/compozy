package task

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	enginetask "github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

// New creates a basic task configuration using functional options
func New(ctx context.Context, id string, opts ...Option) (*enginetask.Config, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("creating basic task configuration", "task", id)
	cfg := &enginetask.Config{
		BaseConfig: enginetask.BaseConfig{
			ID:   strings.TrimSpace(id),
			Type: enginetask.TaskTypeBasic,
		},
	}
	for _, opt := range opts {
		opt(cfg)
	}
	collected := make([]error, 0)
	if err := validateID(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateAgentOrTool(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateTimeout(cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateSleep(cfg); err != nil {
		collected = append(collected, err)
	}
	filtered := filterErrors(collected)
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}
	cloned, err := core.DeepCopy(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to clone task config: %w", err)
	}
	return cloned, nil
}

// NewRouter creates a router task configuration using functional options
func NewRouter(ctx context.Context, id string, opts ...Option) (*enginetask.Config, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("creating router task configuration", "task", id)
	cfg := &enginetask.Config{
		BaseConfig: enginetask.BaseConfig{
			ID:   strings.TrimSpace(id),
			Type: enginetask.TaskTypeRouter,
		},
	}
	for _, opt := range opts {
		opt(cfg)
	}
	collected := make([]error, 0)
	if err := validateID(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateCondition(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateRoutes(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	filtered := filterErrors(collected)
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}
	cloned, err := core.DeepCopy(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to clone router task config: %w", err)
	}
	return cloned, nil
}

// NewParallel creates a parallel task configuration using functional options
func NewParallel(
	ctx context.Context,
	id string,
	tasks []enginetask.Config,
	opts ...Option,
) (*enginetask.Config, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("creating parallel task configuration", "task", id)
	cfg := &enginetask.Config{
		BaseConfig: enginetask.BaseConfig{
			ID:   strings.TrimSpace(id),
			Type: enginetask.TaskTypeParallel,
		},
		Tasks: tasks,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	collected := make([]error, 0)
	if err := validateID(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateParallelTasks(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateParallelStrategy(cfg); err != nil {
		collected = append(collected, err)
	}
	filtered := filterErrors(collected)
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}
	cloned, err := core.DeepCopy(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to clone parallel task config: %w", err)
	}
	return cloned, nil
}

// NewCollection creates a collection task configuration using functional options
func NewCollection(ctx context.Context, id string, items string, opts ...Option) (*enginetask.Config, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("creating collection task configuration", "task", id)
	cfg := &enginetask.Config{
		BaseConfig: enginetask.BaseConfig{
			ID:   strings.TrimSpace(id),
			Type: enginetask.TaskTypeCollection,
		},
		CollectionConfig: enginetask.CollectionConfig{
			Items: strings.TrimSpace(items),
		},
	}
	for _, opt := range opts {
		opt(cfg)
	}
	collected := make([]error, 0)
	if err := validateID(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateCollectionItems(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateCollectionTask(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	filtered := filterErrors(collected)
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}
	cloned, err := core.DeepCopy(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to clone collection task config: %w", err)
	}
	return cloned, nil
}

// NewWait creates a wait task configuration using functional options
func NewWait(ctx context.Context, id string, waitFor string, opts ...Option) (*enginetask.Config, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("creating wait task configuration", "task", id)
	cfg := &enginetask.Config{
		BaseConfig: enginetask.BaseConfig{
			ID:   strings.TrimSpace(id),
			Type: enginetask.TaskTypeWait,
		},
		WaitTask: enginetask.WaitTask{
			WaitFor: strings.TrimSpace(waitFor),
		},
	}
	for _, opt := range opts {
		opt(cfg)
	}
	collected := make([]error, 0)
	if err := validateID(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateWaitFor(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	filtered := filterErrors(collected)
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}
	cloned, err := core.DeepCopy(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to clone wait task config: %w", err)
	}
	return cloned, nil
}

// NewSignal creates a signal task configuration using functional options
func NewSignal(ctx context.Context, id string, signalID string, opts ...Option) (*enginetask.Config, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("creating signal task configuration", "task", id)
	cfg := &enginetask.Config{
		BaseConfig: enginetask.BaseConfig{
			ID:   strings.TrimSpace(id),
			Type: enginetask.TaskTypeSignal,
		},
		SignalTask: enginetask.SignalTask{
			Signal: &enginetask.SignalConfig{
				ID: strings.TrimSpace(signalID),
			},
		},
	}
	for _, opt := range opts {
		opt(cfg)
	}
	collected := make([]error, 0)
	if err := validateID(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateSignal(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	filtered := filterErrors(collected)
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}
	cloned, err := core.DeepCopy(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to clone signal task config: %w", err)
	}
	return cloned, nil
}

// NewMemory creates a memory task configuration using functional options
func NewMemory(
	ctx context.Context,
	id string,
	operation enginetask.MemoryOpType,
	opts ...Option,
) (*enginetask.Config, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("creating memory task configuration", "task", id)
	cfg := &enginetask.Config{
		BaseConfig: enginetask.BaseConfig{
			ID:   strings.TrimSpace(id),
			Type: enginetask.TaskTypeMemory,
		},
		MemoryTask: enginetask.MemoryTask{
			Operation: operation,
		},
	}
	for _, opt := range opts {
		opt(cfg)
	}
	collected := make([]error, 0)
	if err := validateID(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateMemoryOperation(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	filtered := filterErrors(collected)
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}
	cloned, err := core.DeepCopy(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to clone memory task config: %w", err)
	}
	return cloned, nil
}

// Validation helper functions

func validateID(ctx context.Context, cfg *enginetask.Config) error {
	cfg.ID = strings.TrimSpace(cfg.ID)
	if err := validate.ID(ctx, cfg.ID); err != nil {
		return fmt.Errorf("task id is invalid: %w", err)
	}
	return nil
}

func validateAgentOrTool(_ context.Context, cfg *enginetask.Config) error {
	hasAgent := cfg.Agent != nil && (cfg.Agent.ID != "" || cfg.Agent.Instructions != "")
	hasTool := cfg.Tool != nil && cfg.Tool.ID != ""
	if !hasAgent && !hasTool {
		return fmt.Errorf("either agent or tool must be configured")
	}
	if hasAgent && hasTool {
		return fmt.Errorf("cannot configure both agent and tool")
	}
	if hasAgent {
		cfg.Agent.ID = strings.TrimSpace(cfg.Agent.ID)
		cfg.Agent.Instructions = strings.TrimSpace(cfg.Agent.Instructions)
	}
	if hasTool {
		cfg.Tool.ID = strings.TrimSpace(cfg.Tool.ID)
	}
	return nil
}

func validateTimeout(cfg *enginetask.Config) error {
	if cfg.Timeout == "" {
		return nil
	}
	cfg.Timeout = strings.TrimSpace(cfg.Timeout)
	if _, err := time.ParseDuration(cfg.Timeout); err != nil {
		return fmt.Errorf("invalid timeout duration: %w", err)
	}
	return nil
}

func validateSleep(cfg *enginetask.Config) error {
	if cfg.Sleep == "" {
		return nil
	}
	cfg.Sleep = strings.TrimSpace(cfg.Sleep)
	if _, err := time.ParseDuration(cfg.Sleep); err != nil {
		return fmt.Errorf("invalid sleep duration: %w", err)
	}
	return nil
}

func validateCondition(ctx context.Context, cfg *enginetask.Config) error {
	cfg.Condition = strings.TrimSpace(cfg.Condition)
	if err := validate.NonEmpty(ctx, "condition", cfg.Condition); err != nil {
		return err
	}
	return nil
}

func validateRoutes(_ context.Context, cfg *enginetask.Config) error {
	if len(cfg.Routes) == 0 {
		return fmt.Errorf("at least one route must be defined")
	}
	return nil
}

func validateParallelTasks(_ context.Context, cfg *enginetask.Config) error {
	if len(cfg.Tasks) < 2 {
		return fmt.Errorf("parallel task must have at least 2 tasks")
	}
	for i := range cfg.Tasks {
		if strings.TrimSpace(cfg.Tasks[i].ID) == "" {
			return fmt.Errorf("task at index %d is missing an id", i)
		}
	}
	return nil
}

func validateParallelStrategy(cfg *enginetask.Config) error {
	if cfg.Strategy == "" {
		cfg.Strategy = enginetask.StrategyWaitAll
	}
	validStrategies := map[enginetask.ParallelStrategy]bool{
		enginetask.StrategyWaitAll:    true,
		enginetask.StrategyFailFast:   true,
		enginetask.StrategyBestEffort: true,
		enginetask.StrategyRace:       true,
	}
	if !validStrategies[cfg.Strategy] {
		return fmt.Errorf("invalid parallel strategy: %s", cfg.Strategy)
	}
	return nil
}

func validateCollectionItems(ctx context.Context, cfg *enginetask.Config) error {
	cfg.Items = strings.TrimSpace(cfg.Items)
	if err := validate.NonEmpty(ctx, "items", cfg.Items); err != nil {
		return err
	}
	return nil
}

func validateCollectionTask(_ context.Context, cfg *enginetask.Config) error {
	if cfg.Task == nil {
		return fmt.Errorf("collection task must have a task template")
	}
	if strings.TrimSpace(cfg.Task.ID) == "" {
		return fmt.Errorf("collection task template must have an id")
	}
	return nil
}

func validateWaitFor(ctx context.Context, cfg *enginetask.Config) error {
	cfg.WaitFor = strings.TrimSpace(cfg.WaitFor)
	if err := validate.NonEmpty(ctx, "wait_for", cfg.WaitFor); err != nil {
		return err
	}
	return nil
}

func validateSignal(ctx context.Context, cfg *enginetask.Config) error {
	if cfg.Signal == nil {
		return fmt.Errorf("signal configuration is required")
	}
	cfg.Signal.ID = strings.TrimSpace(cfg.Signal.ID)
	if err := validate.NonEmpty(ctx, "signal id", cfg.Signal.ID); err != nil {
		return err
	}
	return nil
}

func validateMemoryOperation(_ context.Context, cfg *enginetask.Config) error {
	validOps := map[enginetask.MemoryOpType]bool{
		enginetask.MemoryOpRead:   true,
		enginetask.MemoryOpWrite:  true,
		enginetask.MemoryOpAppend: true,
		enginetask.MemoryOpDelete: true,
		enginetask.MemoryOpClear:  true,
		enginetask.MemoryOpFlush:  true,
		enginetask.MemoryOpHealth: true,
		enginetask.MemoryOpStats:  true,
	}
	if !validOps[cfg.Operation] {
		return fmt.Errorf("invalid memory operation: %s", cfg.Operation)
	}
	return nil
}

func filterErrors(errors []error) []error {
	filtered := make([]error, 0, len(errors))
	for _, err := range errors {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	return filtered
}
