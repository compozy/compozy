package daemon

import (
	"fmt"
	"strings"
	"sync"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"
)

const watchEventsDoorbellCostLimit uint64 = 10000

type watchEventsDoorbellCondition struct {
	program    cel.Program
	nodeScoped bool
}

type watchEventsDoorbellMatcher interface {
	Match(raw string, inputs map[string]any, event looppkg.WatchEvent) bool
}

type watchEventsDoorbellCompiler struct {
	env   *cel.Env
	mu    sync.Mutex
	cache map[string]*watchEventsDoorbellCondition
}

func newWatchEventsDoorbellCompiler() (*watchEventsDoorbellCompiler, error) {
	env, err := cel.NewEnv(
		cel.Variable("inputs", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("nodes", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("event", cel.MapType(cel.StringType, cel.DynType)),
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: create watch-events doorbell CEL env: %w", err)
	}
	return &watchEventsDoorbellCompiler{
		env:   env,
		cache: map[string]*watchEventsDoorbellCondition{},
	}, nil
}

func (c *watchEventsDoorbellCompiler) Match(
	raw string,
	inputs map[string]any,
	event looppkg.WatchEvent,
) bool {
	condition, ok := c.condition(raw)
	if !ok || condition.nodeScoped {
		return true
	}
	value, _, err := condition.program.Eval(map[string]any{
		"inputs": inputs,
		"nodes":  map[string]any{},
		"event":  event.EventMap(),
	})
	if err != nil {
		return true
	}
	matched, ok := value.Value().(bool)
	return ok && matched
}

func (c *watchEventsDoorbellCompiler) condition(raw string) (*watchEventsDoorbellCondition, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return &watchEventsDoorbellCondition{nodeScoped: true}, true
	}
	c.mu.Lock()
	if cached, ok := c.cache[trimmed]; ok {
		c.mu.Unlock()
		return cached, true
	}
	c.mu.Unlock()

	ast, issues := c.env.Compile(trimmed)
	if issues != nil && issues.Err() != nil {
		return nil, false
	}
	if !cel.BoolType.IsAssignableType(ast.OutputType()) {
		return nil, false
	}
	program, err := c.env.Program(ast, cel.EvalOptions(cel.OptOptimize), cel.CostLimit(watchEventsDoorbellCostLimit))
	if err != nil {
		return nil, false
	}
	condition := &watchEventsDoorbellCondition{
		program:    program,
		nodeScoped: watchEventsExprReferencesIdent(ast.NativeRep().Expr(), "nodes"),
	}
	c.mu.Lock()
	c.cache[trimmed] = condition
	c.mu.Unlock()
	return condition, true
}

func watchEventsExprReferencesIdent(expression celast.Expr, ident string) bool {
	if expression.ID() == 0 && expression.Kind() == celast.UnspecifiedExprKind {
		return false
	}
	switch expression.Kind() {
	case celast.IdentKind:
		return expression.AsIdent() == ident
	case celast.SelectKind:
		return watchEventsExprReferencesIdent(expression.AsSelect().Operand(), ident)
	case celast.CallKind:
		return watchEventsCallReferencesIdent(expression.AsCall(), ident)
	case celast.ListKind:
		return watchEventsExprsReferenceIdent(expression.AsList().Elements(), ident)
	case celast.MapKind:
		return watchEventsEntriesReferenceIdent(expression.AsMap().Entries(), ident)
	case celast.StructKind:
		return watchEventsEntriesReferenceIdent(expression.AsStruct().Fields(), ident)
	case celast.ComprehensionKind:
		comprehension := expression.AsComprehension()
		return watchEventsExprsReferenceIdent([]celast.Expr{
			comprehension.IterRange(),
			comprehension.AccuInit(),
			comprehension.LoopCondition(),
			comprehension.LoopStep(),
			comprehension.Result(),
		}, ident)
	default:
		return false
	}
}

func watchEventsCallReferencesIdent(call celast.CallExpr, ident string) bool {
	if call.IsMemberFunction() && watchEventsExprReferencesIdent(call.Target(), ident) {
		return true
	}
	return watchEventsExprsReferenceIdent(call.Args(), ident)
}

func watchEventsExprsReferenceIdent(expressions []celast.Expr, ident string) bool {
	for _, expression := range expressions {
		if watchEventsExprReferencesIdent(expression, ident) {
			return true
		}
	}
	return false
}

func watchEventsEntriesReferenceIdent(entries []celast.EntryExpr, ident string) bool {
	for _, entry := range entries {
		switch entry.Kind() {
		case celast.MapEntryKind:
			mapEntry := entry.AsMapEntry()
			if watchEventsExprReferencesIdent(mapEntry.Key(), ident) ||
				watchEventsExprReferencesIdent(mapEntry.Value(), ident) {
				return true
			}
		case celast.StructFieldKind:
			if watchEventsExprReferencesIdent(entry.AsStructField().Value(), ident) {
				return true
			}
		}
	}
	return false
}
