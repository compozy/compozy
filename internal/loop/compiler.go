package loop

import (
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/dsl/refs"
)

// Compiler compiles authoring definitions into runtime-ready resolved form.
type Compiler struct {
	tools  ToolSchemaSource
	linter *DefinitionLinter
}

// CompilerOption configures a compiler.
type CompilerOption func(*Compiler)

// WithCompilerToolSchemaSource injects the pure schema source used during compile.
func WithCompilerToolSchemaSource(source ToolSchemaSource) CompilerOption {
	return func(compiler *Compiler) {
		compiler.tools = source
		compiler.linter = NewLinter(WithToolSchemaSource(source))
	}
}

// NewCompiler creates a resolved-form compiler.
func NewCompiler(opts ...CompilerOption) *Compiler {
	compiler := &Compiler{linter: NewLinter()}
	for _, opt := range opts {
		opt(compiler)
	}
	return compiler
}

// LintFailedError carries blocking lint diagnostics from compile.
type LintFailedError struct {
	Errors []LintError
}

func (e *LintFailedError) Error() string {
	if len(e.Errors) == 0 {
		return "loop definition has 0 lint error(s)"
	}
	codes := make([]string, 0, len(e.Errors))
	for _, lintError := range e.Errors {
		if !slices.Contains(codes, lintError.Code) {
			codes = append(codes, lintError.Code)
		}
	}
	return fmt.Sprintf(
		"loop definition has %d lint error(s): %s",
		len(e.Errors),
		strings.Join(codes, ", "),
	)
}

// ResolvedDefinition is the publish-time artifact hydrated by runtime execution.
type ResolvedDefinition struct {
	Definition           dsl.Definition
	DefinitionVersion    int
	Templates            map[string]*refs.Template
	Conditions           map[string]*refs.Condition
	ToolSchemas          map[string]ToolSchemaSnapshot
	WatchEventsContracts map[hooks.HookEvent]WatchEventsContract
	Defaults             ResolvedDefaults
	EffectiveConfig      EffectiveConfig
	compiled             bool
}

// ResolvedDefaults records defaults folded at compile time.
type ResolvedDefaults struct {
	FanOutBatchSize int                   `json:"fan_out_batch_size"`
	RunLoopMode     dsl.RunLoopMode       `json:"run_loop_mode"`
	Concurrency     dsl.ConcurrencyPolicy `json:"concurrency"`
}

// Compile lints, parses templates/CEL, snapshots tool schemas, and folds defaults.
func (c *Compiler) Compile(def dsl.Definition) (*ResolvedDefinition, error) {
	def.Normalize()
	lintErrors := c.linter.Lint(def)
	blockingErrors := blockingLintErrors(lintErrors)
	if len(blockingErrors) > 0 {
		return nil, &LintFailedError{Errors: blockingErrors}
	}
	if err := normalizeDefinitionParticipation(&def); err != nil {
		return nil, fmt.Errorf("normalize Loop definition participation: %w", err)
	}

	ctx := newLintContext(def, &DefinitionLinter{tools: c.tools})
	ctx.indexGraphTrusted()

	resolved := &ResolvedDefinition{
		Definition:  foldDefinitionDefaults(def),
		Templates:   map[string]*refs.Template{},
		Conditions:  map[string]*refs.Condition{},
		ToolSchemas: map[string]ToolSchemaSnapshot{},
		WatchEventsContracts: referencedWatchEventsContracts(
			def,
			SupportedWatchEvents(),
		),
		Defaults: ResolvedDefaults{
			FanOutBatchSize: 1,
			RunLoopMode:     dsl.RunLoopAwait,
			Concurrency:     def.Concurrency,
		},
		compiled: true,
	}
	resolved.DefinitionVersion = resolved.Definition.Meta.Version

	if err := compileContract(resolved, def, ctx, ctx.namespace(false, false)); err != nil {
		return nil, err
	}
	if err := compileStarts(resolved, def, ctx); err != nil {
		return nil, err
	}
	for _, node := range def.Graph.Nodes {
		namespace := ctx.namespace(ctx.inFanoutScope(node.ID), ctx.hasTriggerStart())
		if err := compileNode(resolved, node, namespace, ctx); err != nil {
			return nil, err
		}
		if node.Class == dsl.NodeClassAction && !dsl.IsReservedActionKind(node.Kind) &&
			c.tools != nil {
			if snapshot, ok := c.tools.Snapshot(node.Kind); ok {
				resolved.ToolSchemas[node.Kind] = snapshot
			}
		}
	}
	if err := compileSubLoopBodies(resolved, def, c.tools); err != nil {
		return nil, err
	}
	return resolved, nil
}

func blockingLintErrors(errors []LintError) []LintError {
	blocking := make([]LintError, 0, len(errors))
	for _, lintError := range errors {
		if lintError.Severity == SeverityError {
			blocking = append(blocking, lintError)
		}
	}
	return blocking
}

func foldDefinitionDefaults(def dsl.Definition) dsl.Definition {
	def.Graph.Nodes = append([]dsl.Node(nil), def.Graph.Nodes...)
	def.Graph.Edges = append([]dsl.Edge(nil), def.Graph.Edges...)
	foldGraphNodeDefaults(def.Graph.Nodes)
	return def
}

func foldGraphNodeDefaults(nodes []dsl.Node) {
	for idx := range nodes {
		node := &nodes[idx]
		node.Params = cloneNodeParams(node.Params)
		if node.Class == dsl.NodeClassControl && dsl.ControlKind(node.Kind) == dsl.ControlFanOut {
			if node.BatchSize == 0 {
				node.BatchSize = 1
			}
		}
		if node.Class == dsl.NodeClassAction && dsl.ActionKind(node.Kind) == dsl.ActionRunLoop {
			var params dsl.RunLoopParams
			if err := node.Params.Decode(&params); err == nil && params.Mode == "" {
				if node.Params == nil {
					node.Params = dsl.NodeParams{}
				}
				node.Params["mode"] = string(dsl.RunLoopAwait)
			}
		}
		if node.Class == dsl.NodeClassAction && dsl.ActionKind(node.Kind) == dsl.ActionGoal {
			if node.Session == nil {
				node.Session = &dsl.SessionSpec{Mode: dsl.SessionModeContinuous}
			}
			var params dsl.GoalParams
			if err := node.Params.Decode(&params); err == nil && params.OnExhausted == "" {
				if node.Params == nil {
					node.Params = dsl.NodeParams{}
				}
				node.Params["on_exhausted"] = dsl.GoalOnExhaustedHalt
			}
		}
		if node.Body != nil {
			body := *node.Body
			body.Nodes = append([]dsl.Node(nil), body.Nodes...)
			body.Edges = append([]dsl.Edge(nil), body.Edges...)
			foldGraphNodeDefaults(body.Nodes)
			node.Body = &body
		}
	}
}

func cloneNodeParams(params dsl.NodeParams) dsl.NodeParams {
	if params == nil {
		return nil
	}
	cloned := make(dsl.NodeParams, len(params))
	for key, value := range params {
		cloned[key] = cloneAny(value)
	}
	return cloned
}

func cloneAny(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		cloned := make(map[string]any, len(typed))
		for key, child := range typed {
			cloned[key] = cloneAny(child)
		}
		return cloned
	case map[any]any:
		cloned := make(map[any]any, len(typed))
		for key, child := range typed {
			cloned[key] = cloneAny(child)
		}
		return cloned
	case []any:
		cloned := make([]any, 0, len(typed))
		for _, child := range typed {
			cloned = append(cloned, cloneAny(child))
		}
		return cloned
	default:
		return value
	}
}

func compileContract(
	resolved *ResolvedDefinition,
	def dsl.Definition,
	ctx *lintContext,
	namespace refs.Namespace,
) error {
	if strings.TrimSpace(def.Contract.StopWhen) == "" {
		return compileContractVerification(resolved, def, namespace)
	}
	condition, err := ctx.compileCondition(def.Contract.StopWhen, namespace)
	if err != nil {
		return fmt.Errorf("compile contract.stop_when: %w", err)
	}
	resolved.Conditions[contractStopWhenConditionKey] = condition
	return compileContractVerification(resolved, def, namespace)
}

func compileContractVerification(
	resolved *ResolvedDefinition,
	def dsl.Definition,
	namespace refs.Namespace,
) error {
	for idx, criterion := range def.Contract.Verification {
		prefix := fmt.Sprintf("contract.verification[%d]", idx)
		for _, item := range criterionStringFields(prefix, criterion) {
			if strings.TrimSpace(item.value) == "" {
				continue
			}
			template, err := refs.CompileTemplate(item.name, item.value, namespace)
			if err != nil {
				return fmt.Errorf("compile %s: %w", item.name, err)
			}
			resolved.Templates[item.name] = template
		}
	}
	return nil
}

func compileStarts(resolved *ResolvedDefinition, def dsl.Definition, ctx *lintContext) error {
	for idx, start := range def.Start {
		namespace := ctx.namespace(false, startAllowsTrigger(start.Kind))
		for input, raw := range start.InputMapping {
			key := fmt.Sprintf("start[%d].input_mapping.%s", idx, input)
			template, err := refs.CompileTemplate(key, raw, namespace)
			if err != nil {
				return fmt.Errorf("compile %s: %w", key, err)
			}
			resolved.Templates[key] = template
		}
	}
	return nil
}

func compileNode(
	resolved *ResolvedDefinition,
	node dsl.Node,
	namespace refs.Namespace,
	ctx *lintContext,
) error {
	for _, item := range nodeStringFields(node) {
		if strings.TrimSpace(item.value) == "" {
			continue
		}
		key := fmt.Sprintf("nodes.%s.%s", node.ID, item.name)
		template, err := refs.CompileTemplate(key, item.value, namespace)
		if err != nil {
			return fmt.Errorf("compile %s: %w", key, err)
		}
		resolved.Templates[key] = template
	}
	for _, item := range nodeConditionFields(node) {
		if strings.TrimSpace(item.value) == "" {
			continue
		}
		key := fmt.Sprintf("nodes.%s.%s", node.ID, item.name)
		condition, err := ctx.compileCondition(item.value, namespace)
		if err != nil {
			return fmt.Errorf("compile %s: %w", key, err)
		}
		resolved.Conditions[key] = condition
	}
	if err := compileWatchEventsFilters(resolved, node, namespace, ctx); err != nil {
		return err
	}
	return nil
}

func compileWatchEventsFilters(
	resolved *ResolvedDefinition,
	node dsl.Node,
	namespace refs.Namespace,
	ctx *lintContext,
) error {
	if node.Class != dsl.NodeClassSource || dsl.SourceKind(node.Kind) != dsl.SourceWatchEvents {
		return nil
	}
	eventNamespace := namespaceWithEvent(namespace)
	for idx, subscription := range node.Events {
		if strings.TrimSpace(subscription.Filter) == "" {
			continue
		}
		key := fmt.Sprintf("nodes.%s.events.%d.filter", node.ID, idx)
		condition, err := ctx.compileCondition(subscription.Filter, eventNamespace)
		if err != nil {
			return fmt.Errorf("compile %s: %w", key, err)
		}
		resolved.Conditions[key] = condition
	}
	return nil
}
