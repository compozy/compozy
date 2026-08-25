package settings

import (
	"context"
	"strings"

	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/store"
)

func (s *service) withSkillSourceEventCorrelation(
	ctx context.Context,
	result MutationResult,
) (context.Context, error) {
	if result.Section != SectionSkills {
		return ctx, nil
	}
	profileID := store.DefaultProfileID
	profileName := strings.TrimSpace(result.ProfileName)
	if profileName != "" && s.profileResolver != nil {
		resolvedID, err := s.profileResolver.AvailableProfileID(ctx, profileName)
		if err != nil {
			return ctx, err
		}
		profileID = strings.TrimSpace(resolvedID)
	}
	actorID := mutationSourceFromContext(ctx)
	return skillspkg.WithSourceEventCorrelation(ctx, skillspkg.SourceEventCorrelation{
		Scope:       strings.TrimSpace(string(result.Scope)),
		ProfileID:   profileID,
		WorkspaceID: strings.TrimSpace(result.WorkspaceID),
		ActorKind:   "settings",
		ActorID:     actorID,
	}), nil
}
