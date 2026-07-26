package loop

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	storepkg "github.com/compozy/agh/internal/store"
)

const (
	defaultChannelResultPollInterval = 100 * time.Millisecond
	defaultChannelResultMessageLimit = 100
	channelResultSignalIntent        = "result"
)

// ChannelResultConversationStore is the durable network conversation read seam used by channel_result harvest.
type ChannelResultConversationStore interface {
	ListConversationMessages(
		ctx context.Context,
		ref storepkg.NetworkConversationRef,
		query storepkg.NetworkConversationMessageQuery,
	) ([]storepkg.NetworkConversationMessage, error)
}

// StoreChannelResultHarvester reads the durable AGH Network conversation store for result messages.
type StoreChannelResultHarvester struct {
	conversations ChannelResultConversationStore
	pollInterval  time.Duration
	limit         int
}

var _ ChannelResultHarvester = (*StoreChannelResultHarvester)(nil)

// NewStoreChannelResultHarvester creates the default durable-store channel_result harvester.
func NewStoreChannelResultHarvester(
	conversations ChannelResultConversationStore,
) (*StoreChannelResultHarvester, error) {
	if conversations == nil {
		return nil, reasonError(
			ReasonCodeActionDependencyMissing,
			ErrActionDependencyMissing,
			map[string]string{actionDependencyMetaKey: "network_conversation_store"},
		)
	}
	return &StoreChannelResultHarvester{
		conversations: conversations,
		pollInterval:  defaultChannelResultPollInterval,
		limit:         defaultChannelResultMessageLimit,
	}, nil
}

// HarvestChannelResult waits within the declared window for the designated result message.
func (h *StoreChannelResultHarvester) HarvestChannelResult(
	ctx context.Context,
	req *ChannelResultHarvestRequest,
) (ChannelResultHarvestResult, error) {
	if ctx == nil {
		return ChannelResultHarvestResult{}, fmt.Errorf("%w: channel_result context is required", ErrValidation)
	}
	if req == nil {
		return ChannelResultHarvestResult{}, fmt.Errorf("%w: channel_result request is required", ErrValidation)
	}
	if h == nil || h.conversations == nil {
		return ChannelResultHarvestResult{}, reasonError(
			ReasonCodeActionDependencyMissing,
			ErrActionDependencyMissing,
			map[string]string{actionDependencyMetaKey: "network_conversation_store"},
		)
	}
	window, err := parseChannelResultWindow(req.Window)
	if err != nil {
		return ChannelResultHarvestResult{}, err
	}
	send, err := channelResultSendMetadata(req.Raw)
	if err != nil {
		return ChannelResultHarvestResult{}, err
	}
	ref, err := send.conversationRef()
	if err != nil {
		return ChannelResultHarvestResult{}, err
	}
	cursor := send.MessageID
	deadline := time.Now().Add(window)
	firstScan := true
	for {
		if !firstScan {
			if err := ctx.Err(); err != nil {
				return ChannelResultHarvestResult{}, err
			}
			if !time.Now().Before(deadline) {
				return ChannelResultHarvestResult{Found: false}, nil
			}
		}
		firstScan = false
		result, nextCursor, err := h.scanChannelResult(ctx, ref, cursor, send, req)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) && time.Now().After(deadline) {
				return ChannelResultHarvestResult{Found: false}, nil
			}
			return ChannelResultHarvestResult{}, err
		}
		if result.Found {
			return result, nil
		}
		if nextCursor != "" && nextCursor != cursor {
			cursor = nextCursor
			continue
		}
		if !time.Now().Before(deadline) {
			return ChannelResultHarvestResult{Found: false}, nil
		}
		if err := waitForChannelResultPoll(ctx, deadline, h.pollInterval); err != nil {
			return ChannelResultHarvestResult{}, err
		}
	}
}

func (h *StoreChannelResultHarvester) scanChannelResult(
	ctx context.Context,
	ref storepkg.NetworkConversationRef,
	cursor string,
	send channelResultSendMeta,
	req *ChannelResultHarvestRequest,
) (ChannelResultHarvestResult, string, error) {
	messages, err := h.conversations.ListConversationMessages(ctx, ref, storepkg.NetworkConversationMessageQuery{
		AfterMessageID: cursor,
		WorkID:         send.WorkID,
		Limit:          h.queryLimit(),
	})
	if err != nil {
		return ChannelResultHarvestResult{}, "", fmt.Errorf("list channel_result conversation messages: %w", err)
	}
	nextCursor := cursor
	for _, message := range messages {
		if strings.TrimSpace(message.MessageID) != "" {
			nextCursor = message.MessageID
		}
		result, matched, err := channelResultFromMessage(message, req)
		if err != nil {
			return ChannelResultHarvestResult{}, "", err
		}
		if matched {
			return result, nextCursor, nil
		}
	}
	return ChannelResultHarvestResult{}, nextCursor, nil
}

func (h *StoreChannelResultHarvester) queryLimit() int {
	if h.limit > 0 {
		return h.limit
	}
	return defaultChannelResultMessageLimit
}

func parseChannelResultWindow(raw string) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, fmt.Errorf("%w: channel_result window is required", ErrValidation)
	}
	window, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("%w: parse channel_result window %q: %w", ErrValidation, trimmed, err)
	}
	if window <= 0 {
		return 0, fmt.Errorf("%w: channel_result window must be positive", ErrValidation)
	}
	return window, nil
}

func waitForChannelResultPoll(ctx context.Context, deadline time.Time, interval time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	wait := interval
	if wait <= 0 {
		wait = defaultChannelResultPollInterval
	}
	if remaining := time.Until(deadline); remaining < wait {
		wait = remaining
	}
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
