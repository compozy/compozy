package workspace

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/providers"
	toml "github.com/pelletier/go-toml/v2"
)

type Context struct {
	Root       string
	CompozyDir string
	ConfigPath string
	Config     ProjectConfig
}

type ProjectConfig struct {
	Defaults     DefaultsConfig     `toml:"defaults"`
	Start        StartConfig        `toml:"start"`
	FixReviews   FixReviewsConfig   `toml:"fix_reviews"`
	FetchReviews FetchReviewsConfig `toml:"fetch_reviews"`
}

type DefaultsConfig struct {
	IDE                    *string   `toml:"ide"`
	Model                  *string   `toml:"model"`
	ReasoningEffort        *string   `toml:"reasoning_effort"`
	Timeout                *string   `toml:"timeout"`
	TailLines              *int      `toml:"tail_lines"`
	AddDirs                *[]string `toml:"add_dirs"`
	AutoCommit             *bool     `toml:"auto_commit"`
	MaxRetries             *int      `toml:"max_retries"`
	RetryBackoffMultiplier *float64  `toml:"retry_backoff_multiplier"`
}

type StartConfig struct {
	IncludeCompleted *bool `toml:"include_completed"`
}

type FixReviewsConfig struct {
	Concurrent      *int  `toml:"concurrent"`
	BatchSize       *int  `toml:"batch_size"`
	Grouped         *bool `toml:"grouped"`
	IncludeResolved *bool `toml:"include_resolved"`
}

type FetchReviewsConfig struct {
	Provider *string `toml:"provider"`
}

func Resolve(startDir string) (Context, error) {
	root, err := Discover(startDir)
	if err != nil {
		return Context{}, err
	}

	cfg, configPath, err := LoadConfig(root)
	if err != nil {
		return Context{}, err
	}

	return Context{
		Root:       root,
		CompozyDir: model.CompozyDir(root),
		ConfigPath: configPath,
		Config:     cfg,
	}, nil
}

func Discover(startDir string) (string, error) {
	resolvedStart := strings.TrimSpace(startDir)
	if resolvedStart == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
		resolvedStart = cwd
	}

	absStart, err := filepath.Abs(resolvedStart)
	if err != nil {
		return "", fmt.Errorf("resolve workspace start dir: %w", err)
	}

	current := absStart
	for {
		candidate := filepath.Join(current, model.WorkflowRootDirName)
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return current, nil
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("stat workspace marker %s: %w", candidate, err)
		}

		parent := filepath.Dir(current)
		if parent == current {
			return absStart, nil
		}
		current = parent
	}
}

func LoadConfig(workspaceRoot string) (ProjectConfig, string, error) {
	configPath := model.ConfigPathForWorkspace(workspaceRoot)
	content, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ProjectConfig{}, configPath, nil
		}
		return ProjectConfig{}, configPath, fmt.Errorf("read workspace config: %w", err)
	}

	var cfg ProjectConfig
	decoder := toml.NewDecoder(bytes.NewReader(content)).DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return ProjectConfig{}, configPath, fmt.Errorf("decode workspace config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return ProjectConfig{}, configPath, err
	}

	return cfg, configPath, nil
}

func (cfg ProjectConfig) Validate() error {
	if err := validateDefaults(cfg.Defaults); err != nil {
		return err
	}
	if err := validateStart(cfg.Start); err != nil {
		return err
	}
	if err := validateFixReviews(cfg.FixReviews); err != nil {
		return err
	}
	if err := validateFetchReviews(cfg.FetchReviews); err != nil {
		return err
	}
	return nil
}

func validateDefaults(cfg DefaultsConfig) error {
	validators := []func(DefaultsConfig) error{
		validateDefaultIDE,
		validateDefaultReasoningEffort,
		validateDefaultTimeout,
		validateDefaultTailLines,
		validateDefaultMaxRetries,
		validateDefaultRetryBackoffMultiplier,
	}
	for _, validate := range validators {
		if err := validate(cfg); err != nil {
			return err
		}
	}
	return nil
}

func validateStart(_ StartConfig) error {
	return nil
}

func validateFixReviews(cfg FixReviewsConfig) error {
	if cfg.Concurrent != nil && *cfg.Concurrent <= 0 {
		return fmt.Errorf("workspace config fix_reviews.concurrent must be greater than zero (got %d)", *cfg.Concurrent)
	}
	if cfg.BatchSize != nil && *cfg.BatchSize <= 0 {
		return fmt.Errorf("workspace config fix_reviews.batch_size must be greater than zero (got %d)", *cfg.BatchSize)
	}
	return nil
}

func validateFetchReviews(cfg FetchReviewsConfig) error {
	if cfg.Provider == nil {
		return nil
	}
	name := strings.TrimSpace(*cfg.Provider)
	if name == "" {
		return errors.New("workspace config fetch_reviews.provider cannot be empty")
	}
	if _, err := providers.DefaultRegistry().Get(name); err != nil {
		return fmt.Errorf("workspace config fetch_reviews.provider: %w", err)
	}
	return nil
}

func validateDefaultIDE(cfg DefaultsConfig) error {
	if cfg.IDE == nil {
		return nil
	}
	if strings.TrimSpace(*cfg.IDE) == "" {
		return errors.New("workspace config defaults.ide cannot be empty")
	}
	if _, err := agent.DriverCatalogEntryForIDE(strings.TrimSpace(*cfg.IDE)); err != nil {
		return fmt.Errorf("workspace config defaults.ide: %w", err)
	}
	return nil
}

func validateDefaultReasoningEffort(cfg DefaultsConfig) error {
	if cfg.ReasoningEffort == nil {
		return nil
	}
	switch strings.TrimSpace(*cfg.ReasoningEffort) {
	case "low", "medium", "high", "xhigh":
		return nil
	default:
		return fmt.Errorf(
			"workspace config defaults.reasoning_effort must be one of low, medium, high, xhigh (got %q)",
			strings.TrimSpace(*cfg.ReasoningEffort),
		)
	}
}

func validateDefaultTimeout(cfg DefaultsConfig) error {
	if cfg.Timeout == nil {
		return nil
	}

	timeout := strings.TrimSpace(*cfg.Timeout)
	if timeout == "" {
		return errors.New("workspace config defaults.timeout cannot be empty")
	}
	duration, err := time.ParseDuration(timeout)
	if err != nil {
		return fmt.Errorf("workspace config defaults.timeout: %w", err)
	}
	if duration <= 0 {
		return fmt.Errorf("workspace config defaults.timeout must be greater than zero (got %s)", timeout)
	}
	return nil
}

func validateDefaultTailLines(cfg DefaultsConfig) error {
	if cfg.TailLines != nil && *cfg.TailLines < 0 {
		return fmt.Errorf("workspace config defaults.tail_lines must be 0 or greater (got %d)", *cfg.TailLines)
	}
	return nil
}

func validateDefaultMaxRetries(cfg DefaultsConfig) error {
	if cfg.MaxRetries != nil && *cfg.MaxRetries < 0 {
		return fmt.Errorf("workspace config defaults.max_retries cannot be negative (got %d)", *cfg.MaxRetries)
	}
	return nil
}

func validateDefaultRetryBackoffMultiplier(cfg DefaultsConfig) error {
	if cfg.RetryBackoffMultiplier != nil && *cfg.RetryBackoffMultiplier <= 0 {
		return fmt.Errorf(
			"workspace config defaults.retry_backoff_multiplier must be positive (got %.2f)",
			*cfg.RetryBackoffMultiplier,
		)
	}
	return nil
}
