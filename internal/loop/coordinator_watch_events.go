package loop

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/loop/dsl"
	watchpkg "github.com/compozy/compozy/internal/loop/watch"
	"github.com/compozy/compozy/internal/task"
)

const (
	watchEventsNotReadyReason        = "watch_events_not_ready"
	watchEventsSpecInvalidReason     = "watch_events_spec_invalid"
	watchEventsLedgerUnavailableCode = "watch_events_ledger_unavailable"
	watchEventsFilterReason          = "watch_events_filter"
)

type coordinatorWatchEventsRuntime struct {
	ledger       WatchEventsLedger
	cursorReader WatchEventsCursorReader
	now          func() time.Time
	logger       *slog.Logger
}

type watchEventsEvaluationContext struct {
	control     controlEvalContext
	plan        *task.CoordinatorCompletionPlan
	outputs     []GenerationOutput
	outputBlobs *[]GenerationOutputBlob
}

func (r *CoordinatorRunner) watchEventsRuntime() coordinatorWatchEventsRuntime {
	now := r.now
	if now == nil {
		now = time.Now
	}
	var cursorReader WatchEventsCursorReader
	if reader, ok := r.watchEventsLedger.(WatchEventsCursorReader); ok {
		cursorReader = reader
	}
	return coordinatorWatchEventsRuntime{
		ledger:       r.watchEventsLedger,
		cursorReader: cursorReader,
		now:          now,
		logger:       r.logger,
	}
}

func isWatchEventsNode(node dsl.Node) bool {
	return node.Class == dsl.NodeClassSource && node.Kind == string(dsl.SourceWatchEvents)
}

func evaluateWatchEventsNode(
	evaluation *watchEventsEvaluationContext,
	output GenerationOutput,
	node dsl.Node,
) (GenerationOutput, *task.CoordinatorTerminal, error) {
	control := evaluation.control
	runtime := control.watchEventsRuntime
	if runtime.ledger == nil || runtime.cursorReader == nil {
		return output, watchEventsBlockedTerminal(watchEventsLedgerUnavailableCode), nil
	}
	state, terminal, err := recoverWatchEventsState(
		control.ctx,
		control.run,
		output,
		node,
		control.resolved.WatchEventsContracts,
		runtime,
	)
	if err != nil || terminal != nil {
		return output, terminal, err
	}
	query, err := watchEventsQuery(control.run, state, control.resolved.WatchEventsContracts)
	if err != nil {
		return output, watchEventsBlockedTerminal(watchEventsSpecInvalidReason), nil
	}
	rows, err := runtime.ledger.ReadMatches(control.ctx, query)
	if err != nil {
		return GenerationOutput{}, nil, fmt.Errorf("loop: read watch-events matches for node %s: %w", node.ID, err)
	}
	matches, nextCursors, readCounts, err := filterWatchEventsRows(
		evaluation,
		output,
		node,
		state,
		rows,
	)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	if watchEventsReadMayBeTruncated(readCounts, query.Limit) {
		evaluation.plan.PostCommitWakes = append(evaluation.plan.PostCommitWakes, task.CoordinatorWakeSpec{
			LoopRunID:      string(control.run.ID),
			IdempotencyKey: watchEventsWakeKey(control.run.ID, node.ID),
		})
	}
	if len(matches) == 0 {
		state.Cursors = nextCursors
		ref, err := watchpkg.EventsPendingOutputRef(state)
		if err != nil {
			return GenerationOutput{}, nil, err
		}
		output.OutputRef = ref
		if control.run.Status == StatusWatching {
			evaluation.plan.Yield = true
			return output, nil, nil
		}
		return output, watchEventsWaitingTerminal(), nil
	}

	ref, err := confirmedWatchEventsOutputRef(matches, nextCursors, evaluation.outputBlobs, runtime.now().UTC())
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	output.Status = generationOutputSucceeded
	output.OutputRef = ref
	logWatchEventsEvaluation(runtime, control.run, node, len(rows), len(matches), nextCursors)
	return output, nil, nil
}

func filterWatchEventsRows(
	evaluation *watchEventsEvaluationContext,
	output GenerationOutput,
	node dsl.Node,
	state watchpkg.EventsPendingState,
	rows []WatchEvent,
) ([]WatchEvent, map[string]int64, map[string]int, error) {
	control := evaluation.control
	var namespace map[string]any
	if watchEventsSubscriptionsHaveFilters(state.Subscriptions) && len(rows) > 0 {
		var err error
		namespace, err = runtimeNamespaceWithHistory(
			control.run,
			control.generation,
			control.resolved.Definition.Graph,
			control.topology,
			evaluation.outputs,
			control.history,
			node.ID,
			output.ItemIndex,
		)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	nextCursors := cloneInt64Map(state.Cursors)
	readCounts := map[string]int{}
	matches := make([]WatchEvent, 0, len(rows))
	for _, row := range rows {
		stream := strings.TrimSpace(row.Stream)
		readCounts[stream]++
		if row.Seq > nextCursors[stream] {
			nextCursors[stream] = row.Seq
		}
		matched, event, err := rowMatchesWatchEventsSubscriptions(
			control.resolved,
			node,
			namespace,
			state.Subscriptions,
			row,
		)
		if err != nil {
			return nil, nil, nil, err
		}
		if matched {
			matches = append(matches, event)
		}
	}
	return matches, nextCursors, readCounts, nil
}

func watchEventsSubscriptionsHaveFilters(subscriptions []watchpkg.EventSubscriptionRef) bool {
	for _, subscription := range subscriptions {
		if strings.TrimSpace(subscription.Filter) != "" {
			return true
		}
	}
	return false
}

func rowMatchesWatchEventsSubscriptions(
	resolved *ResolvedDefinition,
	node dsl.Node,
	namespace map[string]any,
	subscriptions []watchpkg.EventSubscriptionRef,
	row WatchEvent,
) (bool, WatchEvent, error) {
	for idx, subscription := range subscriptions {
		contract, ok := resolved.WatchEventsContracts[hooks.HookEvent(strings.TrimSpace(subscription.Kind))]
		if !ok {
			return false, WatchEvent{}, fmt.Errorf(
				"%w: watch-events kind is unsupported: %q",
				ErrValidation,
				subscription.Kind,
			)
		}
		if !watchEventsContractStreamMatches(contract.Stream, row.Stream) ||
			!slices.Contains(contract.LedgerTypes, row.ledgerKind()) {
			continue
		}
		event := cloneWatchEvent(row)
		event.Kind = strings.TrimSpace(subscription.Kind)
		if strings.TrimSpace(subscription.Filter) == "" {
			return true, event, nil
		}
		matched, err := evaluateWatchEventsFilter(
			resolved,
			node,
			namespace,
			idx,
			event,
		)
		if err != nil {
			return false, WatchEvent{}, err
		}
		if matched {
			return true, event, nil
		}
	}
	return false, WatchEvent{}, nil
}

func watchEventsContractStreamMatches(contractStream string, rowStream string) bool {
	return strings.TrimSpace(contractStream) == WatchEventsBaseStream(rowStream)
}

func evaluateWatchEventsFilter(
	resolved *ResolvedDefinition,
	node dsl.Node,
	namespace map[string]any,
	subscriptionIndex int,
	event WatchEvent,
) (bool, error) {
	key := fmt.Sprintf("nodes.%s.events.%d.filter", node.ID, subscriptionIndex)
	condition := resolved.Conditions[key]
	if condition == nil {
		return false, fmt.Errorf("%w: compiled watch-events filter %q is missing", ErrValidation, key)
	}
	namespace["event"] = event.eventMap()
	value, _, err := condition.Program.Eval(namespace)
	if err != nil {
		return false, fmt.Errorf("loop: evaluate watch-events filter %s: %w", node.ID, err)
	}
	result, ok := value.Value().(bool)
	if !ok {
		return false, fmt.Errorf(
			"%w: watch-events filter %s returned %T",
			ErrValidation,
			key,
			value.Value(),
		)
	}
	return result, nil
}

func confirmedWatchEventsOutputRef(
	events []WatchEvent,
	cursors map[string]int64,
	outputBlobs *[]GenerationOutputBlob,
	at time.Time,
) (string, error) {
	payload, err := watchpkg.EventsConfirmedOutputPayload(events, cursors)
	if err != nil {
		return "", err
	}
	if !OutputPayloadRequiresRef(payload) {
		return string(payload), nil
	}
	if outputBlobs == nil {
		return "", fmt.Errorf("%w: output blob sink is required", ErrValidation)
	}
	ref := OutputRefForPayload(payload)
	*outputBlobs = append(*outputBlobs, GenerationOutputBlob{
		OutputRef: ref,
		Payload:   payload,
		At:        at,
	})
	return ref, nil
}

func watchEventsReadMayBeTruncated(readCounts map[string]int, limit int) bool {
	if limit <= 0 {
		return false
	}
	for _, count := range readCounts {
		if count >= limit {
			return true
		}
	}
	return false
}

func watchEventsWakeKey(runID RunID, nodeID dsl.NodeID) string {
	return fmt.Sprintf("loop.coordinator.watch_events.%s.%s", runID, nodeID)
}

func watchEventsBlockedTerminal(reasonCode string) *task.CoordinatorTerminal {
	return &task.CoordinatorTerminal{
		Status:     string(StatusBlocked),
		Cause:      string(TransitionCauseContract),
		ReasonCode: reasonCode,
	}
}

func watchEventsWaitingTerminal() *task.CoordinatorTerminal {
	return &task.CoordinatorTerminal{
		Status:     string(StatusWatching),
		Cause:      string(TransitionCauseWatchEvents),
		ReasonCode: watchEventsNotReadyReason,
	}
}

func cloneWatchEvent(src WatchEvent) WatchEvent {
	src.Payload = cloneAnyMap(src.Payload)
	return src
}

func cloneInt64Map(src map[string]int64) map[string]int64 {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]int64, len(src))
	maps.Copy(dst, src)
	return dst
}

func logWatchEventsEvaluation(
	runtime coordinatorWatchEventsRuntime,
	run Run,
	node dsl.Node,
	readCount int,
	matchCount int,
	cursors map[string]int64,
) {
	if runtime.logger == nil {
		return
	}
	runtime.logger.Debug(
		"loop watch-events evaluation",
		"loop_run_id", string(run.ID),
		"workspace_id", string(run.WorkspaceID),
		"node_id", string(node.ID),
		"read_count", readCount,
		"match_count", matchCount,
		"cursors", cursors,
	)
}

var _ WatchEventsLedger = (*watchEventsLedgerFunc)(nil)

type watchEventsLedgerFunc struct {
	read func(context.Context, WatchEventsQuery) ([]WatchEvent, error)
}

func (f *watchEventsLedgerFunc) ReadMatches(ctx context.Context, query WatchEventsQuery) ([]WatchEvent, error) {
	return f.read(ctx, query)
}
