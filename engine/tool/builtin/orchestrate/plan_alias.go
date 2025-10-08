package orchestrate

import (
	specpkg "github.com/compozy/compozy/engine/tool/builtin/orchestrate/spec"
)

// The orchestrate package re-exports plan and step definitions from the spec
// subpackage to preserve the existing public API while allowing the planner
// subpackage to depend on the shared types without introducing an import cycle.
// Consumers continue using orchestrate.Plan and friends, but the actual
// implementation now lives under spec.

type (
	Plan            = specpkg.Plan
	Step            = specpkg.Step
	AgentStep       = specpkg.AgentStep
	ParallelStep    = specpkg.ParallelStep
	StepTransitions = specpkg.StepTransitions
	StepType        = specpkg.StepType
	StepStatus      = specpkg.StepStatus
	StepEvent       = specpkg.StepEvent
	MergeStrategy   = specpkg.MergeStrategy
)

const (
	StepTypeAgent    = specpkg.StepTypeAgent
	StepTypeParallel = specpkg.StepTypeParallel

	StepStatusPending StepStatus = specpkg.StepStatusPending
	StepStatusRunning StepStatus = specpkg.StepStatusRunning
	StepStatusSuccess StepStatus = specpkg.StepStatusSuccess
	StepStatusFailed  StepStatus = specpkg.StepStatusFailed
	StepStatusPartial StepStatus = specpkg.StepStatusPartial
	StepStatusSkipped StepStatus = specpkg.StepStatusSkipped

	StepEventStartPlan       = specpkg.StepEventStartPlan
	StepEventPlannerFinished = specpkg.StepEventPlannerFinished
	StepEventValidationFail  = specpkg.StepEventValidationFail
	StepEventDispatchStep    = specpkg.StepEventDispatchStep
	StepEventStepSuccess     = specpkg.StepEventStepSuccess
	StepEventStepFailed      = specpkg.StepEventStepFailed
	StepEventParallelDone    = specpkg.StepEventParallelDone
	StepEventTimeout         = specpkg.StepEventTimeout
	StepEventPanic           = specpkg.StepEventPanic

	MergeStrategyCollect      = specpkg.MergeStrategyCollect
	MergeStrategyFirstSuccess = specpkg.MergeStrategyFirstSuccess
)

var (
	PlanSchema    = specpkg.PlanSchema
	DecodePlanMap = specpkg.DecodePlanMap
)
