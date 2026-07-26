package loop

import (
	"context"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/task"
)

func evaluateSourceControlNode(
	ctx context.Context,
	plan *task.CoordinatorCompletionPlan,
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	topology controlTopology,
	watchRuntime coordinatorWatchRuntime,
	watchEventsRuntime coordinatorWatchEventsRuntime,
	output GenerationOutput,
	node dsl.Node,
	outputs []GenerationOutput,
	outputBlobs *[]GenerationOutputBlob,
) (GenerationOutput, *task.CoordinatorTerminal, bool, error) {
	if isInputSourceNode(node) {
		evaluated, err := evaluateInputSourceNode(run, output, node)
		return evaluated, nil, true, err
	}
	if isFileImportSourceNode(node) {
		evaluated, err := evaluateFileImportNode(
			run,
			generation,
			resolved,
			topology,
			output,
			node,
			outputs,
		)
		return evaluated, nil, true, err
	}
	if isWatchSourceNode(node) {
		evaluated, terminal, err := evaluateWatchSourceNode(
			ctx,
			plan,
			run,
			generation,
			resolved,
			topology,
			output,
			node,
			outputs,
			watchRuntime,
		)
		return evaluated, terminal, true, err
	}
	if isWatchEventsNode(node) {
		evaluated, terminal, err := evaluateWatchEventsNode(
			ctx,
			plan,
			run,
			generation,
			resolved,
			topology,
			output,
			node,
			outputs,
			outputBlobs,
			watchEventsRuntime,
		)
		return evaluated, terminal, true, err
	}
	return output, nil, false, nil
}
