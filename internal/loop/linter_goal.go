package loop

import (
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/dsl/refs"
)

func (c *lintContext) lintGoalNode(node dsl.Node) {
	var params dsl.GoalParams
	if err := node.Params.Decode(&params); err != nil {
		c.add(node.ID, refs.CodeUnresolvablePath, "goal params are invalid: %v", err)
		return
	}
	if strings.TrimSpace(params.Objective) == "" {
		c.add(node.ID, CodeGoalObjectiveRequired, "goal params.objective is required")
	}
	c.lintGoalJudges(node.ID, params.Judge)
	if params.MaxTurns < 1 {
		c.add(node.ID, CodeGoalMaxTurnsRequired, "goal params.max_turns must be at least 1")
	}
	if !goalOutputAllowsBlocked(params.OutputSchema) {
		c.add(
			node.ID,
			CodeGoalOutputStatusMissingBlocked,
			"goal params.output_schema status enum must include blocked",
		)
	}
	if params.OnExhausted != "" && params.OnExhausted != dsl.GoalOnExhaustedHalt &&
		params.OnExhausted != dsl.GoalOnExhaustedEscalate {
		c.add(
			node.ID,
			CodeGoalOnExhaustedInvalid,
			"goal params.on_exhausted must be halt or escalate",
		)
	}

	continuous := c.lintGoalSession(node)
	c.lintGoalRetry(node, continuous)
	if continuous && c.goalHasParallelFanOutAncestor(node.ID) {
		c.add(
			node.ID,
			CodeContinuousForbidsParallel,
			"continuous goal cannot execute beneath fan-out with max_parallel greater than 1",
		)
	}
}

func (c *lintContext) lintGoalJudges(nodeID dsl.NodeID, criteria []dsl.GateCriterion) {
	if len(criteria) == 0 {
		c.add(nodeID, CodeGoalJudgeRequired, "goal params.judge must declare at least one criterion")
		return
	}
	ids := make(map[string]struct{}, len(criteria))
	for idx, criterion := range criteria {
		id := strings.TrimSpace(criterion.ID)
		if id == "" {
			c.add(nodeID, CodeGoalJudgeRequired, "goal judge criterion %d id is required", idx)
		} else if _, exists := ids[id]; exists {
			c.add(nodeID, CodeGoalJudgeRequired, "goal judge criterion id %q must be unique", id)
		}
		ids[id] = struct{}{}

		switch criterion.Type {
		case dsl.CriterionCommand:
			if strings.TrimSpace(criterion.Check) == "" {
				c.add(nodeID, CodeGoalJudgeRequired, "goal command judge %q check is required", id)
			}
		case dsl.CriterionAgentJudge:
			if strings.TrimSpace(criterion.Rubric) == "" && strings.TrimSpace(criterion.Prompt) == "" {
				c.add(nodeID, CodeGoalJudgeRequired, "goal agent judge %q rubric is required", id)
			}
		case dsl.CriterionExtension:
			if strings.TrimSpace(criterion.Tool) == "" {
				c.add(nodeID, CodeGoalJudgeRequired, "goal extension judge %q tool is required", id)
			}
		case dsl.CriterionHuman:
			c.add(nodeID, CodeGoalHumanJudgeUnsupported, "goal human judge %q is unsupported", id)
		default:
			c.add(nodeID, CodeGoalJudgeRequired, "goal judge %q type %q is unsupported", id, criterion.Type)
		}
	}
}

func (c *lintContext) lintGoalSession(node dsl.Node) bool {
	if node.Session == nil {
		return true
	}
	session := node.Session
	isContinuous := session.Mode == dsl.SessionModeContinuous && !session.Isolated
	isIsolated := session.Isolated && session.Mode == "" && strings.TrimSpace(session.Handle) == ""
	if !isContinuous && !isIsolated {
		c.add(
			node.ID,
			CodeSessionSpecAmbiguous,
			"goal session must select exactly isolated: true or mode: continuous",
		)
	}
	return isContinuous
}

func (c *lintContext) lintGoalRetry(node dsl.Node, continuous bool) {
	if node.Retry == nil {
		return
	}
	if _, exists := node.Retry.Extra["max"]; exists {
		c.add(node.ID, CodeRetryMaxUnsupported, "retry.max is unsupported; use retry.max_attempts")
	}
	if node.Retry.OnFailure == "" {
		return
	}
	if node.Retry.OnFailure != dsl.RetryOnFailureFreshSession || !continuous ||
		node.Retry.MaxAttempts < 2 {
		c.add(
			node.ID,
			CodeRetryFreshSessionRequiresContinuous,
			"retry.on_failure fresh_session requires continuous mode and max_attempts of at least 2",
		)
	}
}

func goalOutputAllowsBlocked(schema *dsl.Schema) bool {
	if schema == nil {
		return false
	}
	status, ok := mapValue(map[string]any(*schema), "status")
	if !ok {
		if properties, propertiesOK := mapValue(map[string]any(*schema), "properties"); propertiesOK {
			status, ok = mapValue(asStringMap(properties), "status")
		}
	}
	if !ok {
		return false
	}
	statusSchema := asStringMap(status)
	values, ok := statusSchema[jsonSchemaEnumKey]
	if !ok {
		return false
	}
	switch typed := values.(type) {
	case []any:
		return slices.ContainsFunc(typed, func(value any) bool {
			return fmt.Sprint(value) == string(ActionDispositionBlocked)
		})
	case []string:
		return slices.Contains(typed, "blocked")
	default:
		return false
	}
}

func mapValue(values map[string]any, key string) (any, bool) {
	value, ok := values[key]
	return value, ok
}

func asStringMap(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	case dsl.Schema:
		return map[string]any(typed)
	case map[any]any:
		converted := make(map[string]any, len(typed))
		for key, child := range typed {
			converted[fmt.Sprint(key)] = child
		}
		return converted
	default:
		return nil
	}
}

func (c *lintContext) goalHasParallelFanOutAncestor(id dsl.NodeID) bool {
	seen := map[dsl.NodeID]struct{}{}
	var visit func(dsl.NodeID) bool
	visit = func(current dsl.NodeID) bool {
		if _, exists := seen[current]; exists {
			return false
		}
		seen[current] = struct{}{}
		if node, ok := c.nodeByID[current]; ok && isCollectNode(node) {
			return false
		}
		for _, previous := range c.reverse[current] {
			ancestor, ok := c.nodeByID[previous]
			if ok && ancestor.Class == dsl.NodeClassControl &&
				dsl.ControlKind(ancestor.Kind) == dsl.ControlFanOut && ancestor.MaxParallel > 1 {
				return true
			}
			if visit(previous) {
				return true
			}
		}
		return false
	}
	return visit(id)
}

func (c *lintContext) lintContinuousGoalHandles() {
	labels := map[string]dsl.NodeID{}
	for _, node := range c.def.Graph.Nodes {
		if node.Class != dsl.NodeClassAction || dsl.ActionKind(node.Kind) != dsl.ActionGoal ||
			!goalUsesContinuousSession(node) {
			continue
		}
		label := string(node.ID)
		if node.Session != nil && strings.TrimSpace(node.Session.Handle) != "" {
			label = strings.TrimSpace(node.Session.Handle)
		}
		if previous, exists := labels[label]; exists {
			c.add(
				node.ID,
				CodeContinuousHandleReused,
				"continuous goal handle label %q is reused by nodes %q and %q",
				label,
				previous,
				node.ID,
			)
			continue
		}
		labels[label] = node.ID
	}
}

func goalUsesContinuousSession(node dsl.Node) bool {
	return node.Session == nil || node.Session.Mode == dsl.SessionModeContinuous
}
