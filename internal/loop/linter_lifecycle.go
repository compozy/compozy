package loop

import (
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
)

func (c *lintContext) lintLifecycleGrammar() {
	for _, node := range c.def.Graph.Nodes {
		c.lintLifecycleNode(node)
	}
	c.lintEffectLists("", contractEffectLists(c.def.Contract))
}

func (c *lintContext) lintLifecycleNode(node dsl.Node) {
	timeout, timeoutOK := c.lintLifecycleDuration(node.ID, "timeout", node.Timeout)
	deadline, deadlineOK := c.lintLifecycleDuration(node.ID, "deadline", node.Deadline)
	if timeoutOK && deadlineOK && timeout > deadline {
		c.add(node.ID, CodeTimeoutExceedsDeadline, "timeout %s exceeds deadline %s", node.Timeout, node.Deadline)
	}
	c.lintRetryLifecycle(node)
	c.lintErrorPolicy(node)
	c.lintResultContract(node)
	c.lintEffectLists(node.ID, nodeEffectLists(node))
	c.lintParentClose(node)
	c.lintWatchIdentity(node)
	if node.Class == dsl.NodeClassControl && dsl.ControlKind(node.Kind) == dsl.ControlWait {
		c.lintWait(node)
	}
	if node.Class == dsl.NodeClassControl && dsl.ControlKind(node.Kind) == dsl.ControlAsk {
		var params dsl.AskParams
		if err := node.Params.Decode(&params); err == nil && params.Expires != nil {
			c.lintWaitExpiry(node, params.Expires, "ask")
		}
	}
	if node.Review != nil && node.Review.Expires != nil {
		c.lintWaitExpiry(node, node.Review.Expires, "review")
	}
}

func (c *lintContext) lintRetryLifecycle(node dsl.Node) {
	if node.Class == dsl.NodeClassAction && dsl.ActionKind(node.Kind) == dsl.ActionGoal {
		hasGenericRetry := node.Retry != nil && (node.Retry.Backoff != nil || len(node.Retry.NonRetryable) > 0)
		if strings.TrimSpace(node.Deadline) != "" || hasGenericRetry {
			c.add(
				node.ID,
				CodeRetryOnGoalNode,
				"goal nodes do not accept deadline, retry.backoff, or retry.non_retryable",
			)
		}
	}
	if node.Retry == nil {
		return
	}
	if node.Retry.Backoff == nil {
		return
	}
	base, baseOK := c.lintLifecycleDuration(node.ID, "retry.backoff.base", node.Retry.Backoff.Base)
	maximum, maxOK := c.lintLifecycleDuration(node.ID, "retry.backoff.max", node.Retry.Backoff.Max)
	if baseOK && maxOK && base > maximum {
		c.add(node.ID, CodeDurationInvalid, "retry.backoff.base must not exceed retry.backoff.max")
	}
}

func (c *lintContext) lintLifecycleDuration(nodeID dsl.NodeID, field, raw string) (time.Duration, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}
	duration, err := time.ParseDuration(trimmed)
	if err != nil || duration <= 0 {
		c.add(nodeID, CodeDurationInvalid, "%s must be a positive duration: %q", field, raw)
		return 0, false
	}
	return duration, true
}

func (c *lintContext) lintErrorPolicy(node dsl.Node) {
	if node.OnError == nil {
		return
	}
	if node.OnError.Route != "" && node.OnError.AllowFail {
		c.add(node.ID, CodeErrorRouteConflict, "on_error.route and on_error.allow_fail are mutually exclusive")
	}
	if node.OnError.Route == "" {
		return
	}
	if !containsNodeID(c.adjacency[node.ID], node.OnError.Route) {
		c.add(
			node.ID,
			CodeErrorRouteBackward,
			"on_error.route %q must be a direct forward edge from %q",
			node.OnError.Route,
			node.ID,
		)
	}
	if node.Class == dsl.NodeClassSource && dsl.SourceKind(node.Kind) == dsl.SourceInput {
		c.warn(node.ID, CodeErrorRouteDead, "on_error.route %q is dead on infallible input source", node.OnError.Route)
	}
}

func (c *lintContext) lintResultContract(node dsl.Node) {
	if node.ResultContract == nil {
		return
	}
	failureField := strings.TrimSpace(node.ResultContract.FailureField)
	if failureField == "" {
		c.add(node.ID, CodeResultContractInvalid, "result_contract.failure_field is required")
		return
	}
	schema, ok := c.declaredSchema(node)
	if !ok {
		c.add(node.ID, CodeResultContractInvalid, "result_contract requires a declared output schema")
		return
	}
	namespace := refs.Namespace{Nodes: map[string]refs.NodeSchema{
		string(node.ID): {Output: schema, HasOutput: true},
	}}
	for _, field := range []string{failureField, strings.TrimSpace(node.ResultContract.MessageField)} {
		if field == "" {
			continue
		}
		path := append([]string{namespaceNodesKey, string(node.ID), namespaceOutputKey}, strings.Split(field, ".")...)
		if err := namespace.ValidatePath(path); err != nil {
			c.add(node.ID, CodeResultContractInvalid, "result_contract field %q is invalid: %v", field, err)
		}
	}
}

func (c *lintContext) lintParentClose(node dsl.Node) {
	if node.OnParentClose == "" {
		return
	}
	if node.Class != dsl.NodeClassAction || dsl.ActionKind(node.Kind) != dsl.ActionRunLoop {
		c.add(node.ID, CodeParentCloseInvalid, "on_parent_close is valid only on run-loop nodes")
		return
	}
	if !dsl.IsKnownParentClosePolicy(node.OnParentClose) {
		c.add(node.ID, CodeParentCloseInvalid, "on_parent_close %q is not in the closed enum", node.OnParentClose)
	}
}

func (c *lintContext) lintWatchIdentity(node dsl.Node) {
	if node.Class != dsl.NodeClassSource || dsl.SourceKind(node.Kind) != dsl.SourceWatchSource {
		return
	}
	identities, ok := c.linter.tools.(WatchIdentitySource)
	if !ok {
		return
	}
	kindValue, exists := node.WatchSpec["kind"]
	kind, valid := kindValue.(string)
	if exists && !valid {
		c.add(node.ID, CodeWatchIdentityRequired, "watch source kind must be a string")
		return
	}
	kind = strings.TrimSpace(kind)
	if kind != "" && !identities.SupportsStableWatchIdentity(kind) {
		c.add(node.ID, CodeWatchIdentityRequired, "watch source kind %q does not provide stable event identity", kind)
	}
}

func containsNodeID(values []dsl.NodeID, want dsl.NodeID) bool {
	return slices.Contains(values, want)
}

func nodeEffectLists(node dsl.Node) [][]dsl.EffectSpec {
	lists := [][]dsl.EffectSpec{
		node.OnRetry,
		node.OnSuccess,
		node.OnPause,
		node.OnTimeout,
		node.OnCancel,
		node.OnQuarantine,
	}
	if node.OnError != nil {
		lists = append(lists, node.OnError.Effects)
	}
	return lists
}

func contractEffectLists(contract dsl.Contract) [][]dsl.EffectSpec {
	return [][]dsl.EffectSpec{
		contract.OnDone,
		contract.OnNoOp,
		contract.OnBlocked,
		contract.OnFailed,
		contract.OnExhausted,
		contract.OnStalled,
		contract.OnCanceled,
	}
}
