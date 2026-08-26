package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
)

const (
	exposureEventCreated        = eventspkg.SkillExposureCreated
	exposureEventRemoved        = eventspkg.SkillExposureRemoved
	exposureEventCleanupFailure = eventspkg.SkillExposureCleanupFailed
)

type exposureEventContent struct {
	Skill            string `json:"skill"`
	Target           string `json:"target"`
	LinkPath         string `json:"link_path,omitempty"`
	Status           string `json:"status,omitempty"`
	ErrorClass       string `json:"error_class,omitempty"`
	ProfileID        string `json:"profile_id"`
	ActorKind        string `json:"actor_kind,omitempty"`
	ActorID          string `json:"actor_id,omitempty"`
	ConfigGeneration int64  `json:"config_generation"`
	OwnerScope       string `json:"owner_scope,omitempty"`
	WorkspaceID      string `json:"workspace_id,omitempty"`
}

func (m *ExposeManager) emitExposureEvent(
	ctx context.Context,
	eventType string,
	record ExposureRecord,
	status ExposureStatus,
	err error,
) {
	if m == nil || m.events == nil {
		return
	}
	correlation := SourceEventCorrelationFromContext(ctx)
	content, marshalErr := json.Marshal(exposureEventContent{
		Skill: record.SkillName, Target: record.TargetSlug, LinkPath: record.LinkPath,
		Status: string(status), ErrorClass: exposureErrorCode(err),
		ProfileID: correlation.ProfileID, ActorKind: correlation.ActorKind, ActorID: correlation.ActorID,
		ConfigGeneration: ConfigGenerationFromContext(ctx),
		OwnerScope:       string(record.OwnerScope), WorkspaceID: record.WorkspaceID,
	})
	if marshalErr != nil {
		m.logger.Warn("skills: marshal exposure event", "type", eventType, "error", marshalErr)
		return
	}
	summary := store.EventSummary{
		ProfileID:   normalizedSkillEventProfileID(correlation.ProfileID),
		WorkspaceID: strings.TrimSpace(record.WorkspaceID),
		Type:        eventType,
		Summary:     fmt.Sprintf("skill %s exposure %s for %s", record.SkillName, eventType, record.TargetSlug),
		EventCorrelation: store.EventCorrelation{
			ActorKind: correlation.ActorKind,
			ActorID:   correlation.ActorID,
		},
	}
	summary.SetContent(content)
	if writeErr := m.events.WriteEventSummary(ctx, summary); writeErr != nil {
		m.logger.Warn("skills: write exposure event", "type", eventType, "error", writeErr)
	}
}

func (m *ExposeManager) emitExposureFailure(
	ctx context.Context,
	skill *Skill,
	target string,
	linkPath string,
	err error,
) {
	m.emitSkillExposureFailure(ctx, eventspkg.SkillExposureOperationFailed, skill, target, linkPath, err)
}

func (m *ExposeManager) emitExposureCleanupFailure(
	ctx context.Context,
	skill *Skill,
	target string,
	linkPath string,
	err error,
) {
	m.emitSkillExposureFailure(ctx, eventspkg.SkillExposureCleanupFailed, skill, target, linkPath, err)
}

func (m *ExposeManager) emitSkillExposureFailure(
	ctx context.Context,
	eventType string,
	skill *Skill,
	target string,
	linkPath string,
	err error,
) {
	record := ExposureRecord{TargetSlug: target, LinkPath: linkPath}
	if skill != nil {
		record.SkillName = strings.TrimSpace(skill.Meta.Name)
		record.OwnerScope, record.WorkspaceID = exposureEventOwner(skill)
	}
	m.emitExposureEvent(ctx, eventType, record, "", err)
}

func (m *ExposeManager) emitExposureDivergence(ctx context.Context, state ExposureState) {
	if state.Status == ExposureHealthy {
		return
	}
	m.emitExposureEvent(ctx, eventspkg.SkillExposureBrokenDetected, state.Record, state.Status, nil)
}

func exposureEventOwner(skill *Skill) (store.SkillExposureOwnerScope, string) {
	owner, err := exposureOwnerFromScope(skill.ResourceScope.Normalize())
	if err != nil {
		return "", ""
	}
	return owner.scope, owner.workspaceID
}
