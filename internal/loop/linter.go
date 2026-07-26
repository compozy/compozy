package loop

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/dsl/refs"
)

var nodeIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// LinterOption configures the default linter.
type LinterOption func(*DefinitionLinter)

// WithToolSchemaSource injects the pure schema snapshot source for open ToolID actions.
func WithToolSchemaSource(source ToolSchemaSource) LinterOption {
	return func(linter *DefinitionLinter) {
		linter.tools = source
	}
}

// DefinitionLinter is the shared structural and reference linter.
type DefinitionLinter struct {
	tools ToolSchemaSource
}

var _ Linter = (*DefinitionLinter)(nil)

// NewLinter creates the shared loop linter.
func NewLinter(opts ...LinterOption) *DefinitionLinter {
	linter := &DefinitionLinter{}
	for _, opt := range opts {
		opt(linter)
	}
	return linter
}

// Lint validates one loop definition.
func (l *DefinitionLinter) Lint(def dsl.Definition) []LintError {
	def.Normalize()
	ctx := newLintContext(def, l)
	ctx.indexGraph()
	ctx.lintContractShape()
	ctx.lintNetworkParticipation()
	ctx.lintNodeIDs()
	ctx.lintKindsAndSchemas()
	ctx.lintGraphShape()
	ctx.lintReferences()
	return ctx.errors
}

type conditionCompilerKey struct {
	allowFanout  bool
	allowTrigger bool
	allowEvent   bool
}

type lintContext struct {
	def                dsl.Definition
	linter             *DefinitionLinter
	errors             []LintError
	nodeByID           map[dsl.NodeID]dsl.Node
	adjacency          map[dsl.NodeID][]dsl.NodeID
	reverse            map[dsl.NodeID][]dsl.NodeID
	conditionCompilers map[conditionCompilerKey]*refs.ConditionCompiler
	hasCycle           bool
}

func newLintContext(def dsl.Definition, linter *DefinitionLinter) *lintContext {
	return &lintContext{
		def:                def,
		linter:             linter,
		errors:             []LintError{},
		nodeByID:           map[dsl.NodeID]dsl.Node{},
		adjacency:          map[dsl.NodeID][]dsl.NodeID{},
		reverse:            map[dsl.NodeID][]dsl.NodeID{},
		conditionCompilers: map[conditionCompilerKey]*refs.ConditionCompiler{},
	}
}

func (c *lintContext) indexGraph() {
	c.indexGraphWithDiagnostics(true)
}

func (c *lintContext) indexGraphTrusted() {
	c.indexGraphWithDiagnostics(false)
}

func (c *lintContext) indexGraphWithDiagnostics(reportDiagnostics bool) {
	for _, node := range c.def.Graph.Nodes {
		if _, exists := c.nodeByID[node.ID]; exists {
			if reportDiagnostics {
				c.add(node.ID, CodeDuplicateNodeID, "node id %q is duplicated", node.ID)
			}
			continue
		}
		c.nodeByID[node.ID] = node
	}
	for _, edge := range c.def.Graph.Edges {
		c.adjacency[edge.From] = append(c.adjacency[edge.From], edge.To)
		c.reverse[edge.To] = append(c.reverse[edge.To], edge.From)
		if _, ok := c.nodeByID[edge.From]; !ok {
			if reportDiagnostics {
				c.add(
					edge.From,
					refs.CodeUnknownReference,
					"edge references unknown from node %q",
					edge.From,
				)
			}
		}
		if _, ok := c.nodeByID[edge.To]; !ok {
			if reportDiagnostics {
				c.add(
					edge.To,
					refs.CodeUnknownReference,
					"edge references unknown to node %q",
					edge.To,
				)
			}
		}
	}
}

func (c *lintContext) lintContractShape() {
	for _, state := range c.def.Contract.TerminalStates {
		if !dsl.IsKnownTerminalState(state) {
			c.add("", CodeUnknownTerminalState, "terminal state %q is not in the closed enum", state)
		}
	}
}

func (c *lintContext) lintNodeIDs() {
	for _, node := range c.def.Graph.Nodes {
		if !nodeIDPattern.MatchString(string(node.ID)) {
			c.add(node.ID, CodeNodeIDInvalid, "node id %q must match ^[a-z][a-z0-9_]*$", node.ID)
		}
		if node.ID == BudgetGateID {
			c.add(node.ID, CodeNodeIDInvalid, "node id %q is reserved for budget approvals", node.ID)
		}
	}
}

func (c *lintContext) lintKindsAndSchemas() {
	for _, node := range c.def.Graph.Nodes {
		c.lintWatchEventsEnvelopeShape(node)
		switch node.Class {
		case dsl.NodeClassAction:
			c.lintActionNode(node)
		case dsl.NodeClassControl:
			c.lintControlNode(node)
		case dsl.NodeClassSource:
			c.lintSourceNode(node)
		default:
			c.add(node.ID, refs.CodeUnknownReference, "node class %q is not supported", node.Class)
		}
	}
	c.lintContinuousGoalHandles()
}

func (c *lintContext) lintActionNode(node dsl.Node) {
	c.lintActionHarvest(node)
	if dsl.IsReservedActionKind(node.Kind) {
		c.lintReservedActionNode(node)
		return
	}
	if c.linter.tools == nil {
		c.add(
			node.ID,
			CodeUnknownActionKind,
			"action ToolID %q cannot be resolved without a schema snapshot source",
			node.Kind,
		)
		return
	}
	if _, ok := c.linter.tools.Snapshot(node.Kind); !ok {
		c.add(node.ID, CodeUnknownActionKind, "action ToolID %q is not resolvable", node.Kind)
	}
}

func (c *lintContext) lintControlNode(node dsl.Node) {
	if !dsl.IsKnownControlKind(node.Kind) {
		c.add(
			node.ID,
			CodeUnknownControlKind,
			"control kind %q is not in the closed enum",
			node.Kind,
		)
		return
	}
	switch dsl.ControlKind(node.Kind) {
	case dsl.ControlFanOut:
		c.lintFanOut(node)
	case dsl.ControlCollect:
		return
	case dsl.ControlBranch:
		if strings.TrimSpace(node.Condition) == "" {
			c.add(node.ID, refs.CodeConditionNotBool, "branch condition is required")
		}
	case dsl.ControlGate:
		c.lintGate(node)
	case dsl.ControlSubLoop:
		if node.Body == nil || len(node.Body.Nodes) == 0 {
			c.add(node.ID, CodeNonTerminatingStructure, "sub-loop must declare a nested body")
			return
		}
		c.lintSubLoopBody(node)
	}
}

func (c *lintContext) lintFanOut(node dsl.Node) {
	if strings.TrimSpace(node.Collection) == "" || node.MaxFanOut <= 0 {
		c.add(node.ID, CodeFanOutUnbounded, "fan-out must declare collection and max_fan_out")
	}
	if node.MaxFanOut > LoopMaxFanoutWidth {
		c.add(
			node.ID,
			CodeFanOutCeilingExceeded,
			"fan-out max_fan_out %d exceeds ceiling %d",
			node.MaxFanOut,
			LoopMaxFanoutWidth,
		)
	}
	if node.MaxParallel > LoopMaxFanoutWidth {
		c.add(
			node.ID,
			CodeFanOutCeilingExceeded,
			"fan-out max_parallel %d exceeds ceiling %d",
			node.MaxParallel,
			LoopMaxFanoutWidth,
		)
	}
}

func (c *lintContext) lintGate(node dsl.Node) {
	if node.MaxRevisions > LoopMaxGateRevisions {
		c.add(
			node.ID,
			CodeGateMaxRevisionsCeilingExceeded,
			"gate max_revisions %d exceeds ceiling %d",
			node.MaxRevisions,
			LoopMaxGateRevisions,
		)
	}
	if node.VerdictPolicy == dsl.VerdictPolicyReviseUntilClean &&
		!hasJudgeCriterion(node.Criteria) {
		c.add(
			node.ID,
			CodeVerdictPolicyRequiresJudge,
			"verdict_policy revise_until_clean requires an agent-judge or human criterion",
		)
	}
}

func hasJudgeCriterion(criteria []dsl.GateCriterion) bool {
	return slices.ContainsFunc(criteria, func(criterion dsl.GateCriterion) bool {
		return criterion.Type == dsl.CriterionAgentJudge || criterion.Type == dsl.CriterionHuman
	})
}

func (c *lintContext) lintSourceNode(node dsl.Node) {
	if !dsl.IsKnownSourceKind(node.Kind) {
		c.add(node.ID, CodeUnknownSourceKind, "source kind %q is not in the closed enum", node.Kind)
		return
	}
	switch dsl.SourceKind(node.Kind) {
	case dsl.SourceInput:
		if strings.TrimSpace(node.InputRef) == "" {
			c.add(node.ID, refs.CodeUnknownReference, "input source must declare input_ref")
			return
		}
		if _, ok := c.def.Inputs[node.InputRef]; !ok {
			c.add(
				node.ID,
				refs.CodeUnknownReference,
				"input_ref %q does not name a declared input",
				node.InputRef,
			)
		}
	case dsl.SourceFileImport:
		if strings.TrimSpace(node.Pattern) == "" {
			c.add(node.ID, refs.CodeUnresolvablePath, "file-import pattern is required")
		}
		if node.Parse != dsl.FileParseJSON && node.Parse != dsl.FileParseText {
			c.add(
				node.ID,
				CodeFileImportParseRequired,
				fileImportParseRequiredMessage,
			)
		}
	case dsl.SourceWatchSource:
		kind, ok := node.WatchSpec["kind"].(string)
		if !ok || strings.TrimSpace(kind) == "" {
			c.add(node.ID, CodeWatchKindRequired, "watch-source must declare watch.kind")
		}
		return
	case dsl.SourceWatchEvents:
		c.lintWatchEventsNode(node)
		return
	}
}

func (c *lintContext) lintGraphShape() {
	c.detectCycles()
	c.detectUnreachable()
	c.detectNonTerminating()
}

func (c *lintContext) detectCycles() {
	state := map[dsl.NodeID]int{}
	var visit func(dsl.NodeID)
	visit = func(id dsl.NodeID) {
		switch state[id] {
		case 1:
			c.hasCycle = true
			c.add(id, CodeCycle, "cycle detected at node %q", id)
			return
		case 2:
			return
		}
		state[id] = 1
		for _, next := range c.adjacency[id] {
			if _, ok := c.nodeByID[next]; ok {
				visit(next)
			}
		}
		state[id] = 2
	}
	for id := range c.nodeByID {
		visit(id)
	}
}

func (c *lintContext) detectUnreachable() {
	if len(c.nodeByID) == 0 {
		return
	}
	roots := c.sourceRoots()
	visited := map[dsl.NodeID]struct{}{}
	queue := append([]dsl.NodeID(nil), roots...)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if _, ok := visited[current]; ok {
			continue
		}
		visited[current] = struct{}{}
		queue = append(queue, c.adjacency[current]...)
	}
	for id := range c.nodeByID {
		if _, ok := visited[id]; !ok {
			c.add(id, CodeUnreachableNode, "node %q is unreachable from source roots", id)
		}
	}
}

func (c *lintContext) sourceRoots() []dsl.NodeID {
	roots := []dsl.NodeID{}
	for _, node := range c.def.Graph.Nodes {
		if node.Class == dsl.NodeClassSource {
			roots = append(roots, node.ID)
		}
	}
	if len(roots) > 0 {
		return roots
	}
	if len(c.def.Graph.Edges) == 0 {
		for _, node := range c.def.Graph.Nodes {
			roots = append(roots, node.ID)
		}
		return roots
	}
	for id := range c.nodeByID {
		if len(c.reverse[id]) == 0 {
			roots = append(roots, id)
		}
	}
	return roots
}

func (c *lintContext) detectNonTerminating() {
	if len(c.nodeByID) == 0 {
		c.add("", CodeNonTerminatingStructure, "graph must declare at least one node")
		return
	}
	if c.hasCycle {
		return
	}
	for id := range c.nodeByID {
		if len(c.adjacency[id]) == 0 {
			return
		}
	}
	c.add("", CodeNonTerminatingStructure, "graph must contain at least one terminal node")
}

func (c *lintContext) lintReferences() {
	base := c.namespace(false, false)
	c.lintContractReferences(base)
	c.lintStartReferences()
	for _, node := range c.def.Graph.Nodes {
		namespace := c.namespace(c.inFanoutScope(node.ID), c.hasTriggerStart())
		c.lintNodeReferences(node, namespace)
	}
}

func (c *lintContext) hasTriggerStart() bool {
	for _, start := range c.def.Start {
		switch start.Kind {
		case dsl.StartTrigger, dsl.StartSchedule, dsl.StartWebhook:
			return true
		default:
			continue
		}
	}
	return false
}

func (c *lintContext) addRefsError(nodeID dsl.NodeID, err error) {
	if refErr, ok := errors.AsType[*refs.Error](err); ok {
		c.errors = append(c.errors, LintError{
			NodeID:   nodeID,
			Code:     refErr.Code,
			Message:  refErr.Error(),
			Severity: SeverityError,
		})
		return
	}
	c.errors = append(c.errors, LintError{
		NodeID:   nodeID,
		Code:     refs.CodeUnresolvablePath,
		Message:  err.Error(),
		Severity: SeverityError,
	})
}

func (c *lintContext) add(nodeID dsl.NodeID, code string, format string, args ...any) {
	c.errors = append(c.errors, LintError{
		NodeID:   nodeID,
		Code:     code,
		Message:  fmt.Sprintf(format, args...),
		Severity: SeverityError,
	})
}

func (c *lintContext) warn(nodeID dsl.NodeID, code string, format string, args ...any) {
	c.errors = append(c.errors, LintError{
		NodeID:   nodeID,
		Code:     code,
		Message:  fmt.Sprintf(format, args...),
		Severity: SeverityWarning,
	})
}
