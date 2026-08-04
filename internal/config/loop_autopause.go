package config

import (
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
)

const (
	loopAutopauseActionPause     = "pause"
	loopAutopauseRuleInvalidCode = "autopause_rule_invalid"
)

func validateLoopAutopauseRules(path string, rules []LoopAutopauseRule) error {
	env, err := cel.NewEnv(
		cel.Variable("node_id", cel.StringType),
		cel.Variable("family", cel.StringType),
		cel.Variable("target", cel.StringType),
		cel.Variable("class", cel.StringType),
		cel.Variable("attempt", cel.IntType),
	)
	if err != nil {
		return fmt.Errorf("create loop autopause CEL environment: %w", err)
	}
	for index, rule := range rules {
		rulePath := fmt.Sprintf("%s[%d]", path, index)
		match := strings.TrimSpace(rule.Match)
		if match == "" {
			return loopAutopauseValidationError(rulePath+".match", "must be a non-empty CEL expression")
		}
		ast, issues := env.Compile(match)
		if issues != nil && issues.Err() != nil {
			return loopAutopauseValidationError(rulePath+".match", issues.Err().Error())
		}
		if !cel.BoolType.IsAssignableType(ast.OutputType()) {
			return loopAutopauseValidationError(rulePath+".match", "must return bool")
		}
		if strings.TrimSpace(rule.Action) != loopAutopauseActionPause {
			return loopAutopauseValidationError(rulePath+".action", "must be pause")
		}
	}
	return nil
}

func loopAutopauseValidationError(path, message string) ValidationError {
	return ValidationError{Code: loopAutopauseRuleInvalidCode, Path: path, Message: message}
}
