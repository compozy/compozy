package skills

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/skillscan"
	"github.com/compozy/compozy/internal/store"
)

const (
	defaultSkillSourceActorKind = "daemon"
	defaultSkillSourceActorID   = "skill-source-registry"
)

// SourceEventCorrelation identifies the exact owner and actor for source lifecycle events.
type SourceEventCorrelation struct {
	Scope       string
	ProfileID   string
	WorkspaceID string
	ActorKind   string
	ActorID     string
	RootCounts  map[string]int
}

type sourceEventCorrelationContextKey struct{}

// WithSourceEventCorrelation binds source lifecycle work to its owning profile, workspace, and actor.
func WithSourceEventCorrelation(ctx context.Context, correlation SourceEventCorrelation) context.Context {
	if ctx == nil {
		return nil
	}
	correlation.RootCounts = cloneSourceRootCounts(correlation.RootCounts)
	return context.WithValue(ctx, sourceEventCorrelationContextKey{}, correlation)
}

// SourceEventCorrelationFromContext returns the normalized source owner and actor bound to work.
func SourceEventCorrelationFromContext(ctx context.Context) SourceEventCorrelation {
	return sourceEventCorrelation(ctx, nil)
}

type skillSourceEventBase struct {
	ProfileID        string `json:"profile_id"`
	WorkspaceID      string `json:"workspace_id,omitempty"`
	ConfigGeneration int64  `json:"config_generation"`
	ActorKind        string `json:"actor_kind"`
	ActorID          string `json:"actor_id"`
}

type skillSourcesAppliedContent struct {
	skillSourceEventBase
	Scope      string         `json:"scope"`
	Generation int64          `json:"generation"`
	RootCounts map[string]int `json:"root_counts"`
}

type skillSourcesSupersededContent struct {
	skillSourceEventBase
	DiscardedGeneration int64 `json:"discarded_generation"`
	WinningGeneration   int64 `json:"winning_generation"`
}

type skillSourcesApplyFailedContent struct {
	skillSourceEventBase
	Scope      string `json:"scope"`
	Generation int64  `json:"generation"`
	ErrorClass string `json:"error_class"`
}

type skillScanTruncatedContent struct {
	skillSourceEventBase
	RootID  string `json:"root_id"`
	Path    string `json:"path"`
	Scanned int    `json:"scanned"`
	Cap     int    `json:"cap"`
}

type skillScanLinkSkippedContent struct {
	skillSourceEventBase
	RootID string `json:"root_id"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

func (r *Registry) writeSkillSourcesApplied(
	ctx context.Context,
	generation int64,
	cfg RegistryConfig,
) error {
	correlation := sourceEventCorrelation(ctx, nil)
	rootCounts := cloneSourceRootCounts(correlation.RootCounts)
	if len(rootCounts) == 0 {
		rootCounts = make(map[string]int)
		for _, root := range cfg.GlobalSkillRoots {
			source := strings.TrimSpace(root.SourceSlug)
			if source == "" {
				source = compozyconfig.SkillSourceCompozy
			}
			rootCounts[source]++
		}
	}
	return r.writeSourceEvent(ctx, correlation, eventspkg.SkillSourcesApplied,
		"skill source policy applied",
		skillSourcesAppliedContent{
			skillSourceEventBase: sourceEventBase(correlation, generation),
			Scope:                normalizedSourceEventScope(correlation.Scope), Generation: generation,
			RootCounts: rootCounts,
		})
}

func (r *Registry) writeSkillSourcesSuperseded(
	ctx context.Context,
	discardedGeneration int64,
	winningGeneration int64,
) error {
	correlation := sourceEventCorrelation(ctx, nil)
	return r.writeSourceEvent(ctx, correlation, eventspkg.SkillSourcesSuperseded,
		"stale skill source policy discarded",
		skillSourcesSupersededContent{
			skillSourceEventBase: sourceEventBase(correlation, discardedGeneration),
			DiscardedGeneration:  discardedGeneration, WinningGeneration: winningGeneration,
		})
}

func (r *Registry) supersededGenerationError(
	ctx context.Context,
	discardedGeneration int64,
	winningGeneration int64,
) error {
	cause := fmt.Errorf("%w: %d", ErrConfigGenerationSuperseded, discardedGeneration)
	return errors.Join(cause, r.writeSkillSourcesSuperseded(ctx, discardedGeneration, winningGeneration))
}

// SourceApplyFailureError adds the canonical failure event without obscuring the original cause.
func (r *Registry) SourceApplyFailureError(ctx context.Context, generation int64, cause error) error {
	if cause == nil || errors.Is(cause, ErrConfigGenerationSuperseded) {
		return cause
	}
	return errors.Join(cause, r.EmitSourceApplyFailed(ctx, generation, cause))
}

// EmitSourceApplyFailed records a source apply failure after the active catalog has been preserved.
func (r *Registry) EmitSourceApplyFailed(
	ctx context.Context,
	generation int64,
	err error,
) error {
	if err == nil {
		return nil
	}
	correlation := sourceEventCorrelation(ctx, nil)
	return r.writeSourceEvent(ctx, correlation, eventspkg.SkillSourcesApplyFailed,
		"skill source policy apply failed",
		skillSourcesApplyFailedContent{
			skillSourceEventBase: sourceEventBase(correlation, generation),
			Scope:                normalizedSourceEventScope(correlation.Scope), Generation: generation,
			ErrorClass: sourceErrorClass(err),
		})
}

func (r *Registry) emitSkillScanEvents(
	ctx context.Context,
	root compozyconfig.SkillRootSpec,
	stats skillscan.RootScanStats,
) error {
	correlation := sourceEventCorrelation(ctx, &root)
	generation := ConfigGenerationFromContext(ctx)
	if stats.Truncated {
		if err := r.writeSourceEvent(ctx, correlation, eventspkg.SkillScanTruncated,
			"skill source scan truncated",
			skillScanTruncatedContent{
				skillSourceEventBase: sourceEventBase(correlation, generation),
				RootID:               root.RootID(), Path: root.Dir, Scanned: stats.ScannedCount,
				Cap: skillscan.MaxCandidates,
			}); err != nil {
			return err
		}
	}
	for _, skipped := range stats.SkippedLinks {
		if err := r.writeSourceEvent(ctx, correlation, eventspkg.SkillScanLinkSkipped,
			"skill source link skipped",
			skillScanLinkSkippedContent{
				skillSourceEventBase: sourceEventBase(correlation, generation),
				RootID:               root.RootID(), Path: skipped.Path, Reason: skipped.Reason,
			}); err != nil {
			return err
		}
	}
	return nil
}

func (r *Registry) writeSourceEvent(
	ctx context.Context,
	correlation SourceEventCorrelation,
	eventType string,
	summaryText string,
	content any,
) error {
	if r == nil || r.events == nil {
		return nil
	}
	payload, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("skills: marshal %s event: %w", eventType, err)
	}
	summary := store.EventSummary{
		ProfileID:   correlation.ProfileID,
		WorkspaceID: correlation.WorkspaceID,
		Type:        eventType,
		Summary:     summaryText,
		EventCorrelation: store.EventCorrelation{
			ActorKind: correlation.ActorKind,
			ActorID:   correlation.ActorID,
		},
	}
	summary.SetContent(payload)
	if err := r.events.WriteEventSummary(ctx, summary); err != nil {
		return fmt.Errorf("skills: persist %s event: %w", eventType, err)
	}
	return nil
}

func sourceEventCorrelation(
	ctx context.Context,
	root *compozyconfig.SkillRootSpec,
) SourceEventCorrelation {
	correlation := SourceEventCorrelation{}
	if ctx != nil {
		stored, ok := ctx.Value(sourceEventCorrelationContextKey{}).(SourceEventCorrelation)
		if ok {
			correlation = stored
		}
	}
	if root != nil {
		if strings.TrimSpace(correlation.ProfileID) == "" {
			correlation.ProfileID = strings.TrimSpace(root.ProfileID)
		}
		if strings.TrimSpace(correlation.WorkspaceID) == "" {
			correlation.WorkspaceID = strings.TrimSpace(root.WorkspaceID)
		}
		if strings.TrimSpace(correlation.Scope) == "" {
			correlation.Scope = strings.TrimSpace(string(root.ResourceScope.Kind.Normalize()))
		}
	}
	correlation.ProfileID = strings.TrimSpace(correlation.ProfileID)
	if correlation.ProfileID == "" {
		correlation.ProfileID = store.DefaultProfileID
	}
	correlation.WorkspaceID = strings.TrimSpace(correlation.WorkspaceID)
	correlation.ActorKind = strings.TrimSpace(correlation.ActorKind)
	if correlation.ActorKind == "" {
		correlation.ActorKind = defaultSkillSourceActorKind
	}
	correlation.ActorID = strings.TrimSpace(correlation.ActorID)
	if correlation.ActorID == "" {
		correlation.ActorID = defaultSkillSourceActorID
	}
	correlation.RootCounts = cloneSourceRootCounts(correlation.RootCounts)
	return correlation
}

func cloneSourceRootCounts(source map[string]int) map[string]int {
	if len(source) == 0 {
		return nil
	}
	return maps.Clone(source)
}

func sourceEventBase(correlation SourceEventCorrelation, generation int64) skillSourceEventBase {
	return skillSourceEventBase{
		ProfileID:        correlation.ProfileID,
		WorkspaceID:      correlation.WorkspaceID,
		ConfigGeneration: generation,
		ActorKind:        correlation.ActorKind,
		ActorID:          correlation.ActorID,
	}
}

func normalizedSourceEventScope(scope string) string {
	if normalized := strings.TrimSpace(scope); normalized != "" {
		return normalized
	}
	return registryUserKey
}

func sourceErrorClass(err error) string {
	switch {
	case errors.Is(err, ErrConfigGenerationSuperseded):
		return "superseded"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "canceled"
	default:
		return "runtime"
	}
}
