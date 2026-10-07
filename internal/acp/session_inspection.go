package acp

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// SessionInspectionRequest describes a short-lived ACP session used only to read advertised options.
type SessionInspectionRequest struct {
	AgentName string
	Command   string
	Cwd       string
	Env       []string
}

// SessionModelInspection retains options for each independently selected transport model.
type SessionModelInspection struct {
	Options []SessionConfigOption
	// Models holds options only for models whose selection the agent acknowledged.
	Models map[string][]SessionConfigOption
	// ModelErrors records each model whose options could not be observed, whether the
	// selection failed or the budget ran out first; Options stays authoritative.
	ModelErrors map[string]error
}

// InspectSessionModels reads model-specific options without submitting a prompt.
// The advertised model list is the discovery result; per-model option probes only
// enrich it, so a rejected model or an exhausted budget never discards the list.
func InspectSessionModels(ctx context.Context, req SessionInspectionRequest) (SessionModelInspection, error) {
	var result SessionModelInspection
	err := inspectSession(ctx, req, func(driver *Driver, proc *AgentProcess) error {
		result.Options = CloneSessionConfigOptions(proc.CapsSnapshot().ConfigOptions)
		model, ok := ModelConfigOption(result.Options)
		if !ok {
			return errors.New("acp: inspected session has no model option")
		}
		result.Models = make(map[string][]SessionConfigOption, len(model.Values))
		for _, value := range model.Values {
			if _, exists := result.Models[value.Value]; exists {
				continue
			}
			if _, failed := result.ModelErrors[value.Value]; failed {
				continue
			}
			if model.ReadOnly && value.Value != model.CurrentValueID {
				continue
			}
			if err := ctx.Err(); err != nil {
				if errors.Is(err, context.Canceled) {
					return err
				}
				result.recordModelError(value.Value, err)
				continue
			}
			if !model.ReadOnly {
				if _, err := driver.applySessionModel(ctx, proc, value.Value); err != nil {
					result.recordModelError(value.Value, err)
					continue
				}
			}
			result.Models[value.Value] = CloneSessionConfigOptions(proc.CapsSnapshot().ConfigOptions)
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			return ctx.Err()
		}
		return nil
	})
	if err != nil {
		return SessionModelInspection{}, err
	}
	return result, nil
}

func (r *SessionModelInspection) recordModelError(modelID string, err error) {
	if r.ModelErrors == nil {
		r.ModelErrors = make(map[string]error)
	}
	r.ModelErrors[modelID] = fmt.Errorf("acp: inspect model %q: %w", modelID, err)
}

// InspectSessionConfigOptions creates a short-lived ACP session and returns its advertised config options.
func InspectSessionConfigOptions(
	ctx context.Context,
	req SessionInspectionRequest,
) ([]SessionConfigOption, error) {
	var options []SessionConfigOption
	err := inspectSession(ctx, req, func(_ *Driver, proc *AgentProcess) error {
		options = CloneSessionConfigOptions(proc.CapsSnapshot().ConfigOptions)
		return nil
	})
	return options, err
}

// inspectSession owns the disposable process and joins its bounded cleanup on every exit.
func inspectSession(
	ctx context.Context,
	req SessionInspectionRequest,
	inspect func(*Driver, *AgentProcess) error,
) (err error) {
	if ctx == nil {
		return errors.New("acp: session inspection context is required")
	}
	driver := New()
	proc, err := driver.Start(ctx, StartOpts{
		AgentName:  strings.TrimSpace(req.AgentName),
		Command:    strings.TrimSpace(req.Command),
		Cwd:        strings.TrimSpace(req.Cwd),
		Env:        append([]string(nil), req.Env...),
		Inspection: true,
	})
	if err != nil {
		return fmt.Errorf("acp: inspect session config options: %w", err)
	}
	defer func() {
		// Stop uses defaultStopTimeout for cooperative shutdown before signal escalation.
		// Give that full lifecycle a separate, bounded budget instead of reporting the
		// normal escalation path as a failed inspection.
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*defaultStopTimeout)
		defer cancel()
		if stopErr := driver.stop(stopCtx, proc, true); stopErr != nil {
			err = errors.Join(err, fmt.Errorf("acp: stop inspected session: %w", stopErr))
		}
	}()

	return inspect(driver, proc)
}
