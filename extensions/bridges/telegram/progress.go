package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

const telegramProgressReactionTypeEmoji = "emoji"

type telegramSendChatActionRequest struct {
	ChatID          string `json:"chat_id"`
	Action          string `json:"action"`
	MessageThreadID int64  `json:"message_thread_id,omitempty"`
}

type telegramReactionType struct {
	Type  string `json:"type"`
	Emoji string `json:"emoji"`
}

type telegramSetMessageReactionRequest struct {
	ChatID    string                 `json:"chat_id"`
	MessageID int64                  `json:"message_id"`
	Reaction  []telegramReactionType `json:"reaction"`
}

type telegramProgressSink struct {
	api             telegramAPI
	bubbleMessageID int64
}

var _ bridgesdk.ProgressSink = (*telegramProgressSink)(nil)

func (p *telegramProvider) deliveryState(instanceID string, deliveryID string) deliveryState {
	state, _ := p.deliveries.Load(deliveryStateKey(instanceID, deliveryID))
	return state
}

func (p *telegramProvider) storeDeliveryState(
	instanceID string,
	deliveryID string,
	event bridgepkg.DeliveryEvent,
	state deliveryState,
) *bridgesdk.ProgressDispatcher {
	key := deliveryStateKey(instanceID, deliveryID)
	if isTerminalTelegramDeliveryEvent(event) {
		p.deliveries.Delete(key)
		return state.Progress
	}
	p.deliveries.Store(key, state)
	return nil
}

func (p *telegramProvider) storeDeliveryRetryState(
	instanceID string,
	deliveryID string,
	state deliveryState,
) {
	if !state.Chunks.Active() {
		return
	}
	p.deliveries.Store(deliveryStateKey(instanceID, deliveryID), state)
}

func (p *telegramProvider) detachProgressDispatcher(
	instanceID string,
	deliveryID string,
	removeState bool,
) *bridgesdk.ProgressDispatcher {
	key := deliveryStateKey(instanceID, deliveryID)
	state, ok := p.deliveries.Load(key)
	if !ok {
		return nil
	}
	dispatcher := state.Progress
	if removeState {
		p.deliveries.Delete(key)
	} else {
		p.deliveries.Update(key, func(current deliveryState) deliveryState {
			current.Progress = nil
			return current
		})
	}
	return dispatcher
}

func (p *telegramProvider) takeAllProgressDispatchers() []*bridgesdk.ProgressDispatcher {
	dispatchers := make([]*bridgesdk.ProgressDispatcher, 0)
	p.deliveries.TransformAll(func(_ string, state deliveryState) deliveryState {
		if state.Progress == nil {
			return state
		}
		dispatchers = append(dispatchers, state.Progress)
		state.Progress = nil
		return state
	})
	return dispatchers
}

func (s *telegramProgressSink) Post(
	ctx context.Context,
	target bridgepkg.DeliveryTarget,
	text string,
) (string, error) {
	chatID, messageThreadID, err := resolveTelegramProgressTarget(target)
	if err != nil {
		return "", err
	}
	sent, err := sendTelegramOutbound(
		ctx,
		s.api,
		telegramSendMessageRequest{ChatID: chatID, MessageThreadID: messageThreadID},
		formatOutbound(text),
	)
	if err != nil {
		return "", fmt.Errorf("telegram: post progress bubble: %w", err)
	}
	s.bubbleMessageID = sent.MessageID
	return strconv.FormatInt(sent.MessageID, 10), nil
}

func (s *telegramProgressSink) Edit(
	ctx context.Context,
	target bridgepkg.DeliveryTarget,
	remoteID string,
	text string,
) error {
	chatID, _, err := resolveTelegramProgressTarget(target)
	if err != nil {
		return err
	}
	messageID, err := strconv.ParseInt(strings.TrimSpace(remoteID), 10, 64)
	if err != nil || messageID <= 0 {
		return fmt.Errorf("telegram: invalid progress message id %q", remoteID)
	}
	outbound := formatOutbound(text)
	_, err = bridgesdk.RetryDo(ctx, bridgesdk.DefaultRetryConfig(), func(callCtx context.Context) (struct{}, error) {
		if editErr := editTelegramOutbound(callCtx, s.api, telegramEditMessageTextRequest{
			ChatID:    chatID,
			MessageID: messageID,
		}, outbound); editErr != nil {
			return struct{}{}, editErr
		}
		return struct{}{}, nil
	})
	if err != nil {
		return fmt.Errorf("telegram: edit progress bubble: %w", err)
	}
	s.bubbleMessageID = messageID
	return nil
}

func (s *telegramProgressSink) Typing(
	ctx context.Context,
	target bridgepkg.DeliveryTarget,
	active bool,
) error {
	if !active {
		return nil
	}
	chatID, messageThreadID, err := resolveTelegramProgressTarget(target)
	if err != nil {
		return err
	}
	if err := s.api.SendChatAction(ctx, telegramSendChatActionRequest{
		ChatID:          chatID,
		Action:          "typing",
		MessageThreadID: messageThreadID,
	}); err != nil {
		return fmt.Errorf("telegram: send progress chat action: %w", err)
	}
	return nil
}

func (s *telegramProgressSink) React(
	ctx context.Context,
	target bridgepkg.DeliveryTarget,
	reaction string,
) error {
	chatID, _, err := resolveTelegramProgressTarget(target)
	if err != nil {
		return err
	}
	if s.bubbleMessageID <= 0 {
		return errors.New("telegram: progress reaction requires a posted bubble")
	}
	if err := s.api.SetMessageReaction(ctx, telegramSetMessageReactionRequest{
		ChatID:    chatID,
		MessageID: s.bubbleMessageID,
		Reaction: []telegramReactionType{{
			Type:  telegramProgressReactionTypeEmoji,
			Emoji: strings.TrimSpace(reaction),
		}},
	}); err != nil {
		return fmt.Errorf("telegram: set progress reaction: %w", err)
	}
	return nil
}

func resolveTelegramProgressTarget(target bridgepkg.DeliveryTarget) (string, int64, error) {
	chatID := firstNonEmpty(target.PeerID, target.GroupID)
	if chatID == "" {
		return "", 0, errors.New("telegram: progress target requires peer_id or group_id")
	}
	messageThreadID, err := resolveTelegramThreadID(target.ThreadID, chatID)
	if err != nil {
		return "", 0, err
	}
	return chatID, messageThreadID, nil
}

func (c *telegramBotClient) SendChatAction(
	ctx context.Context,
	request telegramSendChatActionRequest,
) error {
	var result bool
	return c.callMutation(ctx, "sendChatAction", request, &result)
}

func (c *telegramBotClient) SetMessageReaction(
	ctx context.Context,
	request telegramSetMessageReactionRequest,
) error {
	var result bool
	return c.callMutation(ctx, "setMessageReaction", request, &result)
}

func (p *telegramProvider) handleBridgesProgress(
	ctx context.Context,
	session *bridgesdk.Session,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	cfg, err := p.waitForInstanceConfig(strings.TrimSpace(request.Event.BridgeInstanceID), 500*time.Millisecond)
	if err != nil {
		p.setLastError(err)
		return bridgepkg.DeliveryAck{}, err
	}
	progressConfig := telegramResolvedProgressConfig(cfg)
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
		return bridgepkg.DeliveryAck{}, errors.New("telegram: progress delivery requires a progress payload")
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
				MaxMessageLen:  telegramMaxMessageLen,
				Len: func(value string) int {
					return bridgesdk.UTF16Len(formatOutbound(value).Text)
				},
			}, &telegramProgressSink{api: api}, p.now)
			state.Progress = bridgesdk.NewProgressDispatcher(
				accumulator,
				bridgesdk.WithProgressAsyncErrorHandler(func(asyncErr error) {
					p.setLastError(fmt.Errorf("telegram: async progress flush: %w", asyncErr))
				}),
			)
			return state
		},
	)
	if err := state.Progress.OnProgress(ctx, *request.Event.Progress); err != nil {
		p.reportProgressError(ctx, session, cfg.instanceID, err)
		return bridgepkg.DeliveryAck{}, err
	}
	if err := p.lifecycle.Host().ReportReadyIfNeeded(ctx, session, cfg.instanceID); err != nil {
		p.setLastError(err)
	} else {
		p.clearLastError()
	}
	return session.AckDelivery(request, "", "")
}

func telegramResolvedProgressConfig(cfg resolvedInstanceConfig) bridgepkg.ProgressConfig {
	if cfg.managed == nil {
		return bridgepkg.ResolveProgressConfig(nil, providerTelegramKey)
	}
	return bridgepkg.ResolveProgressConfig(&cfg.managed.Instance, providerTelegramKey)
}

func (p *telegramProvider) finishProgressOnlyDelivery(
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

func (p *telegramProvider) reportProgressError(
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

func (p *telegramProvider) recordProgressCleanupError(action string, err error) {
	if err == nil {
		return
	}
	p.markers.ReportError(action, err)
	p.setLastError(err)
}

func (p *telegramProvider) executeTextDeliveryWithProgress(
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

func (p *telegramProvider) completeTextDeliveryProgress(
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

func (p *telegramProvider) closeAllProgressDispatchers() {
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
