package loop

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/loop/dsl"
	watchpkg "github.com/compozy/agh/internal/loop/watch"
	"github.com/compozy/agh/internal/task"
)

func recoverWatchEventsState(
	ctx context.Context,
	run Run,
	output GenerationOutput,
	node dsl.Node,
	contracts map[hooks.HookEvent]WatchEventsContract,
	runtime coordinatorWatchEventsRuntime,
) (watchpkg.EventsPendingState, *task.CoordinatorTerminal, error) {
	if state, ok, err := watchpkg.EventsPendingFromOutputRef(output.OutputRef); err != nil {
		return watchpkg.EventsPendingState{}, nil, fmt.Errorf(
			"loop: decode watch-events pending state for node %q: %w",
			node.ID,
			err,
		)
	} else if ok {
		if err := validateWatchEventsPendingState(state, contracts); err != nil {
			return watchpkg.EventsPendingState{}, watchEventsBlockedTerminal(watchEventsSpecInvalidReason), nil
		}
		return state, nil, nil
	}
	subscriptions, err := watchEventsSubscriptionsFromNode(node, contracts)
	if err != nil {
		return watchpkg.EventsPendingState{}, watchEventsBlockedTerminal(watchEventsSpecInvalidReason), nil
	}
	streams, kinds, err := watchEventsInitialStreamsAndKinds(subscriptions, run.Inputs, contracts)
	if err != nil {
		return watchpkg.EventsPendingState{}, watchEventsBlockedTerminal(watchEventsSpecInvalidReason), nil
	}
	cursors, err := runtime.cursorReader.ReadCursors(ctx, WatchEventsQuery{
		WorkspaceID: string(run.WorkspaceID),
		Streams:     streams,
		Kinds:       kinds,
		Limit:       LoopMaxFanoutWidth,
	})
	if err != nil {
		return watchpkg.EventsPendingState{}, nil, fmt.Errorf(
			"loop: read watch-events cursors for node %s: %w",
			node.ID,
			err,
		)
	}
	return watchpkg.EventsPendingState{Subscriptions: subscriptions, Cursors: cursors}, nil, nil
}

func watchEventsSubscriptionsFromNode(
	node dsl.Node,
	contracts map[hooks.HookEvent]WatchEventsContract,
) ([]watchpkg.EventSubscriptionRef, error) {
	if len(node.Events) == 0 {
		return nil, fmt.Errorf("%w: watch-events node %q requires subscriptions", ErrValidation, node.ID)
	}
	subscriptions := make([]watchpkg.EventSubscriptionRef, 0, len(node.Events))
	for _, subscription := range node.Events {
		kind := hooks.HookEvent(strings.TrimSpace(subscription.Kind))
		if _, ok := contracts[kind]; !ok {
			return nil, fmt.Errorf("%w: watch-events kind is unsupported: %q", ErrValidation, subscription.Kind)
		}
		subscriptions = append(subscriptions, watchpkg.EventSubscriptionRef{
			Kind:   string(kind),
			Filter: strings.TrimSpace(subscription.Filter),
		})
	}
	return subscriptions, nil
}

func validateWatchEventsPendingState(
	state watchpkg.EventsPendingState,
	contracts map[hooks.HookEvent]WatchEventsContract,
) error {
	if len(state.Subscriptions) == 0 {
		return fmt.Errorf("%w: watch-events pending subscriptions are required", ErrValidation)
	}
	kinds, err := watchEventsLedgerKinds(state.Subscriptions, contracts)
	if err != nil {
		return err
	}
	for stream, cursor := range state.Cursors {
		if strings.TrimSpace(stream) == "" {
			return fmt.Errorf("%w: watch-events cursor stream is required", ErrValidation)
		}
		if cursor < 0 {
			return fmt.Errorf("%w: watch-events cursor for %q must be non-negative", ErrValidation, stream)
		}
	}
	for _, kind := range kinds {
		if !watchEventsPendingStateHasCursorForKind(state, kind, contracts) {
			return fmt.Errorf("%w: watch-events cursor for kind %q is required", ErrValidation, kind)
		}
	}
	return nil
}

func watchEventsInitialStreamsAndKinds(
	subscriptions []watchpkg.EventSubscriptionRef,
	inputs map[string]any,
	contracts map[hooks.HookEvent]WatchEventsContract,
) (map[string]int64, []string, error) {
	streams, err := watchEventsInitialStreams(subscriptions, inputs, contracts)
	if err != nil {
		return nil, nil, err
	}
	kinds, err := watchEventsLedgerKinds(subscriptions, contracts)
	if err != nil {
		return nil, nil, err
	}
	return streams, kinds, nil
}

func watchEventsInitialStreams(
	subscriptions []watchpkg.EventSubscriptionRef,
	inputs map[string]any,
	contracts map[hooks.HookEvent]WatchEventsContract,
) (map[string]int64, error) {
	streams := map[string]int64{}
	for _, subscription := range subscriptions {
		contract, ok := contracts[hooks.HookEvent(strings.TrimSpace(subscription.Kind))]
		if !ok {
			return nil, fmt.Errorf("%w: watch-events kind is unsupported: %q", ErrValidation, subscription.Kind)
		}
		streamKeys, err := watchEventsStreamsForSubscription(subscription, contract, inputs)
		if err != nil {
			return nil, err
		}
		for _, stream := range streamKeys {
			streams[stream] = 0
		}
	}
	return streams, nil
}

func watchEventsStreamsForSubscription(
	subscription watchpkg.EventSubscriptionRef,
	contract WatchEventsContract,
	inputs map[string]any,
) ([]string, error) {
	if contract.Stream != watchEventsSessionStream {
		return []string{contract.Stream}, nil
	}
	sessionIDs, ok, err := watchEventsSessionIDsFromFilter(subscription.Filter, inputs, true)
	if err != nil {
		return nil, err
	}
	if !ok || len(sessionIDs) == 0 {
		return nil, fmt.Errorf(
			"%w: watch-events kind %q requires a session_id filter",
			ErrValidation,
			subscription.Kind,
		)
	}
	streams := make([]string, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		if stream := WatchEventsSessionStreamForSession(sessionID); stream != "" {
			streams = append(streams, stream)
		}
	}
	if len(streams) == 0 {
		return nil, fmt.Errorf("%w: watch-events session_id filter is empty", ErrValidation)
	}
	slices.Sort(streams)
	return streams, nil
}

func watchEventsLedgerKinds(
	subscriptions []watchpkg.EventSubscriptionRef,
	contracts map[hooks.HookEvent]WatchEventsContract,
) ([]string, error) {
	kindSet := map[string]struct{}{}
	for _, subscription := range subscriptions {
		contract, ok := contracts[hooks.HookEvent(strings.TrimSpace(subscription.Kind))]
		if !ok {
			return nil, fmt.Errorf("%w: watch-events kind is unsupported: %q", ErrValidation, subscription.Kind)
		}
		for _, ledgerType := range contract.LedgerTypes {
			if trimmed := strings.TrimSpace(ledgerType); trimmed != "" {
				kindSet[trimmed] = struct{}{}
			}
		}
	}
	kinds := make([]string, 0, len(kindSet))
	for kind := range kindSet {
		kinds = append(kinds, kind)
	}
	slices.Sort(kinds)
	return kinds, nil
}

func watchEventsPendingStateHasCursorForKind(
	state watchpkg.EventsPendingState,
	ledgerKind string,
	contracts map[hooks.HookEvent]WatchEventsContract,
) bool {
	for _, subscription := range state.Subscriptions {
		contract, ok := contracts[hooks.HookEvent(strings.TrimSpace(subscription.Kind))]
		if !ok || !slices.Contains(contract.LedgerTypes, ledgerKind) {
			continue
		}
		for stream := range state.Cursors {
			if watchEventsContractStreamMatches(contract.Stream, stream) {
				return true
			}
		}
	}
	return false
}

func watchEventsQuery(
	run Run,
	state watchpkg.EventsPendingState,
	contracts map[hooks.HookEvent]WatchEventsContract,
) (WatchEventsQuery, error) {
	kinds, err := watchEventsLedgerKinds(state.Subscriptions, contracts)
	if err != nil {
		return WatchEventsQuery{}, err
	}
	return WatchEventsQuery{
		WorkspaceID: string(run.WorkspaceID),
		Streams:     cloneInt64Map(state.Cursors),
		Kinds:       kinds,
		Limit:       LoopMaxFanoutWidth,
	}, nil
}
