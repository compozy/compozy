package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

const teamsTypingActivityType = "typing"

type teamsProgressSink struct {
	api               teamsAPI
	cfg               resolvedInstanceConfig
	event             bridgepkg.DeliveryEvent
	userContextLookup func(string, string) (teamsUserContext, bool)
	target            teamsResolvedTarget
	targetResolved    bool
}

var _ bridgesdk.ProgressSink = (*teamsProgressSink)(nil)

func (p *teamsProvider) handleBridgesProgress(
	ctx context.Context,
	session *bridgesdk.Session,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	cfg, err := p.waitForInstanceConfig(
		strings.TrimSpace(request.Event.BridgeInstanceID),
		500*time.Millisecond,
	)
	if err != nil {
		p.setLastError(err)
		return bridgepkg.DeliveryAck{}, err
	}
	if cfg.managed == nil {
		return bridgepkg.DeliveryAck{}, errors.New("teams: managed bridge instance is required for progress")
	}

	progressConfig := bridgepkg.ResolveProgressConfig(&cfg.managed.Instance, providerTeamsKey)
	if progressConfig.ToolProgress == bridgepkg.ProgressModeOff {
		p.detachTeamsProgressDispatcher(
			cfg.instanceID,
			request.Event.DeliveryID,
			request.Event.IsLifecycleOnlyFinal(),
		)
		return session.AckDelivery(request, "", "")
	}
	if request.Event.IsLifecycleOnlyFinal() {
		return p.finishTeamsProgress(ctx, session, cfg, request)
	}
	if request.Event.Progress == nil {
		return bridgepkg.DeliveryAck{}, errors.New("teams: progress payload is required")
	}

	state, _ := p.deliveries.UpdateOrCreate(
		deliveryStateKey(cfg.instanceID, request.Event.DeliveryID),
		func(state deliveryState, _ bool) deliveryState {
			if state.Progress == nil {
				state.Progress = p.newTeamsProgressDispatcher(cfg, request.Event, progressConfig)
			}
			return state
		},
	)
	if err := state.Progress.OnProgress(ctx, *request.Event.Progress); err != nil {
		p.reportTeamsProgressFailure(ctx, session, cfg.instanceID, err)
		return bridgepkg.DeliveryAck{}, err
	}
	return session.AckDelivery(request, "", "")
}

func (p *teamsProvider) newTeamsProgressDispatcher(
	cfg resolvedInstanceConfig,
	event bridgepkg.DeliveryEvent,
	progressConfig bridgepkg.ProgressConfig,
) *bridgesdk.ProgressDispatcher {
	sink := &teamsProgressSink{
		api:               p.apiFactory(cfg),
		cfg:               cfg,
		event:             event,
		userContextLookup: p.userContext,
	}
	accumulator := bridgesdk.NewProgressAccumulator(bridgesdk.ProgressConfig{
		ProgressConfig: progressConfig,
		Target:         event.DeliveryTarget,
		MaxMessageLen:  teamsMaxMessageLen,
		Len:            utf8.RuneCountInString,
	}, sink, p.now)
	return bridgesdk.NewProgressDispatcher(
		accumulator,
		bridgesdk.WithProgressAsyncErrorHandler(func(err error) {
			wrapped := fmt.Errorf("teams: flush scheduled progress: %w", err)
			p.markers.ReportError("flush scheduled Teams progress", wrapped)
			p.setLastError(wrapped)
		}),
	)
}

func (p *teamsProvider) finishTeamsProgress(
	ctx context.Context,
	session *bridgesdk.Session,
	cfg resolvedInstanceConfig,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	state := p.deliveryState(cfg.instanceID, request.Event.DeliveryID)
	dispatcher := state.Progress
	if dispatcher == nil {
		p.detachTeamsProgressDispatcher(cfg.instanceID, request.Event.DeliveryID, true)
		return session.AckDelivery(request, "", "")
	}
	if err := dispatcher.Flush(ctx); err != nil {
		p.reportTeamsProgressFailure(ctx, session, cfg.instanceID, err)
		return bridgepkg.DeliveryAck{}, err
	}
	cleanupErr := dispatcher.OnContent(ctx)
	p.detachTeamsProgressDispatcher(cfg.instanceID, request.Event.DeliveryID, true)
	if cleanupErr != nil {
		p.recordTeamsProgressCleanupError(cleanupErr)
	}
	return session.AckDelivery(request, "", "")
}

func (p *teamsProvider) reportTeamsProgressFailure(
	ctx context.Context,
	session *bridgesdk.Session,
	instanceID string,
	err error,
) {
	classified := bridgesdk.ClassifyError(err)
	_, _, reportErr := session.ReportClassifiedError(ctx, instanceID, classified)
	if reportErr != nil {
		p.setLastError(reportErr)
		return
	}
	p.setLastError(err)
}

func (p *teamsProvider) recordTeamsProgressCleanupError(err error) {
	if err == nil {
		return
	}
	wrapped := fmt.Errorf("teams: close progress after content: %w", err)
	p.markers.ReportError("close Teams progress after content", wrapped)
	p.setLastError(wrapped)
}

func (p *teamsProvider) closeAllTeamsProgressDispatchers() {
	dispatchers := make([]*bridgesdk.ProgressDispatcher, 0)
	p.deliveries.TransformAll(func(_ string, state deliveryState) deliveryState {
		if state.Progress == nil {
			return state
		}
		dispatchers = append(dispatchers, state.Progress)
		state.Progress = nil
		return state
	})

	for _, dispatcher := range dispatchers {
		dispatcher.Close()
	}
}

func (p *teamsProvider) detachTeamsProgressDispatcher(
	instanceID string,
	deliveryID string,
	deleteDelivery bool,
) {
	key := deliveryStateKey(instanceID, deliveryID)
	state, _ := p.deliveries.Load(key)
	dispatcher := state.Progress
	if deleteDelivery {
		p.deliveries.Delete(key)
	} else if dispatcher != nil {
		p.deliveries.Update(key, func(current deliveryState) deliveryState {
			current.Progress = nil
			return current
		})
	}
	if dispatcher != nil {
		dispatcher.Close()
	}
}

func (s *teamsProgressSink) Post(
	ctx context.Context,
	_ bridgepkg.DeliveryTarget,
	text string,
) (string, error) {
	target, err := s.resolveTarget(ctx)
	if err != nil {
		return "", err
	}
	response, err := s.api.SendActivity(
		ctx,
		target.ServiceURL,
		target.ConversationID,
		target.ReplyToID,
		teamsOutboundActivity{
			Type:       providerMessageKey,
			Text:       text,
			TextFormat: teamsTextFormatMarkdown,
		},
	)
	if err != nil {
		return "", fmt.Errorf("teams: send progress activity: %w", err)
	}
	if response == nil || strings.TrimSpace(response.ID) == "" {
		return "", &bridgesdk.CommittedMutationError{
			Err: errors.New("teams: send progress activity response omitted id"),
		}
	}
	return encodeRemoteMessageID(teamsRemoteMessageRef{
		ConversationID: target.ConversationID,
		ServiceURL:     target.ServiceURL,
		ActivityID:     strings.TrimSpace(response.ID),
	}), nil
}

func (s *teamsProgressSink) Edit(
	ctx context.Context,
	_ bridgepkg.DeliveryTarget,
	remoteID string,
	text string,
) error {
	ref, err := decodeRemoteMessageID(remoteID)
	if err != nil {
		return err
	}
	_, err = bridgesdk.RetryDo(ctx, bridgesdk.DefaultRetryConfig(), func(retryCtx context.Context) (struct{}, error) {
		updateErr := s.api.UpdateActivity(
			retryCtx,
			ref.ServiceURL,
			ref.ConversationID,
			ref.ActivityID,
			teamsOutboundActivity{
				Type:       providerMessageKey,
				Text:       text,
				TextFormat: teamsTextFormatMarkdown,
			},
		)
		return struct{}{}, updateErr
	})
	if err != nil {
		return fmt.Errorf("teams: update progress activity: %w", err)
	}
	return nil
}

func (s *teamsProgressSink) Typing(
	ctx context.Context,
	_ bridgepkg.DeliveryTarget,
	active bool,
) error {
	if !active {
		return nil
	}
	target, err := s.resolveTarget(ctx)
	if err != nil {
		return err
	}
	if _, err := s.api.SendActivity(
		ctx,
		target.ServiceURL,
		target.ConversationID,
		target.ReplyToID,
		teamsOutboundActivity{Type: teamsTypingActivityType},
	); err != nil {
		return fmt.Errorf("teams: send typing activity: %w", err)
	}
	return nil
}

func (s *teamsProgressSink) React(
	context.Context,
	bridgepkg.DeliveryTarget,
	string,
) error {
	return nil
}

func (s *teamsProgressSink) resolveTarget(ctx context.Context) (teamsResolvedTarget, error) {
	if s.targetResolved {
		return s.target, nil
	}
	target, err := resolveTeamsDeliveryTarget(s.cfg, s.event, s.userContextLookup)
	if err != nil {
		return teamsResolvedTarget{}, err
	}
	target, err = ensureTeamsConversation(ctx, s.api, s.cfg, target)
	if err != nil {
		return teamsResolvedTarget{}, err
	}
	s.target = target
	s.targetResolved = true
	return target, nil
}
