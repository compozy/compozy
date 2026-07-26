package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

type gchatProgressSink struct {
	api    gchatAPI
	target gchatResolvedTarget
}

var _ bridgesdk.ProgressSink = (*gchatProgressSink)(nil)

func (p *gchatProvider) handleBridgesProgress(
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
	if strings.TrimSpace(cfg.managed.Instance.ID) == "" {
		return bridgepkg.DeliveryAck{}, errors.New("gchat: managed bridge instance is required for progress")
	}
	progressConfig := bridgepkg.ResolveProgressConfig(&cfg.managed.Instance, providerGchatKey)
	if progressConfig.ToolProgress == bridgepkg.ProgressModeOff {
		p.detachGChatProgressDispatcher(
			cfg.instanceID,
			request.Event.DeliveryID,
			request.Event.IsLifecycleOnlyFinal(),
		)
		return session.AckDelivery(request, "", "")
	}
	if cfg.configError != nil {
		p.setLastError(cfg.configError)
		return bridgepkg.DeliveryAck{}, cfg.configError
	}
	if request.Event.IsLifecycleOnlyFinal() {
		return p.finishGChatProgress(ctx, session, &cfg, request)
	}
	if request.Event.Progress == nil {
		return bridgepkg.DeliveryAck{}, errors.New("gchat: progress payload is required")
	}

	var resolveErr error
	state, _ := p.deliveries.UpdateOrCreate(
		deliveryStateKey(cfg.instanceID, request.Event.DeliveryID),
		func(state deliveryState, _ bool) deliveryState {
			if state.Progress != nil {
				return state
			}
			target, err := resolveGChatDeliveryTarget(request.Event)
			if err != nil {
				resolveErr = err
				return state
			}
			state.Progress = p.newGChatProgressDispatcher(&cfg, request.Event, progressConfig, target)
			return state
		},
	)
	if resolveErr != nil {
		wrapped := fmt.Errorf("gchat: resolve progress target: %w", resolveErr)
		p.reportGChatProgressFailure(ctx, session, cfg.instanceID, wrapped)
		return bridgepkg.DeliveryAck{}, wrapped
	}
	if err := state.Progress.OnProgress(ctx, *request.Event.Progress); err != nil {
		p.reportGChatProgressFailure(ctx, session, cfg.instanceID, err)
		return bridgepkg.DeliveryAck{}, err
	}
	return session.AckDelivery(request, "", "")
}

func (p *gchatProvider) newGChatProgressDispatcher(
	cfg *resolvedInstanceConfig,
	event bridgepkg.DeliveryEvent,
	progressConfig bridgepkg.ProgressConfig,
	target gchatResolvedTarget,
) *bridgesdk.ProgressDispatcher {
	sink := &gchatProgressSink{api: p.apiFactory(cfg), target: target}
	accumulator := bridgesdk.NewProgressAccumulator(bridgesdk.ProgressConfig{
		ProgressConfig: progressConfig,
		Target:         event.DeliveryTarget,
		MaxMessageLen:  gchatMaxMessageBytes,
	}, sink, p.now)
	return bridgesdk.NewProgressDispatcher(
		accumulator,
		bridgesdk.WithProgressAsyncErrorHandler(func(err error) {
			wrapped := fmt.Errorf("gchat: flush scheduled progress: %w", err)
			p.markers.ReportError("flush scheduled Google Chat progress", wrapped)
			p.setLastError(wrapped)
		}),
	)
}

func (p *gchatProvider) finishGChatProgress(
	ctx context.Context,
	session *bridgesdk.Session,
	cfg *resolvedInstanceConfig,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	state := p.deliveryState(cfg.instanceID, request.Event.DeliveryID)
	dispatcher := state.Progress
	if dispatcher == nil {
		p.detachGChatProgressDispatcher(cfg.instanceID, request.Event.DeliveryID, true)
		return session.AckDelivery(request, "", "")
	}
	if err := dispatcher.Flush(ctx); err != nil {
		p.reportGChatProgressFailure(ctx, session, cfg.instanceID, err)
		return bridgepkg.DeliveryAck{}, err
	}
	cleanupErr := dispatcher.OnContent(ctx)
	p.detachGChatProgressDispatcher(cfg.instanceID, request.Event.DeliveryID, true)
	if cleanupErr != nil {
		p.recordGChatProgressCleanupError(cleanupErr)
	}
	return session.AckDelivery(request, "", "")
}

func (p *gchatProvider) reportGChatProgressFailure(
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

func (p *gchatProvider) recordGChatProgressCleanupError(err error) {
	if err == nil {
		return
	}
	wrapped := fmt.Errorf("gchat: close progress after content: %w", err)
	p.markers.ReportError("close Google Chat progress after content", wrapped)
	p.setLastError(wrapped)
}

func (p *gchatProvider) closeAllGChatProgressDispatchers() {
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

func (p *gchatProvider) detachGChatProgressDispatcher(
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

func (s *gchatProgressSink) Post(
	ctx context.Context,
	_ bridgepkg.DeliveryTarget,
	text string,
) (string, error) {
	return postGChatChunks(ctx, s.api, s.target, []string{text})
}

func (s *gchatProgressSink) Edit(
	ctx context.Context,
	_ bridgepkg.DeliveryTarget,
	remoteID string,
	text string,
) error {
	_, err := bridgesdk.RetryDo(
		ctx,
		bridgesdk.DefaultRetryConfig(),
		func(retryCtx context.Context) (*gchatSentMessage, error) {
			return s.api.UpdateMessage(retryCtx, gchatUpdateMessageRequest{
				MessageName: remoteID,
				Text:        text,
			})
		},
	)
	if err != nil {
		return fmt.Errorf("gchat: update progress message: %w", err)
	}
	return nil
}

func (s *gchatProgressSink) Typing(
	context.Context,
	bridgepkg.DeliveryTarget,
	bool,
) error {
	return nil
}

func (s *gchatProgressSink) React(
	context.Context,
	bridgepkg.DeliveryTarget,
	string,
) error {
	return nil
}
