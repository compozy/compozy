package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cmdpalette"
	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (s *daemonExtensionService) ListEnablement(
	ctx context.Context,
	name string,
) ([]contract.ExtensionEnablementPayload, error) {
	if err := s.checkProfileEnablementReady(); err != nil {
		return nil, err
	}
	if _, err := s.registry.Get(name); err != nil {
		return nil, err
	}
	profiles, err := s.profiles.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("daemon: list profiles for extension enablement: %w", err)
	}
	items := make([]contract.ExtensionEnablementPayload, 0, len(profiles))
	for _, profile := range profiles {
		enabled, enabledErr := s.registry.IsEnabledForProfile(name, profile.ID)
		if enabledErr != nil {
			return nil, enabledErr
		}
		items = append(items, contract.ExtensionEnablementPayload{Profile: profile.Name, Enabled: enabled})
	}
	return items, nil
}

func (s *daemonExtensionService) SetEnablement(
	ctx context.Context,
	name string,
	req contract.SetExtensionEnablementRequest,
	actor taskpkg.ActorContext,
) (contract.ExtensionEnablementPayload, error) {
	if err := s.checkProfileEnablementReady(); err != nil {
		return contract.ExtensionEnablementPayload{}, err
	}
	boundActor, err := s.bindExtensionEnablementActor(ctx, actor)
	if err != nil {
		return contract.ExtensionEnablementPayload{}, err
	}
	actor = boundActor
	if err := validateExtensionWriteActor(actor); err != nil {
		return contract.ExtensionEnablementPayload{}, err
	}
	profileName := strings.TrimSpace(req.Profile)
	if profileName == "" {
		return contract.ExtensionEnablementPayload{}, errors.New("daemon: extension enablement profile is required")
	}
	requestedProfile, err := s.profiles.GetByName(ctx, profileName)
	if err != nil {
		return contract.ExtensionEnablementPayload{}, err
	}
	if !actor.ReadScope.AllProfiles {
		callerProfileID := strings.TrimSpace(actor.ReadScope.ProfileID)
		if callerProfileID != "" && callerProfileID != requestedProfile.ID {
			return contract.ExtensionEnablementPayload{}, fmt.Errorf(
				"daemon: extension enablement profile %q is outside caller profile scope", profileName,
			)
		}
	}
	var result contract.ExtensionEnablementPayload
	err = s.lifecycle.withName(ctx, name, func() error {
		if err := s.profiles.EnsureAvailableName(ctx, profileName); err != nil {
			return err
		}
		profile, err := s.profiles.GetByName(ctx, profileName)
		if err != nil {
			return err
		}
		previous, err := s.registry.IsEnabledForProfile(name, profile.ID)
		if err != nil {
			return err
		}
		result = contract.ExtensionEnablementPayload{Profile: profile.Name, Enabled: req.Enabled}
		if previous == req.Enabled {
			return nil
		}
		if err := s.registry.SetEnabledForProfile(name, profile.ID, req.Enabled); err != nil {
			return err
		}
		if err := s.reload(ctx); err != nil {
			return s.rollbackProfileEnablement(ctx, name, profile.ID, previous, err)
		}
		if err := s.recordExtensionEnablementChanged(
			ctx, actor, name, profile.ID, profile.Name, req.Enabled,
		); err != nil {
			return s.rollbackProfileEnablement(ctx, name, profile.ID, previous, err)
		}
		if s.paletteNotifier != nil {
			if err := s.paletteNotifier.NotifyExtensionProfileChanged(
				ctx,
				"",
				name,
				cmdpalette.ScopedProfileLens(cmdpalette.ProfileLensID(profile.ID), profile.Name),
			); err != nil {
				return s.rollbackProfileEnablement(
					ctx, name, profile.ID, previous,
					fmt.Errorf("daemon: invalidate extension palette: %w", err),
				)
			}
		}
		return nil
	})
	if err != nil {
		return contract.ExtensionEnablementPayload{}, err
	}
	return result, nil
}

func (s *daemonExtensionService) bindExtensionEnablementActor(
	ctx context.Context,
	actor taskpkg.ActorContext,
) (taskpkg.ActorContext, error) {
	if actor.Actor.Kind.Normalize() != taskpkg.ActorKindAgentSession ||
		strings.TrimSpace(actor.ReadScope.ProfileID) != "" || actor.ReadScope.AllProfiles {
		return actor, nil
	}
	if s.sessions == nil {
		return taskpkg.ActorContext{}, errors.New("daemon: extension enablement session profile lookup is unavailable")
	}
	sessionID := strings.TrimSpace(actor.Scope.SessionID)
	if sessionID == "" {
		return taskpkg.ActorContext{}, errors.New("daemon: extension enablement agent session is missing")
	}
	info, err := s.sessions.Status(ctx, sessionID)
	if err != nil {
		return taskpkg.ActorContext{}, fmt.Errorf("daemon: resolve extension enablement session profile: %w", err)
	}
	if info == nil || strings.TrimSpace(info.ProfileID) == "" {
		return taskpkg.ActorContext{}, errors.New("daemon: extension enablement session has no bound profile")
	}
	actor.ReadScope = store.ReadScope{ProfileID: strings.TrimSpace(info.ProfileID)}
	return actor, actor.Validate()
}

func (s *daemonExtensionService) rollbackProfileEnablement(
	ctx context.Context,
	name string,
	profileID string,
	previous bool,
	operationErr error,
) error {
	rollbackCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		extensionLifecycleRollbackTimeout,
	)
	defer cancel()
	var rollbackErr error
	if err := s.registry.SetEnabledForProfile(name, profileID, previous); err != nil {
		rollbackErr = errors.Join(rollbackErr, fmt.Errorf("daemon: restore extension enablement: %w", err))
	}
	if err := s.reload(rollbackCtx); err != nil {
		rollbackErr = errors.Join(rollbackErr, fmt.Errorf("daemon: reload restored extension enablement: %w", err))
	}
	return errors.Join(operationErr, rollbackErr)
}

func (s *daemonExtensionService) checkProfileEnablementReady() error {
	if err := s.checkReady(); err != nil {
		return err
	}
	if s.profiles == nil {
		return errors.New("daemon: profile manager is required for extension enablement")
	}
	return nil
}

func (s *daemonExtensionService) recordExtensionEnablementChanged(
	ctx context.Context,
	actor taskpkg.ActorContext,
	extensionName string,
	profileID string,
	profileName string,
	enabled bool,
) error {
	if s.eventWriter == nil {
		return nil
	}
	payload := struct {
		ExtensionName string `json:"extension_name"`
		ProfileID     string `json:"profile_id"`
		ProfileName   string `json:"profile_name"`
		ActorKind     string `json:"actor_kind"`
		Enabled       bool   `json:"enabled"`
	}{
		ExtensionName: strings.TrimSpace(extensionName), ProfileID: profileID,
		ProfileName: profileName, ActorKind: string(actor.Actor.Kind.Normalize()), Enabled: enabled,
	}
	content, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("daemon: encode extension enablement event: %w", err)
	}
	if err := s.eventWriter.WriteEventSummary(context.WithoutCancel(ctx), daemonEventSummary(store.EventSummary{
		ProfileID: profileID, Type: eventspkg.ExtensionEnablementChanged,
		Outcome:   string(eventspkg.OutcomeFor(eventspkg.ExtensionEnablementChanged)),
		Summary:   fmt.Sprintf("extension %s enablement changed for profile %s", extensionName, profileName),
		Timestamp: s.now().UTC(), EventCorrelation: store.EventCorrelation{
			ActorKind: string(actor.Actor.Kind.Normalize()), ActorID: strings.TrimSpace(actor.Actor.Ref),
		},
	}, content)); err != nil {
		return fmt.Errorf("daemon: record extension enablement event: %w", err)
	}
	return nil
}
