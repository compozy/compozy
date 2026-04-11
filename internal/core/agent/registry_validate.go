package agent

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

var ErrRuntimeConfigNil = errors.New("runtime config is nil")

// ValidateRuntimeConfig verifies that the runtime config references a supported agent runtime.
func ValidateRuntimeConfig(cfg *model.RuntimeConfig) error {
	if cfg == nil {
		return ErrRuntimeConfigNil
	}
	if err := validateRuntimeMode(cfg.Mode); err != nil {
		return err
	}
	spec, err := lookupAgentSpec(cfg.IDE)
	if err != nil {
		return fmt.Errorf("invalid --ide value %q: must be %s", cfg.IDE, quotedSupportedIDEs())
	}
	if err := validateRuntimeAccessMode(cfg.AccessMode); err != nil {
		return err
	}
	if err := validateAddDirSupport("--add-dir", spec, cfg.AddDirs); err != nil {
		return err
	}
	if cfg.Mode == model.ExecutionModePRDTasks && cfg.BatchSize != 1 {
		return fmt.Errorf("batch size must be 1 for prd-tasks mode (got %d)", cfg.BatchSize)
	}
	if err := validateRuntimeOutputFormat(cfg); err != nil {
		return err
	}
	if err := validateRuntimeExecMode(cfg); err != nil {
		return err
	}
	if err := validateRuntimePromptSource(cfg); err != nil {
		return err
	}
	if cfg.MaxRetries < 0 {
		return fmt.Errorf("max-retries cannot be negative (got %d)", cfg.MaxRetries)
	}
	if cfg.RetryBackoffMultiplier <= 0 {
		return fmt.Errorf("retry-backoff-multiplier must be positive (got %.2f)", cfg.RetryBackoffMultiplier)
	}
	return nil
}

// ValidateAddDirSupport verifies that the selected runtime supports extra writable roots.
func ValidateAddDirSupport(fieldName string, ide string, addDirs []string) error {
	spec, err := lookupAgentSpec(strings.TrimSpace(ide))
	if err != nil {
		return err
	}
	return validateAddDirSupport(fieldName, spec, addDirs)
}

func validateRuntimeMode(mode model.ExecutionMode) error {
	switch mode {
	case model.ExecutionModePRReview, model.ExecutionModePRDTasks, model.ExecutionModeExec:
		return nil
	default:
		return fmt.Errorf(
			"invalid --mode value %q: must be %q, %q, or %q",
			mode,
			model.ModeCodeReview,
			model.ModePRDTasks,
			model.ModeExec,
		)
	}
}

func validateRuntimeAccessMode(accessMode string) error {
	switch accessMode {
	case "", model.AccessModeDefault, model.AccessModeFull:
		return nil
	default:
		return fmt.Errorf(
			"invalid --access-mode value %q: must be %q or %q",
			accessMode,
			model.AccessModeDefault,
			model.AccessModeFull,
		)
	}
}

func validateRuntimeOutputFormat(cfg *model.RuntimeConfig) error {
	format := cfg.OutputFormat
	if format == "" {
		format = model.OutputFormatText
	}

	switch format {
	case model.OutputFormatText, model.OutputFormatJSON, model.OutputFormatRawJSON:
	default:
		return fmt.Errorf(
			"invalid output format %q: must be %q, %q, or %q",
			format,
			model.OutputFormatText,
			model.OutputFormatJSON,
			model.OutputFormatRawJSON,
		)
	}
	return nil
}

func validateRuntimePromptSource(cfg *model.RuntimeConfig) error {
	sources := runtimePromptSourceCount(cfg)
	if cfg.Mode != model.ExecutionModeExec {
		if sources > 0 {
			return errors.New("prompt source fields are only supported for exec mode")
		}
		return nil
	}
	switch {
	case sources == 0:
		return errors.New("exec mode requires exactly one prompt source: prompt text, prompt file, or stdin")
	case sources > 1:
		return errors.New("exec mode accepts only one prompt source: prompt text, prompt file, or stdin")
	default:
		return nil
	}
}

func validateRuntimeExecMode(cfg *model.RuntimeConfig) error {
	format := cfg.OutputFormat
	if format == "" {
		format = model.OutputFormatText
	}

	if cfg.Mode != model.ExecutionModeExec {
		switch {
		case cfg.Persist:
			return errors.New("persist is only supported for exec mode")
		case strings.TrimSpace(cfg.RunID) != "":
			return errors.New("run-id is only supported for exec mode")
		}
	}
	if (format == model.OutputFormatJSON || format == model.OutputFormatRawJSON) && cfg.TUI {
		return errors.New("tui mode is not supported with json or raw-json output")
	}
	return nil
}

func runtimePromptSourceCount(cfg *model.RuntimeConfig) int {
	sources := 0
	if strings.TrimSpace(cfg.PromptText) != "" {
		sources++
	}
	if strings.TrimSpace(cfg.PromptFile) != "" {
		sources++
	}
	if cfg.ReadPromptStdin {
		sources++
	}
	if sources == 0 && strings.TrimSpace(cfg.ResolvedPromptText) != "" {
		sources = 1
	}
	return sources
}

func validateAddDirSupport(fieldName string, spec Spec, addDirs []string) error {
	if !hasConfiguredAddDirs(addDirs) || spec.SupportsAddDirs {
		return nil
	}

	return fmt.Errorf(
		"%s is only supported for %s; runtime %q does not support extra writable roots",
		fieldName,
		strings.Join(supportedAddDirIDEs(), ", "),
		spec.ID,
	)
}

func hasConfiguredAddDirs(addDirs []string) bool {
	for _, dir := range addDirs {
		if strings.TrimSpace(dir) != "" {
			return true
		}
	}
	return false
}

func supportedAddDirIDEs() []string {
	snapshot := currentCatalogSnapshot()

	ides := make([]string, 0, len(snapshot.order))
	for _, ide := range snapshot.order {
		spec, ok := snapshot.specs[ide]
		if !ok || !spec.SupportsAddDirs {
			continue
		}
		ides = append(ides, spec.ID)
	}
	return ides
}

func quotedSupportedIDEs() string {
	snapshot := currentCatalogSnapshot()
	items := make([]string, 0, len(snapshot.order))
	for _, ide := range snapshot.order {
		items = append(items, fmt.Sprintf("%q", ide))
	}
	return strings.Join(items, ", ")
}
