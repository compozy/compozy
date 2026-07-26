package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/compozy/agh/internal/loop/dsl"
	watchpkg "github.com/compozy/agh/internal/loop/watch"
	"github.com/compozy/agh/internal/task"
)

const (
	watchSourceNotReadyReason    = "watch_source_not_ready"
	watchSourceSilenceReason     = "watch_source_silence"
	watchSourceSpecInvalidReason = "watch_source_spec_invalid"
	watchSourcePollerReason      = "watch_source_poller_unavailable"
)

type coordinatorWatchRuntime struct {
	poller        WatchPoller
	silenceWindow time.Duration
	now           func() time.Time
	logger        *slog.Logger
}

func (r *CoordinatorRunner) watchRuntime() coordinatorWatchRuntime {
	now := r.now
	if now == nil {
		now = time.Now
	}
	return coordinatorWatchRuntime{
		poller:        r.watchPoller,
		silenceWindow: r.watchSilenceWindow,
		now:           now,
		logger:        r.logger,
	}
}

func isWatchSourceNode(node dsl.Node) bool {
	return node.Class == dsl.NodeClassSource && node.Kind == string(dsl.SourceWatchSource)
}

func evaluateWatchSourceNode(
	ctx context.Context,
	plan *task.CoordinatorCompletionPlan,
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	topology controlTopology,
	output GenerationOutput,
	node dsl.Node,
	outputs []GenerationOutput,
	runtime coordinatorWatchRuntime,
) (GenerationOutput, *task.CoordinatorTerminal, error) {
	if runtime.poller == nil {
		return output, watchBlockedTerminal(watchSourcePollerReason), nil
	}
	namespace, err := runtimeNamespace(
		run,
		generation,
		resolved.Definition.Graph,
		topology,
		outputs,
		node.ID,
		output.ItemIndex,
	)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	spec, terminal, err := watchSpecRaw(node, namespace)
	if err != nil || terminal != nil {
		return output, terminal, err
	}
	expectedDigest, err := watchpkg.ExpectedStateDigestFromOutputRef(output.OutputRef)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	adapter, err := watchpkg.NewAdapter(runtime.poller)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	result, err := adapter.Tick(ctx, watchpkg.TickRequest{
		Spec:                spec,
		ExpectedStateDigest: expectedDigest,
		LastProgressAt:      run.LastProgressAt,
		SilenceWindow:       runtime.silenceWindow,
		Now:                 runtime.now().UTC(),
	})
	if err != nil {
		return GenerationOutput{}, nil, newWatchPollFailureError(err)
	}
	logWatchTransitions(runtime, run, node, result.Transitions)
	switch result.Outcome {
	case watchpkg.OutcomeReady:
		ref, err := watchpkg.ConfirmedOutputRef(result.Response)
		if err != nil {
			return GenerationOutput{}, nil, err
		}
		output.Status = generationOutputSucceeded
		output.OutputRef = ref
		return output, nil, nil
	case watchpkg.OutcomeStalled:
		ref, err := watchpkg.PendingOutputRef(result.Response)
		if err != nil {
			return GenerationOutput{}, nil, err
		}
		output.OutputRef = ref
		return output, watchStalledTerminal(), nil
	case watchpkg.OutcomeWaiting:
		ref, err := watchpkg.PendingOutputRef(result.Response)
		if err != nil {
			return GenerationOutput{}, nil, err
		}
		output.OutputRef = ref
		if run.Status == StatusWatching {
			plan.Yield = true
			return output, nil, nil
		}
		return output, watchWaitingTerminal(), nil
	default:
		return GenerationOutput{}, nil, fmt.Errorf("%w: unknown watch outcome %q", ErrValidation, result.Outcome)
	}
}

func logWatchTransitions(
	runtime coordinatorWatchRuntime,
	run Run,
	node dsl.Node,
	transitions []watchpkg.Transition,
) {
	if len(transitions) == 0 || runtime.logger == nil {
		return
	}
	for _, transition := range transitions {
		runtime.logger.Info(
			"loop watch source transition",
			"loop_run_id", string(run.ID),
			"workspace_id", string(run.WorkspaceID),
			"generation", run.Generation,
			"node_id", string(node.ID),
			"event", transition.Event,
			"from", transition.From,
			"to", transition.To,
		)
	}
}

func watchSpecRaw(node dsl.Node, namespace map[string]any) (json.RawMessage, *task.CoordinatorTerminal, error) {
	if len(node.WatchSpec) == 0 {
		return nil, watchBlockedTerminal(watchSourceSpecInvalidReason), nil
	}
	rendered, err := renderAny(fmt.Sprintf("nodes.%s.watch", node.ID), node.WatchSpec, namespace)
	if err != nil {
		return nil, nil, err
	}
	specMap, ok := rendered.(map[string]any)
	if !ok {
		return nil, watchBlockedTerminal(watchSourceSpecInvalidReason), nil
	}
	kindValue, ok := specMap["kind"]
	if !ok {
		return nil, watchBlockedTerminal(watchSourceSpecInvalidReason), nil
	}
	kind, ok := kindValue.(string)
	if !ok || strings.TrimSpace(kind) == "" {
		return nil, watchBlockedTerminal(watchSourceSpecInvalidReason), nil
	}
	spec, err := json.Marshal(specMap)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: marshal watch spec for node %q: %w", ErrValidation, node.ID, err)
	}
	return spec, nil, nil
}

func watchBlockedTerminal(reasonCode string) *task.CoordinatorTerminal {
	return &task.CoordinatorTerminal{
		Status:     string(StatusBlocked),
		Cause:      string(TransitionCauseContract),
		ReasonCode: reasonCode,
	}
}

func watchWaitingTerminal() *task.CoordinatorTerminal {
	return &task.CoordinatorTerminal{
		Status:     string(StatusWatching),
		Cause:      string(TransitionCauseWatchPoll),
		ReasonCode: watchSourceNotReadyReason,
	}
}

func watchStalledTerminal() *task.CoordinatorTerminal {
	return &task.CoordinatorTerminal{
		Status:     string(StatusStalled),
		Cause:      string(TransitionCauseNoProgress),
		ReasonCode: watchSourceSilenceReason,
	}
}
