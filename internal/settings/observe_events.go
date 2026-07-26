package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	eventspkg "github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/store"
)

type settingsChangedContent struct {
	Section     string `json:"section"`
	Source      string `json:"source"`
	Operation   string `json:"operation"`
	Scope       string `json:"scope,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	AgentName   string `json:"agent_name,omitempty"`
	WriteTarget string `json:"write_target,omitempty"`
	Behavior    string `json:"behavior,omitempty"`
}

type mutationSourceContextKey struct{}

// WithMutationSource annotates a settings-service context with the public
// transport or runtime source that initiated a persisted mutation.
func WithMutationSource(ctx context.Context, source string) context.Context {
	if ctx == nil {
		return nil
	}
	return context.WithValue(ctx, mutationSourceContextKey{}, strings.TrimSpace(source))
}

func mutationSourceFromContext(ctx context.Context) string {
	if ctx == nil {
		return applyRecordActorRuntime
	}
	source, ok := ctx.Value(mutationSourceContextKey{}).(string)
	if !ok || strings.TrimSpace(source) == "" {
		return applyRecordActorRuntime
	}
	return strings.TrimSpace(source)
}

func (s *service) emitSettingsChanged(
	ctx context.Context,
	result MutationResult,
	operation string,
) error {
	if s == nil || s.eventSummaries == nil {
		return nil
	}

	content, err := json.Marshal(settingsChangedContent{
		Section:     strings.TrimSpace(string(result.Section)),
		Source:      mutationSourceFromContext(ctx),
		Operation:   strings.TrimSpace(operation),
		Scope:       strings.TrimSpace(string(result.Scope)),
		WorkspaceID: strings.TrimSpace(result.WorkspaceID),
		AgentName:   strings.TrimSpace(result.AgentName),
		WriteTarget: strings.TrimSpace(string(result.WriteTarget)),
		Behavior:    strings.TrimSpace(string(result.Behavior)),
	})
	if err != nil {
		return fmt.Errorf("settings: marshal settings.changed content: %w", err)
	}

	summary := strings.TrimSpace(string(result.Section)) + " settings changed"
	if strings.TrimSpace(operation) != "" {
		summary += " (" + strings.TrimSpace(operation) + ")"
	}

	return s.eventSummaries.WriteEventSummary(ctx, store.EventSummary{
		Type:    eventspkg.SettingsChanged,
		Content: content,
		Summary: summary,
	})
}

func (s *service) finalizeCommittedCollectionMutation(
	ctx context.Context,
	result MutationResult,
	err error,
	operation string,
	skipNoChangeEvent bool,
) (MutationResult, error) {
	if err != nil {
		return MutationResult{}, err
	}
	if skipNoChangeEvent && mutationResultHasNoChanges(result) {
		return result, nil
	}
	if emitErr := s.emitSettingsChanged(ctx, result, operation); emitErr != nil {
		result.Warnings = append(
			result.Warnings,
			"committed settings change event was not persisted: "+emitErr.Error(),
		)
	}
	return result, nil
}
