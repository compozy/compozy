package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

const (
	slackProgressFallbackReaction = "eyes"
	slackProgressThinkingStatus   = "is thinking..."
)

type slackSetThreadStatusRequest struct {
	ChannelID string `json:"channel_id"`
	ThreadTS  string `json:"thread_ts"`
	Status    string `json:"status"`
}

type slackAddReactionRequest struct {
	Channel   string `json:"channel"`
	Timestamp string `json:"timestamp"`
	Name      string `json:"name"`
}

type slackProgressSink struct {
	api             slackAPI
	bubbleTimestamp string
	reactions       map[string]struct{}
}

var _ bridgesdk.ProgressSink = (*slackProgressSink)(nil)

func (s *slackProgressSink) Post(
	ctx context.Context,
	target bridgepkg.DeliveryTarget,
	text string,
) (string, error) {
	channel, threadTS, err := resolveSlackProgressTarget(target)
	if err != nil {
		return "", err
	}
	sent, err := s.api.PostMessage(ctx, slackPostMessageRequest{
		Channel:  channel,
		ThreadTS: threadTS,
		Text:     formatOutbound(text),
		Mrkdwn:   true,
	})
	if err != nil {
		return "", fmt.Errorf("slack: post progress bubble: %w", err)
	}
	sent, err = validateSlackPostedMutation(sent)
	if err != nil {
		return "", err
	}
	timestamp := sent.TS
	s.bubbleTimestamp = timestamp
	s.reactions = make(map[string]struct{})
	return timestamp, nil
}

func (s *slackProgressSink) Edit(
	ctx context.Context,
	target bridgepkg.DeliveryTarget,
	remoteID string,
	text string,
) error {
	channel, _, err := resolveSlackProgressTarget(target)
	if err != nil {
		return err
	}
	timestamp := strings.TrimSpace(remoteID)
	if timestamp == "" {
		return errors.New("slack: progress edit requires a message timestamp")
	}
	formatted := formatOutbound(text)
	_, err = bridgesdk.RetryDo(ctx, bridgesdk.DefaultRetryConfig(), func(callCtx context.Context) (struct{}, error) {
		if updateErr := s.api.UpdateMessage(callCtx, slackUpdateMessageRequest{
			Channel: channel,
			TS:      timestamp,
			Text:    formatted,
		}); updateErr != nil {
			return struct{}{}, updateErr
		}
		return struct{}{}, nil
	})
	if err != nil {
		return fmt.Errorf("slack: edit progress bubble: %w", err)
	}
	s.bubbleTimestamp = timestamp
	return nil
}

func (s *slackProgressSink) Typing(
	ctx context.Context,
	target bridgepkg.DeliveryTarget,
	active bool,
) error {
	channel, threadTS, err := resolveSlackProgressTarget(target)
	if err != nil {
		return err
	}
	if threadTS == "" {
		return nil
	}
	status := ""
	if active {
		status = slackProgressThinkingStatus
	}
	if err := s.api.SetThreadStatus(ctx, slackSetThreadStatusRequest{
		ChannelID: channel,
		ThreadTS:  threadTS,
		Status:    status,
	}); err != nil {
		return fmt.Errorf("slack: set progress thread status: %w", err)
	}
	return nil
}

func (s *slackProgressSink) React(
	ctx context.Context,
	target bridgepkg.DeliveryTarget,
	reaction string,
) error {
	channel, _, err := resolveSlackProgressTarget(target)
	if err != nil {
		return err
	}
	timestamp := strings.TrimSpace(s.bubbleTimestamp)
	if timestamp == "" {
		return errors.New("slack: progress reaction requires a posted bubble")
	}
	name := slackReactionName(reaction)
	if _, applied := s.reactions[name]; applied {
		return nil
	}
	if err := s.api.AddReaction(ctx, slackAddReactionRequest{
		Channel:   channel,
		Timestamp: timestamp,
		Name:      name,
	}); err != nil {
		return fmt.Errorf("slack: add progress reaction: %w", err)
	}
	if s.reactions == nil {
		s.reactions = make(map[string]struct{})
	}
	s.reactions[name] = struct{}{}
	return nil
}

func resolveSlackProgressTarget(target bridgepkg.DeliveryTarget) (string, string, error) {
	channel := firstNonEmpty(target.PeerID, target.GroupID)
	if channel == "" {
		return "", "", errors.New("slack: progress target requires peer_id or group_id")
	}
	return channel, strings.TrimSpace(target.ThreadID), nil
}

func slackReactionName(reaction string) string {
	switch strings.TrimSpace(reaction) {
	case "\u2705":
		return "white_check_mark"
	case "\u274c":
		return "x"
	default:
		return slackProgressFallbackReaction
	}
}

func (c *slackBotClient) SetThreadStatus(ctx context.Context, request slackSetThreadStatusRequest) error {
	var result json.RawMessage
	return c.callMutation(ctx, "assistant.threads.setStatus", request, &result)
}

func (c *slackBotClient) AddReaction(ctx context.Context, request slackAddReactionRequest) error {
	var result json.RawMessage
	return c.callMutation(ctx, "reactions.add", request, &result)
}

func (p *slackProvider) handleBridgesProgress(
	ctx context.Context,
	session *bridgesdk.Session,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	cfg, err := p.waitForInstanceConfig(strings.TrimSpace(request.Event.BridgeInstanceID), 500*time.Millisecond)
	if err != nil {
		p.setLastError(err)
		return bridgepkg.DeliveryAck{}, err
	}
	progressConfig := slackResolvedProgressConfig(cfg)
	if progressConfig.ToolProgress == bridgepkg.ProgressModeOff {
		dispatcher := p.detachProgressDispatcher(
			cfg.instanceID,
			request.Event.DeliveryID,
			request.Event.IsLifecycleOnlyFinal(),
		)
		closeProgressDispatcher(dispatcher)
		return session.AckDelivery(request, "", "")
	}
	if request.Event.IsLifecycleOnlyFinal() {
		return p.finishProgressOnlyDelivery(ctx, session, cfg.instanceID, request)
	}
	if request.Event.Progress == nil {
		return bridgepkg.DeliveryAck{}, errors.New("slack: progress delivery requires a progress payload")
	}

	state, _ := p.deliveries.UpdateOrCreate(
		deliveryStateKey(cfg.instanceID, request.Event.DeliveryID),
		func(state deliveryState, _ bool) deliveryState {
			if state.Progress != nil {
				return state
			}
			api := p.apiFactory(&cfg)
			accumulator := bridgesdk.NewProgressAccumulator(bridgesdk.ProgressConfig{
				ProgressConfig: progressConfig,
				Target:         request.Event.DeliveryTarget,
				MaxMessageLen:  slackMaxMessageLen,
				Len: func(value string) int {
					return bridgesdk.UTF16Len(formatOutbound(value))
				},
			}, &slackProgressSink{api: api}, p.now)
			state.Progress = bridgesdk.NewProgressDispatcher(
				accumulator,
				bridgesdk.WithProgressAsyncErrorHandler(func(asyncErr error) {
					p.setLastError(fmt.Errorf("slack: async progress flush: %w", asyncErr))
				}),
			)
			return state
		},
	)
	if err := state.Progress.OnProgress(ctx, *request.Event.Progress); err != nil {
		p.reportProgressError(ctx, session, cfg.instanceID, err)
		return bridgepkg.DeliveryAck{}, err
	}
	p.clearLastError()
	if err := p.lifecycle.Host().ReportReadyIfNeeded(ctx, session, cfg.instanceID); err != nil {
		p.setLastError(err)
	}
	return session.AckDelivery(request, "", "")
}

func slackResolvedProgressConfig(cfg resolvedInstanceConfig) bridgepkg.ProgressConfig {
	if cfg.managed == nil {
		return bridgepkg.ResolveProgressConfig(nil, providerSlackKey)
	}
	return bridgepkg.ResolveProgressConfig(&cfg.managed.Instance, providerSlackKey)
}

func (p *slackProvider) finishProgressOnlyDelivery(
	ctx context.Context,
	session *bridgesdk.Session,
	instanceID string,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	state := p.deliveryState(instanceID, request.Event.DeliveryID)
	if state.Progress != nil {
		if err := state.Progress.Flush(ctx); err != nil {
			p.reportProgressError(ctx, session, instanceID, err)
			return bridgepkg.DeliveryAck{}, err
		}
		if err := state.Progress.OnContent(ctx); err != nil {
			p.recordProgressCleanupError("close progress-only delivery", err)
		}
	}
	dispatcher := p.detachProgressDispatcher(instanceID, request.Event.DeliveryID, true)
	closeProgressDispatcher(dispatcher)
	return session.AckDelivery(request, "", "")
}

func (p *slackProvider) reportProgressError(
	ctx context.Context,
	session *bridgesdk.Session,
	instanceID string,
	err error,
) {
	p.setLastError(err)
	if session == nil {
		return
	}
	if _, _, reportErr := session.ReportClassifiedError(
		ctx,
		instanceID,
		bridgesdk.ClassifyError(err),
	); reportErr != nil {
		p.setLastError(reportErr)
	}
}

func (p *slackProvider) recordProgressCleanupError(action string, err error) {
	if err == nil {
		return
	}
	p.markers.ReportError(action, err)
	p.setLastError(err)
}

func (p *slackProvider) executeTextDeliveryWithProgress(
	ctx context.Context,
	cfg *resolvedInstanceConfig,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	state := p.deliveryState(cfg.instanceID, request.Event.DeliveryID)
	if state.Progress != nil {
		if err := state.Progress.Flush(ctx); err != nil {
			p.recordProgressCleanupError("flush progress before text delivery", err)
			if !bridgesdk.ShouldContinueTextDeliveryAfterProgress(err) {
				return bridgepkg.DeliveryAck{}, state, err
			}
		}
	}
	return executeDelivery(ctx, p.apiFactory(cfg), request, state)
}

func (p *slackProvider) completeTextDeliveryProgress(
	ctx context.Context,
	instanceID string,
	request bridgepkg.DeliveryRequest,
	state deliveryState,
) error {
	var cleanupErr error
	if state.Progress != nil {
		cleanupErr = state.Progress.OnContent(ctx)
	}
	dispatcher := p.storeDeliveryState(instanceID, request.Event.DeliveryID, request.Event, state)
	closeProgressDispatcher(dispatcher)
	return cleanupErr
}

func (p *slackProvider) closeAllProgressDispatchers() {
	dispatchers := p.takeAllProgressDispatchers()
	for _, dispatcher := range dispatchers {
		closeProgressDispatcher(dispatcher)
	}
}

func closeProgressDispatcher(dispatcher *bridgesdk.ProgressDispatcher) {
	if dispatcher != nil {
		dispatcher.Close()
	}
}
