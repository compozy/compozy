package loop

import (
	"math"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func (c *lintContext) lintMetrics() {
	metricSeen := false
	duplicateReported := false
	c.lintMetricCriteria("", c.def.Contract.Verification, &metricSeen, &duplicateReported)
	c.lintMetricGraph(c.def.Graph.Nodes, &metricSeen, &duplicateReported)
}

func (c *lintContext) lintMetricGraph(nodes []dsl.Node, metricSeen, duplicateReported *bool) {
	for _, node := range nodes {
		c.lintMetricCriteria(node.ID, node.Criteria, metricSeen, duplicateReported)
		if node.Body != nil {
			c.lintMetricGraph(node.Body.Nodes, metricSeen, duplicateReported)
		}
	}
}

func (c *lintContext) lintMetricCriteria(
	nodeID dsl.NodeID,
	criteria []dsl.GateCriterion,
	metricSeen *bool,
	duplicateReported *bool,
) {
	for _, criterion := range criteria {
		if criterion.Metric == nil {
			continue
		}
		if *metricSeen && !*duplicateReported {
			c.add(
				nodeID,
				CodeMetricSingle,
				"metric criterion %q exceeds the one-metric limit",
				criterion.ID,
			)
			*duplicateReported = true
		}
		*metricSeen = true
		c.lintMetricCriterion(nodeID, criterion)
	}
}

func (c *lintContext) lintMetricCriterion(nodeID dsl.NodeID, criterion dsl.GateCriterion) {
	if !isMetricCriterionType(criterion.Type) {
		c.add(
			nodeID,
			CodeMetricMachineCriterionRequired,
			"metric criterion %q must use command, agent-judge, or extension type",
			criterion.ID,
		)
	}
	if criterion.Metric.Direction == "" {
		c.add(nodeID, CodeMetricDirectionRequired, "metric criterion %q direction is required", criterion.ID)
	} else if criterion.Metric.Direction != dsl.MetricMaximize && criterion.Metric.Direction != dsl.MetricMinimize {
		c.add(
			nodeID,
			CodeMetricDirectionInvalid,
			"metric criterion %q direction %q must be maximize or minimize",
			criterion.ID,
			criterion.Metric.Direction,
		)
	}
	if criterion.Metric.MinDelta != nil &&
		(math.IsNaN(*criterion.Metric.MinDelta) || math.IsInf(*criterion.Metric.MinDelta, 0) ||
			*criterion.Metric.MinDelta < 0) {
		c.add(
			nodeID,
			CodeMetricMinDeltaInvalid,
			"metric criterion %q min_delta must be finite and non-negative",
			criterion.ID,
		)
	}
}

func isMetricCriterionType(criterionType dsl.CriterionType) bool {
	switch criterionType {
	case dsl.CriterionCommand, dsl.CriterionAgentJudge, dsl.CriterionExtension:
		return true
	default:
		return false
	}
}
