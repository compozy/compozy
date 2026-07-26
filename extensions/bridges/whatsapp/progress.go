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

type whatsappProgressSink struct {
	api           whatsappAPI
	phoneNumberID string
	targetUserID  string
	lastText      string
}

var _ bridgesdk.ProgressSink = (*whatsappProgressSink)(nil)

func (p *whatsappProvider) handleBridgesProgress(
	ctx context.Context,
	session *bridgesdk.Session,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	cfg, err := p.waitForInstanceConfig(
		strings.TrimSpace(request.Event.BridgeInstanceID),
		500*time.Millisecond,
	)
	if err != nil && !errors.Is(err, errWhatsAppInstanceConfigInvalid) {
		p.setInstanceError(request.Event.BridgeInstanceID, err)
		return bridgepkg.DeliveryAck{}, err
	}
	if cfg.managed == nil {
		return bridgepkg.DeliveryAck{}, errors.New("whatsapp: managed bridge instance is required for progress")
	}

	progressConfig := bridgepkg.ResolveProgressConfig(&cfg.managed.Instance, providerWhatsappKey)
	if progressConfig.ToolProgress == bridgepkg.ProgressModeOff {
		p.detachWhatsAppProgressDispatcher(
			cfg.instanceID,
			request.Event.DeliveryID,
			request.Event.IsLifecycleOnlyFinal(),
		)
		return session.AckDelivery(request, "", "")
	}
	if err != nil {
		p.setInstanceError(request.Event.BridgeInstanceID, err)
		return bridgepkg.DeliveryAck{}, err
	}
	if request.Event.IsLifecycleOnlyFinal() {
		return p.finishWhatsAppProgress(ctx, session, cfg, request)
	}
	if request.Event.Progress == nil {
		return bridgepkg.DeliveryAck{}, errors.New("whatsapp: progress payload is required")
	}

	var resolveErr error
	state, _ := p.deliveries.UpdateOrCreate(
		deliveryStateKey(cfg.instanceID, request.Event.DeliveryID),
		func(state deliveryState, _ bool) deliveryState {
			if state.Progress != nil {
				return state
			}
			targetUserID, err := resolveDeliveryTarget(request.Event)
			if err != nil {
				resolveErr = err
				return state
			}
			state.Progress = p.newWhatsAppProgressDispatcher(
				cfg,
				request.Event,
				progressConfig,
				targetUserID,
			)
			return state
		},
	)
	if resolveErr != nil {
		wrapped := fmt.Errorf("whatsapp: resolve progress target: %w", resolveErr)
		p.reportWhatsAppProgressFailure(ctx, session, cfg.instanceID, wrapped)
		return bridgepkg.DeliveryAck{}, wrapped
	}
	if err := state.Progress.OnProgress(ctx, *request.Event.Progress); err != nil {
		p.reportWhatsAppProgressFailure(ctx, session, cfg.instanceID, err)
		return bridgepkg.DeliveryAck{}, err
	}
	return session.AckDelivery(request, "", "")
}

func (p *whatsappProvider) newWhatsAppProgressDispatcher(
	cfg resolvedInstanceConfig,
	event bridgepkg.DeliveryEvent,
	progressConfig bridgepkg.ProgressConfig,
	targetUserID string,
) *bridgesdk.ProgressDispatcher {
	sink := &whatsappProgressSink{
		api:           p.apiFactory(cfg),
		phoneNumberID: cfg.phoneNumberID,
		targetUserID:  targetUserID,
	}
	accumulator := bridgesdk.NewProgressAccumulator(bridgesdk.ProgressConfig{
		ProgressConfig: progressConfig,
		Target:         event.DeliveryTarget,
		MaxMessageLen:  whatsappMaxMessageRunes,
		Len:            utf8.RuneCountInString,
	}, sink, p.now)
	return bridgesdk.NewProgressDispatcher(
		accumulator,
		bridgesdk.WithProgressAsyncErrorHandler(func(err error) {
			wrapped := fmt.Errorf("whatsapp: flush scheduled progress: %w", err)
			p.markers.ReportError("flush scheduled WhatsApp progress", wrapped)
			p.setInstanceError(cfg.instanceID, wrapped)
		}),
	)
}

func (p *whatsappProvider) finishWhatsAppProgress(
	ctx context.Context,
	session *bridgesdk.Session,
	cfg resolvedInstanceConfig,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	state := p.deliveryState(cfg.instanceID, request.Event.DeliveryID)
	dispatcher := state.Progress
	if dispatcher == nil {
		p.detachWhatsAppProgressDispatcher(cfg.instanceID, request.Event.DeliveryID, true)
		return session.AckDelivery(request, "", "")
	}
	if err := dispatcher.Flush(ctx); err != nil {
		p.reportWhatsAppProgressFailure(ctx, session, cfg.instanceID, err)
		return bridgepkg.DeliveryAck{}, err
	}
	cleanupErr := dispatcher.OnContent(ctx)
	p.detachWhatsAppProgressDispatcher(cfg.instanceID, request.Event.DeliveryID, true)
	if cleanupErr != nil {
		p.recordWhatsAppProgressCleanupError(cfg.instanceID, cleanupErr)
	}
	return session.AckDelivery(request, "", "")
}

func (p *whatsappProvider) reportWhatsAppProgressFailure(
	ctx context.Context,
	session *bridgesdk.Session,
	instanceID string,
	err error,
) {
	classified := bridgesdk.ClassifyError(err)
	_, _, reportErr := session.ReportClassifiedError(ctx, instanceID, classified)
	if reportErr != nil {
		p.setInstanceError(instanceID, reportErr)
		return
	}
	p.setInstanceError(instanceID, err)
}

func (p *whatsappProvider) recordWhatsAppProgressCleanupError(instanceID string, err error) {
	if err == nil {
		return
	}
	wrapped := fmt.Errorf("whatsapp: close progress after content: %w", err)
	p.markers.ReportError("close WhatsApp progress after content", wrapped)
	p.setInstanceError(instanceID, wrapped)
}

func (p *whatsappProvider) closeAllWhatsAppProgressDispatchers() {
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

func (p *whatsappProvider) detachWhatsAppProgressDispatcher(
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

func (s *whatsappProgressSink) Post(
	ctx context.Context,
	_ bridgepkg.DeliveryTarget,
	text string,
) (string, error) {
	remoteID, err := sendWhatsAppDeliveryChunks(
		ctx,
		s.api,
		s.phoneNumberID,
		s.targetUserID,
		text,
	)
	if err != nil {
		return "", err
	}
	s.lastText = text
	return remoteID, nil
}

func (s *whatsappProgressSink) Edit(
	ctx context.Context,
	_ bridgepkg.DeliveryTarget,
	_ string,
	text string,
) error {
	appendText := text
	if prefix := s.lastText + "\n"; s.lastText != "" && strings.HasPrefix(text, prefix) {
		appendText = strings.TrimPrefix(text, prefix)
	}
	if _, err := sendWhatsAppDeliveryChunks(
		ctx,
		s.api,
		s.phoneNumberID,
		s.targetUserID,
		appendText,
	); err != nil {
		if bridgesdk.IsCommittedMutation(err) {
			s.lastText = text
		}
		return fmt.Errorf("whatsapp: append progress update: %w", err)
	}
	s.lastText = text
	return nil
}

func (s *whatsappProgressSink) Typing(
	context.Context,
	bridgepkg.DeliveryTarget,
	bool,
) error {
	return nil
}

func (s *whatsappProgressSink) React(
	context.Context,
	bridgepkg.DeliveryTarget,
	string,
) error {
	return nil
}
