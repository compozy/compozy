package loop

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/loop/gate"
)

type controlEvalContext struct {
	ctx                context.Context
	run                Run
	generation         int
	resolved           *ResolvedDefinition
	topology           controlTopology
	effective          EffectiveConfig
	gateEvaluator      gate.GateEvaluator
	gateDecisions      GateDecisionReader
	nodeControls       NodeControlReader
	runtimeCatalog     WorkspaceRuntimeCatalog
	fanOutWidth        int
	watchRuntime       coordinatorWatchRuntime
	watchEventsRuntime coordinatorWatchEventsRuntime
	gateEvaluations    *gateEvaluationCollector
	history            GenerationHistory
	now                time.Time
}
