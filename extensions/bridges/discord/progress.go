package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

type discordProgressSink struct {
	api discordAPI

	mu       sync.Mutex
	remoteID string
}

var _ bridgesdk.ProgressSink = (*discordProgressSink)(nil)

func newDiscordProgressSink(api discordAPI) *discordProgressSink {
	return &discordProgressSink{api: api}
}

func (s *discordProgressSink) Post(
	ctx context.Context,
	target bridgepkg.DeliveryTarget,
	text string,
) (string, error) {
	if s == nil || s.api == nil {
		return "", errors.New("discord: progress api is required")
	}
	channelID, err := resolveDiscordProgressChannelID(target)
	if err != nil {
		return "", err
	}
	sent, err := s.api.PostMessage(ctx, discordPostMessageRequest{
		ChannelID: channelID,
		Content:   text,
	})
	if err != nil {
		return "", fmt.Errorf("discord: post progress message: %w", err)
	}
	remoteID, err := discordPostedRemoteID(channelID, sent)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.remoteID = remoteID
	s.mu.Unlock()
	return remoteID, nil
}

func (s *discordProgressSink) Edit(
	ctx context.Context,
	_ bridgepkg.DeliveryTarget,
	remoteID string,
	text string,
) error {
	if s == nil || s.api == nil {
		return errors.New("discord: progress api is required")
	}
	channelID, messageID, err := decodeRemoteMessageID(remoteID)
	if err != nil {
		return err
	}
	_, err = bridgesdk.RetryDo(ctx, bridgesdk.DefaultRetryConfig(), func(callCtx context.Context) (struct{}, error) {
		updateErr := s.api.UpdateMessage(callCtx, discordUpdateMessageRequest{
			ChannelID: channelID,
			MessageID: messageID,
			Content:   text,
		})
		return struct{}{}, updateErr
	})
	if err != nil {
		return fmt.Errorf("discord: edit progress message: %w", err)
	}
	s.mu.Lock()
	s.remoteID = remoteID
	s.mu.Unlock()
	return nil
}

func (s *discordProgressSink) Typing(
	ctx context.Context,
	target bridgepkg.DeliveryTarget,
	active bool,
) error {
	api, err := s.progressAPI()
	if err != nil {
		return err
	}
	channelID, err := resolveDiscordProgressChannelID(target)
	if err != nil {
		return err
	}
	if err := api.SetTyping(ctx, discordTypingRequest{ChannelID: channelID, Active: active}); err != nil {
		return fmt.Errorf("discord: set progress typing: %w", err)
	}
	return nil
}

func (s *discordProgressSink) React(
	ctx context.Context,
	_ bridgepkg.DeliveryTarget,
	reaction string,
) error {
	api, err := s.progressAPI()
	if err != nil {
		return err
	}
	s.mu.Lock()
	remoteID := s.remoteID
	s.mu.Unlock()
	if strings.TrimSpace(remoteID) == "" {
		return errors.New("discord: progress reaction requires a posted message")
	}
	channelID, messageID, err := decodeRemoteMessageID(remoteID)
	if err != nil {
		return err
	}
	if err := api.AddReaction(ctx, discordReactionRequest{
		ChannelID: channelID,
		MessageID: messageID,
		Reaction:  reaction,
	}); err != nil {
		return fmt.Errorf("discord: add progress reaction: %w", err)
	}
	return nil
}

func (s *discordProgressSink) progressAPI() (discordProgressAPI, error) {
	if s == nil || s.api == nil {
		return nil, errors.New("discord: progress api is required")
	}
	api, ok := s.api.(discordProgressAPI)
	if !ok {
		return nil, errors.New("discord: progress affordance api is required")
	}
	return api, nil
}

func resolveDiscordProgressChannelID(target bridgepkg.DeliveryTarget) (string, error) {
	return resolveDiscordDeliveryChannelID(bridgepkg.DeliveryEvent{DeliveryTarget: target})
}

func (p *discordProvider) handleBridgesProgress(
	ctx context.Context,
	session *bridgesdk.Session,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	if err := request.Validate(); err != nil {
		return bridgepkg.DeliveryAck{}, err
	}
	instanceID := strings.TrimSpace(request.Event.BridgeInstanceID)
	cfg, err := p.waitForInstanceConfig(instanceID, 500*time.Millisecond)
	if err != nil {
		p.setLastError(err)
		return bridgepkg.DeliveryAck{}, err
	}
	config := bridgepkg.ResolveProgressConfig(&cfg.managed.Instance, providerDiscordKey)
	if config.ToolProgress == bridgepkg.ProgressModeOff {
		state := p.deliveryState(instanceID, request.Event.DeliveryID)
		var dispatcher *bridgesdk.ProgressDispatcher
		if request.Event.IsLifecycleOnlyFinal() {
			dispatcher = p.removeDiscordProgressDelivery(instanceID, request.Event.DeliveryID, state.Progress)
		} else {
			dispatcher = p.detachDiscordProgressDispatcher(instanceID, request.Event.DeliveryID, nil)
		}
		if dispatcher != nil {
			dispatcher.Close()
		}
		return session.AckDelivery(request, "", "")
	}
	if request.Event.IsLifecycleOnlyFinal() {
		return p.finishDiscordProgress(ctx, session, request)
	}
	if request.Event.Progress == nil {
		return bridgepkg.DeliveryAck{}, errors.New("discord: progress delivery requires progress payload")
	}

	state, _ := p.deliveries.UpdateOrCreate(
		deliveryStateKey(instanceID, request.Event.DeliveryID),
		func(state deliveryState, _ bool) deliveryState {
			if state.Progress == nil {
				state.Progress = p.newDiscordProgressDispatcher(
					config,
					request.Event.DeliveryTarget,
					p.apiFactory(cfg),
				)
			}
			return state
		},
	)
	if err := state.Progress.OnProgress(ctx, *request.Event.Progress); err != nil {
		p.reportDiscordDeliveryError(ctx, session, instanceID, err)
		return bridgepkg.DeliveryAck{}, err
	}
	p.clearLastError()
	return session.AckDelivery(request, "", "")
}

func (p *discordProvider) finishDiscordProgress(
	ctx context.Context,
	session *bridgesdk.Session,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	instanceID := strings.TrimSpace(request.Event.BridgeInstanceID)
	state := p.deliveryState(instanceID, request.Event.DeliveryID)
	if state.Progress == nil {
		p.removeDiscordProgressDelivery(instanceID, request.Event.DeliveryID, nil)
		return session.AckDelivery(request, "", "")
	}
	if err := state.Progress.Flush(ctx); err != nil {
		p.reportDiscordDeliveryError(ctx, session, instanceID, err)
		return bridgepkg.DeliveryAck{}, err
	}
	cleanupErr := state.Progress.OnContent(ctx)
	dispatcher := p.removeDiscordProgressDelivery(instanceID, request.Event.DeliveryID, state.Progress)
	if dispatcher != nil {
		dispatcher.Close()
	}
	if cleanupErr != nil {
		p.reportDiscordProgressError("clean up lifecycle-only progress", cleanupErr)
	} else {
		p.clearLastError()
	}
	return session.AckDelivery(request, "", "")
}

func (p *discordProvider) newDiscordProgressDispatcher(
	config bridgepkg.ProgressConfig,
	target bridgepkg.DeliveryTarget,
	api discordAPI,
) *bridgesdk.ProgressDispatcher {
	accumulator := bridgesdk.NewProgressAccumulator(bridgesdk.ProgressConfig{
		ProgressConfig: config,
		Target:         target,
		MaxMessageLen:  discordMaxMessageLen,
		Len:            utf8.RuneCountInString,
	}, newDiscordProgressSink(api), p.now)
	options := append([]bridgesdk.ProgressDispatcherOption(nil), p.progressDispatcherOptions...)
	options = append(options, bridgesdk.WithProgressAsyncErrorHandler(func(err error) {
		p.reportDiscordProgressError("flush scheduled progress", err)
	}))
	return bridgesdk.NewProgressDispatcher(accumulator, options...)
}

func (p *discordProvider) detachDiscordProgressDispatcher(
	instanceID string,
	deliveryID string,
	expected *bridgesdk.ProgressDispatcher,
) *bridgesdk.ProgressDispatcher {
	key := deliveryStateKey(instanceID, deliveryID)
	state, ok := p.deliveries.Load(key)
	if !ok {
		return nil
	}
	if expected != nil && state.Progress != expected {
		return nil
	}
	dispatcher := state.Progress
	p.deliveries.Update(key, func(current deliveryState) deliveryState {
		current.Progress = nil
		return current
	})
	return dispatcher
}

func (p *discordProvider) removeDiscordProgressDelivery(
	instanceID string,
	deliveryID string,
	expected *bridgesdk.ProgressDispatcher,
) *bridgesdk.ProgressDispatcher {
	key := deliveryStateKey(instanceID, deliveryID)
	state, ok := p.deliveries.Load(key)
	if !ok || state.Progress != expected {
		return nil
	}
	p.deliveries.Delete(key)
	return state.Progress
}

func (p *discordProvider) takeDiscordProgressDispatchers() []*bridgesdk.ProgressDispatcher {
	unique := make(map[*bridgesdk.ProgressDispatcher]struct{})
	p.deliveries.TransformAll(func(_ string, state deliveryState) deliveryState {
		if state.Progress != nil {
			unique[state.Progress] = struct{}{}
			state.Progress = nil
		}
		return state
	})
	dispatchers := make([]*bridgesdk.ProgressDispatcher, 0, len(unique))
	for dispatcher := range unique {
		dispatchers = append(dispatchers, dispatcher)
	}
	return dispatchers
}

func closeDiscordProgressDispatchers(dispatchers []*bridgesdk.ProgressDispatcher) {
	for _, dispatcher := range dispatchers {
		dispatcher.Close()
	}
}

func (p *discordProvider) reportDiscordDeliveryError(
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

func (p *discordProvider) reportDiscordProgressError(action string, err error) {
	if err == nil {
		return
	}
	p.markers.ReportError(action, err)
	p.setLastError(err)
}
