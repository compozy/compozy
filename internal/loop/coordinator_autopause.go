package loop

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/google/cel-go/cel"
)

const (
	autopauseActorKind = "system"
	autopauseActorID   = "loop-autopause"
)

type autopauseDecision struct {
	matched bool
	ruleID  string
	reason  string
}

func evaluateAutopause(
	node dsl.Node,
	failure ClassifiedFailure,
	attempt int,
	effective EffectiveConfig,
) (autopauseDecision, error) {
	resolved, err := ResolveNodeLifecycleConfig(node, nil, effective.Lifecycle)
	if err != nil {
		return autopauseDecision{}, err
	}
	if len(resolved.Autopause) == 0 {
		return autopauseDecision{}, nil
	}
	family, target := autopauseTarget(node, failure)
	env, err := cel.NewEnv(
		cel.Variable("node_id", cel.StringType),
		cel.Variable("family", cel.StringType),
		cel.Variable("target", cel.StringType),
		cel.Variable("class", cel.StringType),
		cel.Variable("attempt", cel.IntType),
	)
	if err != nil {
		return autopauseDecision{}, fmt.Errorf("loop: create autopause CEL environment: %w", err)
	}
	for index, rule := range resolved.Autopause {
		match := strings.TrimSpace(rule.Match)
		ast, issues := env.Compile(match)
		if issues != nil && issues.Err() != nil {
			return autopauseDecision{}, fmt.Errorf("loop: compile autopause rule %d: %w", index, issues.Err())
		}
		program, err := env.Program(ast, cel.CostLimit(resolved.PredicateCostLimit))
		if err != nil {
			return autopauseDecision{}, fmt.Errorf("loop: build autopause rule %d: %w", index, err)
		}
		value, _, err := program.Eval(map[string]any{
			"node_id": string(node.ID), "family": family, "target": target,
			"class": string(failure.Class), watchEventsPayloadAttempt: int64(attempt),
		})
		if err != nil {
			return autopauseDecision{}, fmt.Errorf("loop: evaluate autopause rule %d: %w", index, err)
		}
		matched, ok := value.Value().(bool)
		if !ok {
			return autopauseDecision{}, fmt.Errorf("%w: autopause rule %d did not return bool", ErrValidation, index)
		}
		if matched {
			return autopauseDecision{
				matched: true,
				ruleID:  autopauseRuleID(index, match),
				reason:  fmt.Sprintf("autopause rule matched %s failure on attempt %d", failure.Class, attempt),
			}, nil
		}
	}
	return autopauseDecision{}, nil
}

func autopauseTarget(node dsl.Node, failure ClassifiedFailure) (string, string) {
	if key, ok := staticTargetHealthKey("", node); ok {
		return targetIdentityFromKey(key)
	}
	family := string(node.Class)
	if family == "" {
		family = "unknown"
	}
	return family, strings.TrimSpace(failure.Target)
}

func autopauseRuleID(index int, match string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(match)))
	return fmt.Sprintf("autopause-%d-%s", index+1, hex.EncodeToString(digest[:6]))
}
