package loop

import (
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"
)

const (
	watchEventsCELAndFunction   = "_&&_"
	watchEventsCELEqualFunction = "_==_"
	watchEventsCELIndexFunction = "_[_]"
)

func watchEventsFilterHasSessionConstraint(raw string) bool {
	_, ok, err := watchEventsSessionIDsFromFilter(raw, nil, false)
	return err == nil && ok
}

func watchEventsSessionIDsFromFilter(
	raw string,
	inputs map[string]any,
	requireResolved bool,
) ([]string, bool, error) {
	expr, err := watchEventsFilterExpr(raw)
	if err != nil {
		return nil, false, err
	}
	values, ok, err := watchEventsSessionIDsFromExpr(expr, inputs, requireResolved)
	if err != nil || !ok {
		return nil, ok, err
	}
	if requireResolved && len(values) == 0 {
		return nil, true, fmt.Errorf("%w: watch-events session_id constraint did not resolve", ErrValidation)
	}
	return values, true, nil
}

func watchEventsFilterExpr(raw string) (celast.Expr, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("%w: watch-events session_id constraint requires a filter", ErrValidation)
	}
	env, err := cel.NewEnv(
		cel.Variable("inputs", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("nodes", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("event", cel.MapType(cel.StringType, cel.DynType)),
	)
	if err != nil {
		return nil, fmt.Errorf("loop: create watch-events constraint CEL env: %w", err)
	}
	ast, issues := env.Compile(trimmed)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("loop: compile watch-events constraint filter: %w", issues.Err())
	}
	return ast.NativeRep().Expr(), nil
}

func watchEventsSessionIDsFromExpr(
	expr celast.Expr,
	inputs map[string]any,
	requireResolved bool,
) ([]string, bool, error) {
	if isWatchEventsEmptyExpr(expr) {
		return nil, false, nil
	}
	if expr.Kind() != celast.CallKind {
		return nil, false, nil
	}
	call := expr.AsCall()
	if call.FunctionName() == watchEventsCELAndFunction {
		for _, arg := range call.Args() {
			values, ok, err := watchEventsSessionIDsFromExpr(arg, inputs, requireResolved)
			if err != nil || ok {
				return values, ok, err
			}
		}
		return nil, false, nil
	}
	if call.FunctionName() != watchEventsCELEqualFunction {
		return nil, false, nil
	}
	args := call.Args()
	if len(args) != 2 {
		return nil, false, nil
	}
	if watchEventsExprIsEventSessionID(args[0]) {
		return watchEventsSessionConstraintValue(args[1], inputs, requireResolved)
	}
	if watchEventsExprIsEventSessionID(args[1]) {
		return watchEventsSessionConstraintValue(args[0], inputs, requireResolved)
	}
	return nil, false, nil
}

func watchEventsSessionConstraintValue(
	expr celast.Expr,
	inputs map[string]any,
	requireResolved bool,
) ([]string, bool, error) {
	if value, ok := watchEventsStringLiteral(expr); ok {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" && requireResolved {
			return nil, true, fmt.Errorf("%w: watch-events session_id constraint is empty", ErrValidation)
		}
		if trimmed == "" {
			return nil, true, nil
		}
		return []string{trimmed}, true, nil
	}
	path, ok := watchEventsExprPath(expr)
	if !ok || len(path) < 2 || path[0] != namespaceInputsKey {
		if requireResolved {
			return nil, true, fmt.Errorf(
				"%w: watch-events session_id constraint must compare event.session_id to a string or inputs path",
				ErrValidation,
			)
		}
		return nil, true, nil
	}
	value, ok := watchEventsLookupInputPath(inputs, path[1:])
	if !ok {
		if requireResolved {
			return nil, true, fmt.Errorf(
				"%w: watch-events session_id input %q is required",
				ErrValidation,
				strings.Join(path[1:], "."),
			)
		}
		return nil, true, nil
	}
	trimmed := strings.TrimSpace(fmt.Sprint(value))
	if trimmed == "" && requireResolved {
		return nil, true, fmt.Errorf("%w: watch-events session_id input is empty", ErrValidation)
	}
	if trimmed == "" {
		return nil, true, nil
	}
	return []string{trimmed}, true, nil
}

func watchEventsLookupInputPath(inputs map[string]any, path []string) (any, bool) {
	if len(path) == 0 {
		return inputs, true
	}
	var current any = inputs
	for _, segment := range path {
		values, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = values[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func watchEventsExprIsEventSessionID(expr celast.Expr) bool {
	path, ok := watchEventsExprPath(expr)
	return ok && len(path) == 2 && path[0] == "event" && path[1] == watchEventsFieldSessionID
}

func watchEventsExprPath(expr celast.Expr) ([]string, bool) {
	if isWatchEventsEmptyExpr(expr) {
		return nil, false
	}
	switch expr.Kind() {
	case celast.IdentKind:
		return []string{expr.AsIdent()}, true
	case celast.SelectKind:
		operand, ok := watchEventsExprPath(expr.AsSelect().Operand())
		if !ok {
			return nil, false
		}
		return append(operand, expr.AsSelect().FieldName()), true
	case celast.CallKind:
		return watchEventsIndexExprPath(expr.AsCall())
	default:
		return nil, false
	}
}

func watchEventsIndexExprPath(call celast.CallExpr) ([]string, bool) {
	if call.FunctionName() != watchEventsCELIndexFunction {
		return nil, false
	}
	args := call.Args()
	operand := call.Target()
	if isWatchEventsEmptyExpr(operand) {
		if len(args) == 0 {
			return nil, false
		}
		operand = args[0]
		args = args[1:]
	}
	if len(args) == 0 {
		return nil, false
	}
	path, ok := watchEventsExprPath(operand)
	if !ok {
		return nil, false
	}
	segment, ok := watchEventsStringLiteral(args[0])
	if !ok {
		return nil, false
	}
	return append(path, segment), true
}

func watchEventsStringLiteral(expr celast.Expr) (string, bool) {
	if isWatchEventsEmptyExpr(expr) || expr.Kind() != celast.LiteralKind {
		return "", false
	}
	value, ok := expr.AsLiteral().Value().(string)
	return value, ok
}

func isWatchEventsEmptyExpr(expr celast.Expr) bool {
	return expr == nil || expr.Kind() == celast.UnspecifiedExprKind
}
