package skills

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	eventspkg "github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/store"
)

const (
	observeEventsAgentLocalValue = "agent-local"
)

type skillShadowContent struct {
	SkillID         string                  `json:"skill_id"`
	WinnerPath      string                  `json:"winner_path"`
	WinnerTier      string                  `json:"winner_tier"`
	Losers          []skillShadowEventLoser `json:"losers"`
	DetectedAt      string                  `json:"detected_at"`
	ResolutionScope string                  `json:"resolution_scope"`
	AgentName       string                  `json:"agent_name,omitempty"`
	WorkspaceID     string                  `json:"workspace_id,omitempty"`
}

type skillShadowEventLoser struct {
	Path string `json:"path"`
	Tier string `json:"tier"`
}

type skillLoadFailedContent struct {
	AgentName   string `json:"agent_name"`
	Source      string `json:"source"`
	Path        string `json:"path,omitempty"`
	ErrorCode   string `json:"error_code"`
	ErrorDetail string `json:"error_detail,omitempty"`
}

type agentLocalLoadError struct {
	path   string
	code   string
	detail string
	err    error
}

func (e *agentLocalLoadError) Error() string {
	if e == nil {
		return ""
	}
	if e.err != nil {
		return e.err.Error()
	}
	return strings.TrimSpace(e.detail)
}

func (e *agentLocalLoadError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func newAgentLocalLoadError(code string, path string, detail string, err error) error {
	return &agentLocalLoadError{
		path:   strings.TrimSpace(path),
		code:   strings.TrimSpace(code),
		detail: strings.TrimSpace(detail),
		err:    err,
	}
}

func agentLocalLoadErrorDetails(err error) (path string, code string, detail string) {
	typed := &agentLocalLoadError{}
	if !errors.As(err, &typed) || typed == nil {
		return "", "validation", strings.TrimSpace(err.Error())
	}
	return strings.TrimSpace(typed.path), strings.TrimSpace(typed.code), strings.TrimSpace(typed.detail)
}

func (r *Registry) buildSkillShadowSummaries(
	base map[string]*Skill,
	overlay map[string]*Skill,
	resolutionScope string,
	workspaceID string,
	agentName string,
) []store.EventSummary {
	if r == nil || len(base) == 0 || len(overlay) == 0 {
		return nil
	}

	names := make([]string, 0, len(overlay))
	for name, skill := range overlay {
		if skill == nil || base[name] == nil {
			continue
		}
		names = append(names, name)
	}
	slices.Sort(names)

	summaries := make([]store.EventSummary, 0, len(names))
	for _, name := range names {
		existing := base[name]
		next := overlay[name]
		if existing == nil || next == nil {
			continue
		}

		detectedAt := normalizedShadowDetectedAt(r.now())
		winner := cloneSkill(next)
		winner.Diagnostics.ShadowedDefinitions = append(
			cloneSkillDefinitionRefs(winner.Diagnostics.ShadowedDefinitions),
			shadowDefinitionRefsForWinner(existing, detectedAt)...,
		)
		shadows, ok := ShadowsForSkill(winner, detectedAt)
		if !ok {
			continue
		}
		losers := skillShadowEventLosers(shadows.Shadows)
		if len(losers) == 0 {
			continue
		}

		content, err := json.Marshal(skillShadowContent{
			SkillID:         strings.TrimSpace(shadows.Name),
			WinnerPath:      strings.TrimSpace(shadows.Winner.Path),
			WinnerTier:      strings.TrimSpace(shadows.Winner.Tier),
			Losers:          losers,
			DetectedAt:      detectedAt.Format(time.RFC3339Nano),
			ResolutionScope: strings.TrimSpace(resolutionScope),
			AgentName:       strings.TrimSpace(agentName),
			WorkspaceID:     strings.TrimSpace(workspaceID),
		})
		if err != nil {
			r.logger.Warn("skills: marshal skill.shadowed content failed", "name", next.Meta.Name, "error", err)
			continue
		}

		summaries = append(summaries, store.EventSummary{
			WorkspaceID: strings.TrimSpace(workspaceID),
			AgentName:   strings.TrimSpace(agentName),
			Type:        eventspkg.SkillShadowed,
			Content:     content,
			Summary: fmt.Sprintf(
				"skill %s resolved to %s and shadowed %d declaration(s)",
				strings.TrimSpace(shadows.Name),
				strings.TrimSpace(shadows.Winner.Tier),
				len(losers),
			),
		})
	}
	return summaries
}

func (r *Registry) buildSkillShadowSummariesFromResolved(
	skills []*Skill,
	resolutionScope string,
	workspaceID string,
	agentName string,
) []store.EventSummary {
	if r == nil || len(skills) == 0 {
		return nil
	}
	detectedAt := normalizedShadowDetectedAt(r.now())
	summaries := make([]store.EventSummary, 0)
	for _, skill := range skills {
		if skill == nil || len(skill.Diagnostics.ShadowedDefinitions) == 0 {
			continue
		}
		shadows, ok := ShadowsForSkill(skill, detectedAt)
		if !ok {
			continue
		}
		losers := skillShadowEventLosers(shadows.Shadows)
		if len(losers) == 0 {
			continue
		}
		content, err := json.Marshal(skillShadowContent{
			SkillID:         strings.TrimSpace(shadows.Name),
			WinnerPath:      strings.TrimSpace(shadows.Winner.Path),
			WinnerTier:      strings.TrimSpace(shadows.Winner.Tier),
			Losers:          losers,
			DetectedAt:      detectedAt.Format(time.RFC3339Nano),
			ResolutionScope: strings.TrimSpace(resolutionScope),
			AgentName:       strings.TrimSpace(agentName),
			WorkspaceID:     strings.TrimSpace(workspaceID),
		})
		if err != nil {
			r.logger.Warn("skills: marshal skill.shadowed content failed", "name", skill.Meta.Name, "error", err)
			continue
		}
		summaries = append(summaries, store.EventSummary{
			WorkspaceID: strings.TrimSpace(workspaceID),
			AgentName:   strings.TrimSpace(agentName),
			Type:        eventspkg.SkillShadowed,
			Content:     content,
			Summary: fmt.Sprintf(
				"skill %s resolved to %s and shadowed %d declaration(s)",
				strings.TrimSpace(shadows.Name),
				strings.TrimSpace(shadows.Winner.Tier),
				len(losers),
			),
		})
	}
	return summaries
}

func skillShadowEventLosers(entries []ShadowEntry) []skillShadowEventLoser {
	if len(entries) == 0 {
		return nil
	}
	losers := make([]skillShadowEventLoser, 0, len(entries))
	for _, entry := range entries {
		if entry.ResolvedToWinner {
			continue
		}
		losers = append(losers, skillShadowEventLoser{
			Path: strings.TrimSpace(entry.Path),
			Tier: precedenceTierFromSourceLabel(entry.Tier),
		})
	}
	return losers
}

func (r *Registry) emitEventSummaries(ctx context.Context, summaries []store.EventSummary) {
	if r == nil || r.events == nil || len(summaries) == 0 {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for _, summary := range summaries {
		if err := r.events.WriteEventSummary(ctx, summary); err != nil {
			r.logger.Warn("skills: write observe event failed", "type", summary.Type, "error", err)
		}
	}
}

func (r *Registry) emitSkillsLoadFailed(ctx context.Context, workspaceID string, agentName string, err error) {
	if r == nil || r.events == nil || err == nil {
		return
	}

	path, code, detail := agentLocalLoadErrorDetails(err)
	content, marshalErr := json.Marshal(skillLoadFailedContent{
		AgentName:   strings.TrimSpace(agentName),
		Source:      observeEventsAgentLocalValue,
		Path:        path,
		ErrorCode:   code,
		ErrorDetail: detail,
	})
	if marshalErr != nil {
		r.logger.Warn("skills: marshal skills.load_failed content failed", "agent_name", agentName, "error", marshalErr)
		return
	}

	if ctx == nil {
		ctx = context.Background()
	}
	if writeErr := r.events.WriteEventSummary(ctx, store.EventSummary{
		WorkspaceID: strings.TrimSpace(workspaceID),
		AgentName:   strings.TrimSpace(agentName),
		Type:        eventspkg.SkillLoadFailed,
		Content:     content,
		Summary:     fmt.Sprintf("agent-local skills load failed for %s", strings.TrimSpace(agentName)),
	}); writeErr != nil {
		r.logger.Warn("skills: write skills.load_failed failed", "agent_name", agentName, "error", writeErr)
	}
}
