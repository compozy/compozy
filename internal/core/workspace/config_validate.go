package workspace

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/providerdefaults"
	"github.com/compozy/compozy/internal/core/tasks"
)

func (cfg ProjectConfig) Validate() error {
	if err := validateDefaults(cfg.Defaults); err != nil {
		return err
	}
	if err := validateStart(cfg.Defaults, cfg.Start); err != nil {
		return err
	}
	if err := validateTasks(cfg.Tasks); err != nil {
		return err
	}
	if err := validateFixReviews(cfg.Defaults, cfg.FixReviews); err != nil {
		return err
	}
	if err := validateFetchReviews(cfg.FetchReviews); err != nil {
		return err
	}
	if err := validateExec(cfg.Defaults, cfg.Exec); err != nil {
		return err
	}
	return nil
}

func validateDefaults(cfg DefaultsConfig) error {
	overrides := RuntimeOverrides(cfg)
	if err := validateRuntimeOverrides("defaults", overrides); err != nil {
		return err
	}
	return validateRuntimeAddDirs("defaults", overrides, nil)
}

func validateStart(defaults DefaultsConfig, cfg StartConfig) error {
	if err := validateOutputFormatValue("start.output_format", cfg.OutputFormat); err != nil {
		return err
	}
	return validateWorkflowTUI("start", defaults, cfg.OutputFormat, cfg.TUI)
}

func validateTasks(cfg TasksConfig) error {
	if cfg.Types == nil {
		return nil
	}
	if len(*cfg.Types) == 0 {
		return errors.New("workspace config tasks.types cannot be empty; omit tasks.types to use built-in defaults")
	}
	if _, err := tasks.NewRegistry(*cfg.Types); err != nil {
		return fmt.Errorf("workspace config tasks.types: %w", err)
	}
	return nil
}

func validateFixReviews(defaults DefaultsConfig, cfg FixReviewsConfig) error {
	if cfg.Concurrent != nil && *cfg.Concurrent <= 0 {
		return fmt.Errorf("workspace config fix_reviews.concurrent must be greater than zero (got %d)", *cfg.Concurrent)
	}
	if cfg.BatchSize != nil && *cfg.BatchSize <= 0 {
		return fmt.Errorf("workspace config fix_reviews.batch_size must be greater than zero (got %d)", *cfg.BatchSize)
	}
	if err := validateOutputFormatValue("fix_reviews.output_format", cfg.OutputFormat); err != nil {
		return err
	}
	return validateWorkflowTUI("fix_reviews", defaults, cfg.OutputFormat, cfg.TUI)
}

func validateFetchReviews(cfg FetchReviewsConfig) error {
	if cfg.Provider == nil {
		return nil
	}
	name := strings.TrimSpace(*cfg.Provider)
	if name == "" {
		return errors.New("workspace config fetch_reviews.provider cannot be empty")
	}
	if _, err := provider.ResolveRegistry(providerdefaults.DefaultRegistry()).Get(name); err != nil {
		return fmt.Errorf("workspace config fetch_reviews.provider: %w", err)
	}
	return nil
}

func validateExec(defaults DefaultsConfig, cfg ExecConfig) error {
	if err := validateRuntimeOverrides("exec", cfg.RuntimeOverrides); err != nil {
		return err
	}
	if err := validateRuntimeAddDirs("exec", cfg.RuntimeOverrides, &defaults); err != nil {
		return err
	}

	effectiveOutputFormat := cfg.OutputFormat
	if effectiveOutputFormat == nil {
		effectiveOutputFormat = defaults.OutputFormat
	}
	if cfg.TUI != nil && effectiveOutputFormat != nil && *cfg.TUI &&
		isExecJSONOutputFormat(*effectiveOutputFormat) {
		return fmt.Errorf(
			"workspace config exec.tui cannot be true when exec.output_format is %q or %q",
			model.OutputFormatJSONValue,
			model.OutputFormatRawJSONValue,
		)
	}
	return nil
}

func validateWorkflowTUI(section string, defaults DefaultsConfig, outputFormat *string, tui *bool) error {
	effectiveOutputFormat := outputFormat
	outputField := fmt.Sprintf("%s.output_format", section)
	if effectiveOutputFormat == nil {
		effectiveOutputFormat = defaults.OutputFormat
		outputField = "defaults.output_format"
	}
	if tui != nil && effectiveOutputFormat != nil && *tui && isExecJSONOutputFormat(*effectiveOutputFormat) {
		return fmt.Errorf(
			"workspace config %s.tui cannot be true when workspace config %s is %q or %q",
			section,
			outputField,
			model.OutputFormatJSONValue,
			model.OutputFormatRawJSONValue,
		)
	}
	return nil
}

func validateRuntimeOverrides(section string, cfg RuntimeOverrides) error {
	validators := []func(string, RuntimeOverrides) error{
		validateRuntimeIDE,
		validateRuntimeOutputFormat,
		validateRuntimeReasoningEffort,
		validateRuntimeAccessMode,
		validateRuntimeTimeout,
		validateRuntimeTailLines,
		validateRuntimeMaxRetries,
		validateRuntimeRetryBackoffMultiplier,
	}
	for _, validate := range validators {
		if err := validate(section, cfg); err != nil {
			return err
		}
	}
	return nil
}

func validateRuntimeIDE(section string, cfg RuntimeOverrides) error {
	if cfg.IDE == nil {
		return nil
	}
	if strings.TrimSpace(*cfg.IDE) == "" {
		return fmt.Errorf("workspace config %s.ide cannot be empty", section)
	}
	if _, err := agent.DriverCatalogEntryForIDE(strings.TrimSpace(*cfg.IDE)); err != nil {
		return fmt.Errorf("workspace config %s.ide: %w", section, err)
	}
	return nil
}

func validateRuntimeOutputFormat(section string, cfg RuntimeOverrides) error {
	return validateOutputFormatValue(runtimeFieldName(section, "output_format"), cfg.OutputFormat)
}

func validateRuntimeReasoningEffort(section string, cfg RuntimeOverrides) error {
	if cfg.ReasoningEffort == nil {
		return nil
	}
	switch strings.TrimSpace(*cfg.ReasoningEffort) {
	case "low", "medium", "high", "xhigh":
		return nil
	default:
		return fmt.Errorf(
			"%s must be one of low, medium, high, xhigh (got %q)",
			runtimeFieldName(section, "reasoning_effort"),
			strings.TrimSpace(*cfg.ReasoningEffort),
		)
	}
}

func validateRuntimeAccessMode(section string, cfg RuntimeOverrides) error {
	if cfg.AccessMode == nil {
		return nil
	}
	switch strings.TrimSpace(*cfg.AccessMode) {
	case model.AccessModeDefault, model.AccessModeFull:
		return nil
	default:
		return fmt.Errorf(
			"%s must be %q or %q (got %q)",
			runtimeFieldName(section, "access_mode"),
			model.AccessModeDefault,
			model.AccessModeFull,
			strings.TrimSpace(*cfg.AccessMode),
		)
	}
}

func validateRuntimeTimeout(section string, cfg RuntimeOverrides) error {
	if cfg.Timeout == nil {
		return nil
	}

	timeout := strings.TrimSpace(*cfg.Timeout)
	if timeout == "" {
		return fmt.Errorf("%s cannot be empty", runtimeFieldName(section, "timeout"))
	}
	duration, err := time.ParseDuration(timeout)
	if err != nil {
		return fmt.Errorf("%s: %w", runtimeFieldName(section, "timeout"), err)
	}
	if duration <= 0 {
		return fmt.Errorf("%s must be greater than zero (got %s)", runtimeFieldName(section, "timeout"), timeout)
	}
	return nil
}

func validateRuntimeAddDirs(section string, cfg RuntimeOverrides, defaults *DefaultsConfig) error {
	addDirs, fieldName := effectiveAddDirs(section, cfg, defaults)
	if len(addDirs) == 0 {
		return nil
	}

	return agent.ValidateAddDirSupport(fieldName, effectiveIDE(cfg, defaults), addDirs)
}

func effectiveIDE(cfg RuntimeOverrides, defaults *DefaultsConfig) string {
	if cfg.IDE != nil && strings.TrimSpace(*cfg.IDE) != "" {
		return strings.TrimSpace(*cfg.IDE)
	}
	if defaults != nil && defaults.IDE != nil && strings.TrimSpace(*defaults.IDE) != "" {
		return strings.TrimSpace(*defaults.IDE)
	}
	return model.IDECodex
}

func effectiveAddDirs(section string, cfg RuntimeOverrides, defaults *DefaultsConfig) ([]string, string) {
	if cfg.AddDirs != nil {
		return *cfg.AddDirs, runtimeFieldName(section, "add_dirs")
	}
	if defaults != nil && defaults.AddDirs != nil {
		return *defaults.AddDirs, runtimeFieldName("defaults", "add_dirs")
	}
	return nil, ""
}

func validateRuntimeTailLines(section string, cfg RuntimeOverrides) error {
	if cfg.TailLines != nil && *cfg.TailLines < 0 {
		return fmt.Errorf("%s must be 0 or greater (got %d)", runtimeFieldName(section, "tail_lines"), *cfg.TailLines)
	}
	return nil
}

func validateRuntimeMaxRetries(section string, cfg RuntimeOverrides) error {
	if cfg.MaxRetries != nil && *cfg.MaxRetries < 0 {
		return fmt.Errorf("%s cannot be negative (got %d)", runtimeFieldName(section, "max_retries"), *cfg.MaxRetries)
	}
	return nil
}

func validateRuntimeRetryBackoffMultiplier(section string, cfg RuntimeOverrides) error {
	if cfg.RetryBackoffMultiplier != nil && *cfg.RetryBackoffMultiplier <= 0 {
		return fmt.Errorf(
			"%s must be positive (got %.2f)",
			runtimeFieldName(section, "retry_backoff_multiplier"),
			*cfg.RetryBackoffMultiplier,
		)
	}
	return nil
}

func validateOutputFormatValue(field string, value *string) error {
	if value == nil {
		return nil
	}
	switch strings.TrimSpace(*value) {
	case "":
		return fmt.Errorf("%s cannot be empty", field)
	case model.OutputFormatTextValue, model.OutputFormatJSONValue, model.OutputFormatRawJSONValue:
		return nil
	default:
		return fmt.Errorf(
			"%s must be %q, %q, or %q (got %q)",
			field,
			model.OutputFormatTextValue,
			model.OutputFormatJSONValue,
			model.OutputFormatRawJSONValue,
			strings.TrimSpace(*value),
		)
	}
}

func isExecJSONOutputFormat(value string) bool {
	switch strings.TrimSpace(value) {
	case model.OutputFormatJSONValue, model.OutputFormatRawJSONValue:
		return true
	default:
		return false
	}
}

func runtimeFieldName(section, field string) string {
	return fmt.Sprintf("workspace config %s.%s", section, field)
}
