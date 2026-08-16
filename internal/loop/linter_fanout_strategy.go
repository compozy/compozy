package loop

import (
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func (c *lintContext) lintFanOut(node dsl.Node) {
	if strings.TrimSpace(node.Collection) == "" || node.MaxFanOut <= 0 {
		c.add(node.ID, CodeFanOutUnbounded, "fan-out must declare collection and max_fan_out")
	}
	c.lintFanOutStrategy(node)
	c.lintFanOutIterationNames(node)
}

func (c *lintContext) lintFanOutStrategy(node dsl.Node) {
	strategy := node.Strategy
	if strategy == nil {
		return
	}
	if err := strategy.ValidateShape(); err != nil {
		c.add(node.ID, CodeStrategyThresholdInvalid, "%v", err)
		return
	}
	if strategy.Kind == dsl.StrategyBestEffort {
		if strategy.Missing != dsl.MissingAcceptable {
			c.add(node.ID, CodeStrategyCoverageUndeclared,
				"best_effort requires missing: acceptable")
		}
		if strategy.Threshold == nil {
			c.add(node.ID, CodeStrategyThresholdInvalid,
				"best_effort requires a threshold")
			return
		}
		if strategy.Threshold.Kind == dsl.ThresholdPercent && strategy.Threshold.Percent == 100 {
			c.warn(node.ID, CodeStrategyWaitAllEquivalent,
				"best_effort at 100%% is equivalent to wait_all")
		}
		return
	}
	if strategy.Threshold != nil || strategy.Missing != "" {
		c.add(node.ID, CodeStrategyThresholdInvalid,
			"strategy %q does not accept threshold or missing", strategy.Kind)
	}
}

func (c *lintContext) lintFanOutIterationNames(node dsl.Node) {
	reserved := map[string]struct{}{
		"inputs": {}, "nodes": {}, "item": {}, "index": {}, "trigger": {},
		"generation": {}, "event": {}, "previous": {}, "best": {}, "output": {}, "progress": {},
	}
	seen := make(map[string]struct{}, 2)
	ancestorNames := map[string]struct{}{}
	for _, ancestor := range c.fanOutAncestors(node.ID) {
		for _, name := range []string{strings.TrimSpace(ancestor.BindAs), strings.TrimSpace(ancestor.IndexAs)} {
			if name != "" {
				ancestorNames[name] = struct{}{}
			}
		}
	}
	for _, raw := range []string{node.BindAs, node.IndexAs} {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		_, conflict := reserved[name]
		_, duplicate := seen[name]
		_, shadowed := ancestorNames[name]
		if conflict || duplicate || shadowed || !nodeIDPattern.MatchString(name) {
			c.add(node.ID, CodeIterationNameConflict,
				"iteration name %q is reserved, duplicated, or invalid", name)
		}
		seen[name] = struct{}{}
	}
}

func (c *lintContext) fanOutAncestors(nodeID dsl.NodeID) []dsl.Node {
	seen := map[dsl.NodeID]struct{}{}
	ancestors := []dsl.Node{}
	var visit func(dsl.NodeID)
	visit = func(current dsl.NodeID) {
		for _, previous := range c.reverse[current] {
			if _, ok := seen[previous]; ok {
				continue
			}
			seen[previous] = struct{}{}
			node, ok := c.nodeByID[previous]
			if !ok || isCollectNode(node) {
				continue
			}
			if isControlKind(node, dsl.ControlFanOut) {
				ancestors = append(ancestors, node)
			}
			visit(previous)
		}
	}
	visit(nodeID)
	slices.SortFunc(ancestors, func(left, right dsl.Node) int {
		return strings.Compare(string(left.ID), string(right.ID))
	})
	return ancestors
}
