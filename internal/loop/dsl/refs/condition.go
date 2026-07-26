package refs

import (
	"fmt"
	"strings"
	"sync"

	"github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"
)

const defaultCostLimit uint64 = 10000

// Condition is a compiled CEL bool expression plus validated references.
type Condition struct {
	Raw        string
	Program    cel.Program
	References []Reference
}

// ConditionCompiler compiles CEL expressions over the shared namespace.
type ConditionCompiler struct {
	namespace Namespace
	env       *cel.Env
	costLimit uint64
	mu        sync.Mutex
	cache     map[string]*Condition
}

// ConditionCompilerOption configures a condition compiler.
type ConditionCompilerOption func(*ConditionCompiler)

// WithCostLimit overrides the CEL runtime cost limit.
func WithCostLimit(limit uint64) ConditionCompilerOption {
	return func(compiler *ConditionCompiler) {
		compiler.costLimit = limit
	}
}

// NewConditionCompiler builds a CEL environment for the shared namespace.
func NewConditionCompiler(
	namespace Namespace,
	opts ...ConditionCompilerOption,
) (*ConditionCompiler, error) {
	compiler := &ConditionCompiler{
		namespace: namespace,
		costLimit: defaultCostLimit,
		cache:     map[string]*Condition{},
	}
	for _, opt := range opts {
		opt(compiler)
	}

	env, err := cel.NewEnv(
		cel.Variable("inputs", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("nodes", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("item", cel.DynType),
		cel.Variable("index", cel.IntType),
		cel.Variable("trigger", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("generation", cel.IntType),
		cel.Variable("event", cel.MapType(cel.StringType, cel.DynType)),
	)
	if err != nil {
		return nil, fmt.Errorf("create loop CEL environment: %w", err)
	}
	compiler.env = env
	return compiler, nil
}

// Compile parses, validates, checks, and caches a CEL bool expression.
func (c *ConditionCompiler) Compile(raw string) (*Condition, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("condition is required")
	}

	c.mu.Lock()
	if cached, ok := c.cache[trimmed]; ok {
		c.mu.Unlock()
		return cached, nil
	}
	c.mu.Unlock()

	ast, issues := c.env.Compile(trimmed)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("compile CEL condition: %w", issues.Err())
	}
	references, err := c.validateReferences(ast)
	if err != nil {
		return nil, err
	}
	if !cel.BoolType.IsAssignableType(ast.OutputType()) {
		return nil, &Error{
			Code:    CodeConditionNotBool,
			Message: fmt.Sprintf("condition must return bool, got %s", ast.OutputType().String()),
		}
	}
	program, err := c.env.Program(ast, cel.EvalOptions(cel.OptOptimize), cel.CostLimit(c.costLimit))
	if err != nil {
		return nil, fmt.Errorf("create CEL program: %w", err)
	}
	condition := &Condition{
		Raw:        trimmed,
		Program:    program,
		References: references,
	}

	c.mu.Lock()
	c.cache[trimmed] = condition
	c.mu.Unlock()
	return condition, nil
}

func (c *ConditionCompiler) validateReferences(ast *cel.Ast) ([]Reference, error) {
	native := ast.NativeRep()
	seen := map[string]struct{}{}
	references := []Reference{}
	if err := c.walkExpr(native.Expr(), seen, &references); err != nil {
		return nil, err
	}
	return references, nil
}

func (c *ConditionCompiler) walkExpr(
	expression celast.Expr,
	seen map[string]struct{},
	references *[]Reference,
) error {
	if isEmptyExpr(expression) {
		return nil
	}
	if expression.Kind() != celast.IdentKind {
		if path, ok := referencePath(expression); ok {
			return c.addReference(path, seen, references)
		}
	}
	switch expression.Kind() {
	case celast.IdentKind:
		if !isScalarNamespaceIdent(expression.AsIdent()) {
			return nil
		}
		return c.addReference([]string{expression.AsIdent()}, seen, references)
	case celast.SelectKind:
		return c.walkExpr(expression.AsSelect().Operand(), seen, references)
	case celast.CallKind:
		return c.walkCall(expression.AsCall(), seen, references)
	case celast.ListKind:
		return c.walkExprs(expression.AsList().Elements(), seen, references)
	case celast.MapKind:
		return c.walkEntries(expression.AsMap().Entries(), seen, references)
	case celast.StructKind:
		return c.walkEntries(expression.AsStruct().Fields(), seen, references)
	case celast.ComprehensionKind:
		return c.walkComprehension(expression.AsComprehension(), seen, references)
	}
	return nil
}

func (c *ConditionCompiler) walkCall(
	call celast.CallExpr,
	seen map[string]struct{},
	references *[]Reference,
) error {
	if call.IsMemberFunction() && !isEmptyExpr(call.Target()) {
		if err := c.walkExpr(call.Target(), seen, references); err != nil {
			return err
		}
	}
	return c.walkExprs(call.Args(), seen, references)
}

func (c *ConditionCompiler) walkExprs(
	expressions []celast.Expr,
	seen map[string]struct{},
	references *[]Reference,
) error {
	for _, expression := range expressions {
		if err := c.walkExpr(expression, seen, references); err != nil {
			return err
		}
	}
	return nil
}

func (c *ConditionCompiler) walkEntries(
	entries []celast.EntryExpr,
	seen map[string]struct{},
	references *[]Reference,
) error {
	for _, entry := range entries {
		if err := c.walkEntry(entry, seen, references); err != nil {
			return err
		}
	}
	return nil
}

func (c *ConditionCompiler) walkComprehension(
	comprehension celast.ComprehensionExpr,
	seen map[string]struct{},
	references *[]Reference,
) error {
	return c.walkExprs(
		[]celast.Expr{
			comprehension.IterRange(),
			comprehension.AccuInit(),
			comprehension.LoopCondition(),
			comprehension.LoopStep(),
			comprehension.Result(),
		},
		seen,
		references,
	)
}

func (c *ConditionCompiler) walkEntry(
	entry celast.EntryExpr,
	seen map[string]struct{},
	references *[]Reference,
) error {
	switch entry.Kind() {
	case celast.MapEntryKind:
		mapEntry := entry.AsMapEntry()
		if err := c.walkExpr(mapEntry.Key(), seen, references); err != nil {
			return err
		}
		if err := c.walkExpr(mapEntry.Value(), seen, references); err != nil {
			return err
		}
	case celast.StructFieldKind:
		if err := c.walkExpr(entry.AsStructField().Value(), seen, references); err != nil {
			return err
		}
	}
	return nil
}

func (c *ConditionCompiler) addReference(
	path []string,
	seen map[string]struct{},
	references *[]Reference,
) error {
	raw := strings.Join(path, ".")
	if _, ok := seen[raw]; ok {
		return nil
	}
	seen[raw] = struct{}{}
	if err := c.namespace.ValidatePath(path); err != nil {
		return err
	}
	*references = append(*references, Reference{Raw: raw, Path: append([]string(nil), path...)})
	return nil
}

func referencePath(expression celast.Expr) ([]string, bool) {
	if isEmptyExpr(expression) {
		return nil, false
	}
	switch expression.Kind() {
	case celast.IdentKind:
		root := expression.AsIdent()
		if isDottedNamespaceRoot(root) {
			return []string{root}, true
		}
		return nil, false
	case celast.SelectKind:
		selection := expression.AsSelect()
		path, ok := referencePath(selection.Operand())
		if !ok {
			return nil, false
		}
		return append(path, selection.FieldName()), true
	case celast.CallKind:
		return indexReferencePath(expression.AsCall())
	default:
		return nil, false
	}
}

func indexReferencePath(call celast.CallExpr) ([]string, bool) {
	if call.FunctionName() != "_[_]" {
		return nil, false
	}
	args := call.Args()
	operand := call.Target()
	if isEmptyExpr(operand) {
		operand = nil
	}
	var index celast.Expr
	if operand != nil {
		if len(args) > 0 {
			index = args[0]
		}
	} else {
		if len(args) == 0 {
			return nil, false
		}
		operand = args[0]
		if len(args) > 1 {
			index = args[1]
		}
	}
	path, ok := referencePath(operand)
	if !ok {
		return nil, false
	}
	if segment, ok := stringLiteral(index); ok {
		path = append(path, segment)
	}
	return path, true
}

func stringLiteral(expression celast.Expr) (string, bool) {
	if isEmptyExpr(expression) || expression.Kind() != celast.LiteralKind {
		return "", false
	}
	value, ok := expression.AsLiteral().Value().(string)
	return value, ok
}

func isEmptyExpr(expression celast.Expr) bool {
	return expression == nil || expression.Kind() == celast.UnspecifiedExprKind
}

func isDottedNamespaceRoot(name string) bool {
	switch name {
	case namespaceInputs, namespaceNodes, namespaceTrigger, namespaceItem, namespaceIndex,
		namespaceGeneration, namespaceEvent:
		return true
	default:
		return false
	}
}

func isScalarNamespaceIdent(name string) bool {
	switch name {
	case namespaceItem, namespaceIndex, namespaceGeneration:
		return true
	default:
		return false
	}
}
