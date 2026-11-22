package automation

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// LoopInputResolutionRequest asks the composition root to run one fire-time Loop input command.
type LoopInputResolutionRequest struct {
	ProfileID       string
	WorkspaceID     string
	LoopName        string
	Kind            LoopStartKind
	Command         string
	AutomationID    string
	AutomationRunID string
}

// LoopInputResolver is optionally implemented by LoopStarter so automation can resolve Loop
// inputs at fire time without importing process-execution helpers.
type LoopInputResolver interface {
	ResolveLoopInputs(ctx context.Context, req LoopInputResolutionRequest) (map[string]any, error)
}

const (
	// LoopInputSourceMetadataKey records which surface produced the resolved Loop inputs.
	LoopInputSourceMetadataKey = "loop_input_source"
	// LoopInputsMetadataKey records the resolved Loop inputs for one automation run.
	LoopInputsMetadataKey = "loop_inputs"
	// LoopInputKeysMetadataKey records the sorted resolved Loop input keys for one automation run.
	LoopInputKeysMetadataKey = "loop_input_keys"
	// LoopInputErrorMetadataKey records the bounded fire-time command failure for one run.
	LoopInputErrorMetadataKey = "loop_input_error"
	// LoopInputSourceCommand reports inputs resolved by the configured fire-time command.
	LoopInputSourceCommand = "command"
	// LoopInputSkipReasonNoInputs reports a fire whose command succeeded without printing inputs.
	LoopInputSkipReasonNoInputs = "no inputs"
)

// loopInputErrorMetadataLimit bounds the stored fire-time command failure text.
const loopInputErrorMetadataLimit = 2000

// resolveLoopInputsForFire merges the static target inputs with the fire-time command output.
// It reports skipped when the command succeeded but produced no inputs, so the firing is recorded
// as a skip instead of starting a Loop without the work unit it needs.
func (d *Dispatcher) resolveLoopInputsForFire(
	ctx context.Context,
	req DispatchRequest,
	target *LoopTarget,
	scheduledRun *Run,
) (map[string]any, bool, error) {
	inputs := cloneJSONMap(target.Inputs)
	command := strings.TrimSpace(target.InputCommand)
	if command == "" {
		return inputs, false, nil
	}
	resolver, ok := d.loopStarter.(LoopInputResolver)
	if !ok || resolver == nil {
		return nil, false, errors.New("automation: loop input command requires a fire-time input resolver")
	}
	resolved, err := resolver.ResolveLoopInputs(ctx, LoopInputResolutionRequest{
		ProfileID:       req.profileID(),
		WorkspaceID:     strings.TrimSpace(target.WorkspaceID),
		LoopName:        strings.TrimSpace(target.LoopName),
		Kind:            req.loopStartKind(),
		Command:         command,
		AutomationID:    req.definitionID(),
		AutomationRunID: strings.TrimSpace(scheduledRun.ID),
	})
	if err != nil {
		scheduledRun.Metadata = loopInputFailureMetadata(scheduledRun.Metadata, err)
		return nil, false, fmt.Errorf("automation: resolve loop inputs: %w", err)
	}
	if len(resolved) == 0 {
		return nil, true, nil
	}
	merged := inputs
	if merged == nil {
		merged = make(map[string]any, len(resolved))
	}
	for key, value := range resolved {
		// The fire-time command wins over a static value with the same key.
		merged[key] = value
	}
	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	scheduledRun.Metadata = loopInputMetadata(scheduledRun.Metadata, merged, keys)
	return merged, false, nil
}

func loopInputMetadata(existing map[string]any, inputs map[string]any, keys []string) map[string]any {
	metadata := cloneJSONMap(existing)
	if metadata == nil {
		metadata = make(map[string]any, 3)
	}
	metadata[LoopInputSourceMetadataKey] = LoopInputSourceCommand
	metadata[LoopInputsMetadataKey] = cloneJSONMap(inputs)
	metadata[LoopInputKeysMetadataKey] = keys
	return metadata
}

func loopInputFailureMetadata(existing map[string]any, runErr error) map[string]any {
	metadata := cloneJSONMap(existing)
	if metadata == nil {
		metadata = make(map[string]any, 1)
	}
	metadata[LoopInputSourceMetadataKey] = LoopInputSourceCommand
	if runErr != nil {
		metadata[LoopInputErrorMetadataKey] = truncateRunMetadata(runErr.Error(), loopInputErrorMetadataLimit)
	}
	return metadata
}

func loopInputSkipMetadata(existing map[string]any) map[string]any {
	metadata := cloneJSONMap(existing)
	if metadata == nil {
		metadata = make(map[string]any, 2)
	}
	metadata[SchedulerSkipReasonMetadataKey] = LoopInputSkipReasonNoInputs
	metadata[LoopInputSourceMetadataKey] = LoopInputSourceCommand
	return metadata
}

func truncateRunMetadata(value string, limit int) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= limit {
		return trimmed
	}
	return trimmed[:limit] + "..."
}
