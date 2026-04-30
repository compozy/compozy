package workspace

import (
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/providerdefaults"
	"github.com/compozy/compozy/internal/core/tasks"
)

const (
	workspaceConfigScope  = "workspace config"
	globalConfigScope     = "global config"
	effectiveConfigScope  = "effective config"
	reasoningEffortLow    = "low"
	reasoningEffortMedium = "medium"
	reasoningEffortHigh   = "high"
	reasoningEffortXHigh  = "xhigh"
	reasoningEffortValues = "low, medium, high, xhigh"
)

func (cfg ProjectConfig) Validate() error {
	return cfg.validate(workspaceConfigScope)
}

func (cfg ProjectConfig) validate(scope string) error {
	if err := validateDefaults(scope, cfg.Defaults); err != nil {
		return err
	}
	if err := validateTasks(scope, cfg.Defaults, cfg.Tasks); err != nil {
		return err
	}
	if err := validateFixReviews(scope, cfg.Defaults, cfg.FixReviews); err != nil {
		return err
	}
	if err := validateFetchReviews(scope, cfg.FetchReviews); err != nil {
		return err
	}
	if err := validateWatchReviews(scope, cfg.Defaults, cfg.WatchReviews); err != nil {
		return err
	}
	if err := validateExec(scope, cfg.Defaults, cfg.Exec); err != nil {
		return err
	}
	if err := validateRuns(scope, cfg.Runs); err != nil {
		return err
	}
	if err := validateSound(scope, cfg.Sound); err != nil {
		return err
	}
	return nil
}

func validateSound(scope string, cfg SoundConfig) error {
	if err := validateSoundField(configFieldName(scope, "sound.on_completed"), cfg.OnCompleted); err != nil {
		return err
	}
	return validateSoundField(configFieldName(scope, "sound.on_failed"), cfg.OnFailed)
}

func validateSoundField(field string, value *string) error {
	if value == nil {
		return nil
	}
	if strings.TrimSpace(*value) == "" {
		return fmt.Errorf("%s cannot be empty", field)
	}
	return nil
}

func validateDefaults(scope string, cfg DefaultsConfig) error {
	overrides := RuntimeOverrides(cfg)
	if err := validateRuntimeOverrides(scope, "defaults", overrides); err != nil {
		return err
	}
	return validateRuntimeAddDirs(scope, "defaults", overrides, nil)
}

func validateTaskRun(scope string, defaults DefaultsConfig, cfg TaskRunConfig) error {
	if err := validateOutputFormatValue(
		configFieldName(scope, "tasks.run.output_format"),
		cfg.OutputFormat,
	); err != nil {
		return err
	}
	if err := validateWorkflowTUI(scope, "tasks.run", defaults, cfg.OutputFormat, cfg.TUI); err != nil {
		return err
	}
	return validateTaskRunRuntimeRules(scope, cfg.TaskRuntimeRules)
}

func validateTasks(scope string, defaults DefaultsConfig, cfg TasksConfig) error {
	if err := validateTaskRun(scope, defaults, cfg.Run); err != nil {
		return err
	}
	if cfg.Types == nil {
		return nil
	}
	if len(*cfg.Types) == 0 {
		return fmt.Errorf(
			"%s cannot be empty; omit tasks.types to use built-in defaults",
			configFieldName(scope, "tasks.types"),
		)
	}
	if _, err := tasks.NewRegistry(*cfg.Types); err != nil {
		return fmt.Errorf("%s: %w", configFieldName(scope, "tasks.types"), err)
	}
	return nil
}

func validateFixReviews(scope string, defaults DefaultsConfig, cfg FixReviewsConfig) error {
	if cfg.Concurrent != nil && *cfg.Concurrent <= 0 {
		return fmt.Errorf(
			"%s must be greater than zero (got %d)",
			configFieldName(scope, "fix_reviews.concurrent"),
			*cfg.Concurrent,
		)
	}
	if cfg.BatchSize != nil && *cfg.BatchSize <= 0 {
		return fmt.Errorf(
			"%s must be greater than zero (got %d)",
			configFieldName(scope, "fix_reviews.batch_size"),
			*cfg.BatchSize,
		)
	}
	if err := validateOutputFormatValue(
		configFieldName(scope, "fix_reviews.output_format"),
		cfg.OutputFormat,
	); err != nil {
		return err
	}
	return validateWorkflowTUI(scope, "fix_reviews", defaults, cfg.OutputFormat, cfg.TUI)
}

func validateFetchReviews(scope string, cfg FetchReviewsConfig) error {
	if cfg.Provider == nil {
		return nil
	}
	name := strings.TrimSpace(*cfg.Provider)
	if name == "" {
		return fmt.Errorf("%s cannot be empty", configFieldName(scope, "fetch_reviews.provider"))
	}
	if _, err := provider.ResolveRegistry(providerdefaults.DefaultRegistry()).Get(name); err != nil {
		return fmt.Errorf("%s: %w", configFieldName(scope, "fetch_reviews.provider"), err)
	}
	return nil
}

func validateWatchReviews(scope string, defaults DefaultsConfig, cfg WatchReviewsConfig) error {
	if cfg.MaxRounds != nil && *cfg.MaxRounds < 0 {
		return fmt.Errorf(
			"%s must be zero or greater (got %d)",
			configFieldName(scope, "watch_reviews.max_rounds"),
			*cfg.MaxRounds,
		)
	}
	if isEnabled(cfg.UntilClean, true) && cfg.MaxRounds != nil && *cfg.MaxRounds == 0 {
		return fmt.Errorf(
			"%s must be greater than zero when watch_reviews.until_clean is true",
			configFieldName(scope, "watch_reviews.max_rounds"),
		)
	}
	if err := validatePositiveDurationField(scope, "watch_reviews.poll_interval", cfg.PollInterval); err != nil {
		return err
	}
	if err := validatePositiveDurationField(scope, "watch_reviews.review_timeout", cfg.ReviewTimeout); err != nil {
		return err
	}
	if err := validatePositiveDurationField(scope, "watch_reviews.quiet_period", cfg.QuietPeriod); err != nil {
		return err
	}
	if err := validateOptionalNonEmptyString(scope, "watch_reviews.push_remote", cfg.PushRemote); err != nil {
		return err
	}
	if err := validateOptionalNonEmptyString(scope, "watch_reviews.push_branch", cfg.PushBranch); err != nil {
		return err
	}
	if (cfg.PushRemote == nil) != (cfg.PushBranch == nil) {
		return fmt.Errorf(
			"%s and %s must be set together or both omitted",
			configFieldName(scope, "watch_reviews.push_remote"),
			configFieldName(scope, "watch_reviews.push_branch"),
		)
	}
	if isEnabled(cfg.AutoPush, false) && defaults.AutoCommit != nil && !*defaults.AutoCommit {
		return fmt.Errorf(
			"%s requires %s to be true",
			configFieldName(scope, "watch_reviews.auto_push"),
			configFieldName(scope, "defaults.auto_commit"),
		)
	}
	return nil
}

func isEnabled(value *bool, defaultValue bool) bool {
	if value == nil {
		return defaultValue
	}
	return *value
}

func validatePositiveDurationField(scope string, field string, value *string) error {
	if value == nil {
		return nil
	}
	durationText := strings.TrimSpace(*value)
	if durationText == "" {
		return fmt.Errorf("%s cannot be empty", configFieldName(scope, field))
	}
	duration, err := time.ParseDuration(durationText)
	if err != nil {
		return fmt.Errorf("%s: %w", configFieldName(scope, field), err)
	}
	if duration <= 0 {
		return fmt.Errorf("%s must be greater than zero (got %s)", configFieldName(scope, field), durationText)
	}
	return nil
}

func validateOptionalNonEmptyString(scope string, field string, value *string) error {
	if value == nil {
		return nil
	}
	if strings.TrimSpace(*value) == "" {
		return fmt.Errorf("%s cannot be empty", configFieldName(scope, field))
	}
	return nil
}

func validateExec(scope string, defaults DefaultsConfig, cfg ExecConfig) error {
	if err := validateRuntimeOverrides(scope, "exec", cfg.RuntimeOverrides); err != nil {
		return err
	}
	if err := validateRuntimeAddDirs(scope, "exec", cfg.RuntimeOverrides, &defaults); err != nil {
		return err
	}

	effectiveOutputFormat := cfg.OutputFormat
	if effectiveOutputFormat == nil {
		effectiveOutputFormat = defaults.OutputFormat
	}
	if cfg.TUI != nil && effectiveOutputFormat != nil && *cfg.TUI &&
		isExecJSONOutputFormat(*effectiveOutputFormat) {
		return fmt.Errorf(
			"%s cannot be true when %s is %q or %q",
			configFieldName(scope, "exec.tui"),
			configFieldName(scope, "exec.output_format"),
			model.OutputFormatJSONValue,
			model.OutputFormatRawJSONValue,
		)
	}
	return nil
}

func validateRuns(scope string, cfg RunsConfig) error {
	if err := validateAttachModeValue(
		configFieldName(scope, "runs.default_attach_mode"),
		cfg.DefaultAttachMode,
	); err != nil {
		return err
	}
	if cfg.KeepTerminalDays != nil && *cfg.KeepTerminalDays < 0 {
		return fmt.Errorf(
			"%s must be zero or greater (got %d)",
			configFieldName(scope, "runs.keep_terminal_days"),
			*cfg.KeepTerminalDays,
		)
	}
	if cfg.KeepMax != nil && *cfg.KeepMax < 0 {
		return fmt.Errorf(
			"%s must be zero or greater (got %d)",
			configFieldName(scope, "runs.keep_max"),
			*cfg.KeepMax,
		)
	}
	if cfg.ShutdownDrainTimeout != nil {
		timeout := strings.TrimSpace(*cfg.ShutdownDrainTimeout)
		if timeout == "" {
			return fmt.Errorf("%s cannot be empty", configFieldName(scope, "runs.shutdown_drain_timeout"))
		}
		duration, err := time.ParseDuration(timeout)
		if err != nil {
			return fmt.Errorf("%s: %w", configFieldName(scope, "runs.shutdown_drain_timeout"), err)
		}
		if duration <= 0 {
			return fmt.Errorf(
				"%s must be greater than zero (got %s)",
				configFieldName(scope, "runs.shutdown_drain_timeout"),
				timeout,
			)
		}
	}
	return nil
}

func validateAttachModeValue(field string, value *string) error {
	if value == nil {
		return nil
	}
	switch strings.TrimSpace(*value) {
	case "":
		return fmt.Errorf("%s cannot be empty", field)
	case "auto", "ui", "stream", "detach":
		return nil
	default:
		return fmt.Errorf(
			"%s must be %q, %q, %q, or %q (got %q)",
			field,
			"auto",
			"ui",
			"stream",
			"detach",
			strings.TrimSpace(*value),
		)
	}
}

func validateWorkflowTUI(scope, section string, defaults DefaultsConfig, outputFormat *string, tui *bool) error {
	effectiveOutputFormat := outputFormat
	outputField := configFieldName(scope, fmt.Sprintf("%s.output_format", section))
	if effectiveOutputFormat == nil {
		effectiveOutputFormat = defaults.OutputFormat
		outputField = configFieldName(scope, "defaults.output_format")
	}
	if tui != nil && effectiveOutputFormat != nil && *tui && isExecJSONOutputFormat(*effectiveOutputFormat) {
		return fmt.Errorf(
			"%s cannot be true when %s is %q or %q",
			configFieldName(scope, fmt.Sprintf("%s.tui", section)),
			outputField,
			model.OutputFormatJSONValue,
			model.OutputFormatRawJSONValue,
		)
	}
	return nil
}

func validateRuntimeOverrides(scope, section string, cfg RuntimeOverrides) error {
	validators := []func(string, string, RuntimeOverrides) error{
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
		if err := validate(scope, section, cfg); err != nil {
			return err
		}
	}
	return nil
}

func validateRuntimeIDE(scope, section string, cfg RuntimeOverrides) error {
	if cfg.IDE == nil {
		return nil
	}
	if strings.TrimSpace(*cfg.IDE) == "" {
		return fmt.Errorf("%s cannot be empty", runtimeFieldName(scope, section, "ide"))
	}
	if _, err := agent.DriverCatalogEntryForIDE(strings.TrimSpace(*cfg.IDE)); err != nil {
		return fmt.Errorf("%s: %w", runtimeFieldName(scope, section, "ide"), err)
	}
	return nil
}

func validateRuntimeOutputFormat(scope, section string, cfg RuntimeOverrides) error {
	return validateOutputFormatValue(runtimeFieldName(scope, section, "output_format"), cfg.OutputFormat)
}

func validateRuntimeReasoningEffort(scope, section string, cfg RuntimeOverrides) error {
	return validateReasoningEffortValue(runtimeFieldName(scope, section, "reasoning_effort"), cfg.ReasoningEffort)
}

func validateRuntimeAccessMode(scope, section string, cfg RuntimeOverrides) error {
	if cfg.AccessMode == nil {
		return nil
	}
	switch strings.TrimSpace(*cfg.AccessMode) {
	case model.AccessModeDefault, model.AccessModeFull:
		return nil
	default:
		return fmt.Errorf(
			"%s must be %q or %q (got %q)",
			runtimeFieldName(scope, section, "access_mode"),
			model.AccessModeDefault,
			model.AccessModeFull,
			strings.TrimSpace(*cfg.AccessMode),
		)
	}
}

func validateRuntimeTimeout(scope, section string, cfg RuntimeOverrides) error {
	if cfg.Timeout == nil {
		return nil
	}

	timeout := strings.TrimSpace(*cfg.Timeout)
	if timeout == "" {
		return fmt.Errorf("%s cannot be empty", runtimeFieldName(scope, section, "timeout"))
	}
	duration, err := time.ParseDuration(timeout)
	if err != nil {
		return fmt.Errorf("%s: %w", runtimeFieldName(scope, section, "timeout"), err)
	}
	if duration <= 0 {
		return fmt.Errorf("%s must be greater than zero (got %s)", runtimeFieldName(scope, section, "timeout"), timeout)
	}
	return nil
}

func validateRuntimeAddDirs(scope, section string, cfg RuntimeOverrides, defaults *DefaultsConfig) error {
	addDirs, fieldName := effectiveAddDirs(scope, section, cfg, defaults)
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

func effectiveAddDirs(scope, section string, cfg RuntimeOverrides, defaults *DefaultsConfig) ([]string, string) {
	if cfg.AddDirs != nil {
		return *cfg.AddDirs, runtimeFieldName(scope, section, "add_dirs")
	}
	if defaults != nil && defaults.AddDirs != nil {
		return *defaults.AddDirs, runtimeFieldName(scope, "defaults", "add_dirs")
	}
	return nil, ""
}

func validateRuntimeTailLines(scope, section string, cfg RuntimeOverrides) error {
	if cfg.TailLines != nil && *cfg.TailLines < 0 {
		return fmt.Errorf(
			"%s must be 0 or greater (got %d)",
			runtimeFieldName(scope, section, "tail_lines"),
			*cfg.TailLines,
		)
	}
	return nil
}

func validateRuntimeMaxRetries(scope, section string, cfg RuntimeOverrides) error {
	if cfg.MaxRetries != nil && *cfg.MaxRetries < 0 {
		return fmt.Errorf(
			"%s cannot be negative (got %d)",
			runtimeFieldName(scope, section, "max_retries"),
			*cfg.MaxRetries,
		)
	}
	return nil
}

func validateRuntimeRetryBackoffMultiplier(scope, section string, cfg RuntimeOverrides) error {
	if cfg.RetryBackoffMultiplier != nil && *cfg.RetryBackoffMultiplier <= 0 {
		return fmt.Errorf(
			"%s must be positive (got %.2f)",
			runtimeFieldName(scope, section, "retry_backoff_multiplier"),
			*cfg.RetryBackoffMultiplier,
		)
	}
	return nil
}

func validateTaskRunRuntimeRules(scope string, rules *[]model.TaskRuntimeRule) error {
	if rules == nil {
		return nil
	}
	for idx, rule := range *rules {
		fieldPrefix := fmt.Sprintf("%s[%d]", configFieldName(scope, "tasks.run.task_runtime_rules"), idx)
		if rule.ID != nil {
			return fmt.Errorf("%s.id is not supported; use CLI --task-runtime for per-task ids", fieldPrefix)
		}
		if rule.Type == nil || strings.TrimSpace(*rule.Type) == "" {
			return fmt.Errorf("%s.type is required", fieldPrefix)
		}
		if !rule.HasOverride() {
			return fmt.Errorf("%s must define at least one of ide, model, or reasoning_effort", fieldPrefix)
		}
		if err := validateTaskRuntimeRuleRuntime(fieldPrefix, rule); err != nil {
			return err
		}
	}
	return nil
}

func validateTaskRuntimeRuleRuntime(fieldPrefix string, rule model.TaskRuntimeRule) error {
	if rule.IDE != nil {
		value := strings.TrimSpace(*rule.IDE)
		if value == "" {
			return fmt.Errorf("%s.ide cannot be empty", fieldPrefix)
		}
		if _, err := agent.DriverCatalogEntryForIDE(value); err != nil {
			return fmt.Errorf("%s.ide: %w", fieldPrefix, err)
		}
	}
	if rule.Model != nil && strings.TrimSpace(*rule.Model) == "" {
		return fmt.Errorf("%s.model cannot be empty", fieldPrefix)
	}
	if err := validateReasoningEffortValue(fieldPrefix+".reasoning_effort", rule.ReasoningEffort); err != nil {
		return err
	}
	return nil
}

func validateReasoningEffortValue(field string, value *string) error {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	switch trimmed {
	case reasoningEffortLow, reasoningEffortMedium, reasoningEffortHigh, reasoningEffortXHigh:
		return nil
	default:
		return fmt.Errorf("%s must be one of %s (got %q)", field, reasoningEffortValues, trimmed)
	}
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

func configFieldName(scope, field string) string {
	return fmt.Sprintf("%s %s", scope, field)
}

func runtimeFieldName(scope, section, field string) string {
	return configFieldName(scope, fmt.Sprintf("%s.%s", section, field))
}
