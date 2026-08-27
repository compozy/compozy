package consolidation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	speedpkg "github.com/compozy/compozy/internal/speed"
)

func createDreamSessionWithFallback(
	ctx context.Context,
	sessions SessionManager,
	route SessionRoute,
	goal string,
	workspace string,
) (*session.Session, error) {
	create := func(
		provider string,
		model string,
		reasoningEffort string,
		speed speedpkg.Speed,
		options []acp.SessionConfigOptionSelection,
	) (*session.Session, error) {
		return sessions.Create(ctx, session.CreateOpts{
			AgentName:       strings.TrimSpace(route.AgentName),
			Provider:        strings.TrimSpace(provider),
			Model:           strings.TrimSpace(model),
			ReasoningEffort: strings.TrimSpace(reasoningEffort),
			Speed:           speed,
			ACPOptions:      acp.CloneSessionConfigOptionSelections(options),
			Name:            strings.TrimSpace(goal),
			Workspace:       strings.TrimSpace(workspace),
			Type:            session.SessionTypeDream,
		})
	}

	created, err := create(route.Provider, route.Model, route.ReasoningEffort, route.Speed, route.ACPOptions)
	if created != nil {
		return created, err
	}
	attemptErrors := []error{dreamAttemptError(0, err)}
	for index, fallback := range route.Fallbacks {
		attempt := index + 1
		if route.BeforeFallback != nil {
			if eventErr := route.BeforeFallback(ctx, attempt, fallback); eventErr != nil {
				return nil, errors.Join(append(attemptErrors, eventErr)...)
			}
		}
		created, err = create(
			fallback.Provider,
			fallback.Model,
			fallback.ReasoningEffort,
			fallback.Speed,
			dreamACPOptionsForSession(fallback.ACPOptions),
		)
		if created != nil {
			return created, err
		}
		attemptErrors = append(attemptErrors, dreamAttemptError(attempt, err))
	}
	return nil, fmt.Errorf(
		"dream role invocation exhausted %d attempt(s): %w",
		len(route.Fallbacks)+1,
		errors.Join(attemptErrors...),
	)
}

func dreamACPOptionsForSession(
	options []compozyconfig.ACPOptionSelection,
) []acp.SessionConfigOptionSelection {
	if len(options) == 0 {
		return nil
	}
	converted := make([]acp.SessionConfigOptionSelection, 0, len(options))
	for _, option := range options {
		candidate := acp.SessionConfigOptionSelection{
			ID:      strings.TrimSpace(option.ID),
			ValueID: strings.TrimSpace(option.ValueID),
		}
		if option.BoolValue != nil {
			candidate.BoolValue = new(*option.BoolValue)
		}
		converted = append(converted, candidate)
	}
	return converted
}

func dreamAttemptError(attempt int, err error) error {
	if err == nil {
		err = errors.New("invocation returned without acceptance")
	}
	return fmt.Errorf("dream role attempt %d failed before acceptance: %w", attempt, err)
}
