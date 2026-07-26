package model

import (
	"errors"
	"fmt"
	"strings"
	"text/template"
	"text/template/parse"
)

var errTriggerPromptTemplateRequired = errors.New("trigger prompt template is required")

// ParseTriggerPromptTemplate parses a trigger prompt template with strict activation-envelope validation.
func ParseTriggerPromptTemplate(prompt string) (*template.Template, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, errTriggerPromptTemplateRequired
	}

	tmpl, err := template.New("trigger_prompt").Option("missingkey=error").Parse(prompt)
	if err != nil {
		return nil, fmt.Errorf("parse trigger prompt template: %w", err)
	}

	for _, subtemplate := range tmpl.Templates() {
		if subtemplate.Tree == nil || subtemplate.Root == nil {
			continue
		}
		if err := validateTemplateNode(subtemplate.Root); err != nil {
			return nil, fmt.Errorf("validate trigger prompt template %q: %w", subtemplate.Name(), err)
		}
	}

	return tmpl, nil
}

// ValidateTriggerPromptTemplate validates a trigger prompt template against the normalized activation-envelope model.
func ValidateTriggerPromptTemplate(prompt string) error {
	if strings.TrimSpace(prompt) == "" {
		return fmt.Errorf("validate trigger prompt template: %w", errTriggerPromptTemplateRequired)
	}
	if !strings.Contains(prompt, "{{") && !strings.Contains(prompt, "}}") {
		return nil
	}
	if _, err := ParseTriggerPromptTemplate(prompt); err != nil {
		return fmt.Errorf("validate trigger prompt template: %w", err)
	}
	return nil
}

func validateTemplateNode(node parse.Node) error {
	return validateTemplateNodeWithState(node, templateValidationState{dotKnown: true})
}

type templateValidationState struct {
	dotPath  []string
	dotKnown bool
}

func validateTemplateNodeWithState(node parse.Node, state templateValidationState) error {
	switch n := node.(type) {
	case nil:
		return nil
	case *parse.ListNode:
		return validateTemplateListNode(n, state)
	case *parse.ActionNode:
		return validateTemplateActionNode(n, state)
	case *parse.IfNode:
		return validateConditionalTemplateNode(n.Pipe, n.List, n.ElseList, state)
	case *parse.RangeNode:
		return validateRangeTemplateNode(n, state)
	case *parse.WithNode:
		return validateWithTemplateNode(n, state)
	case *parse.TemplateNode:
		return validateTemplateInvocationNode(n, state)
	case *parse.TextNode, *parse.CommentNode, *parse.BreakNode, *parse.ContinueNode:
		return nil
	}

	return nil
}

func validateTemplateListNode(node *parse.ListNode, state templateValidationState) error {
	if node == nil {
		return nil
	}
	for _, child := range node.Nodes {
		if err := validateTemplateNodeWithState(child, state); err != nil {
			return err
		}
	}
	return nil
}

func validateTemplateActionNode(node *parse.ActionNode, state templateValidationState) error {
	if node == nil {
		return nil
	}
	return validatePipeNodeWithState(node.Pipe, state)
}

func validateConditionalTemplateNode(
	pipe *parse.PipeNode,
	list *parse.ListNode,
	elseList *parse.ListNode,
	state templateValidationState,
) error {
	if err := validatePipeNodeWithState(pipe, state); err != nil {
		return err
	}
	if err := validateTemplateNodeWithState(list, state); err != nil {
		return err
	}
	return validateTemplateNodeWithState(elseList, state)
}

func validateRangeTemplateNode(node *parse.RangeNode, state templateValidationState) error {
	if node == nil {
		return nil
	}
	if err := validatePipeNodeWithState(node.Pipe, state); err != nil {
		return err
	}
	if err := validateTemplateNodeWithState(node.List, templateValidationState{}); err != nil {
		return err
	}
	return validateTemplateNodeWithState(node.ElseList, state)
}

func validateWithTemplateNode(node *parse.WithNode, state templateValidationState) error {
	if node == nil {
		return nil
	}
	if err := validatePipeNodeWithState(node.Pipe, state); err != nil {
		return err
	}
	if err := validateTemplateNodeWithState(node.List, withTemplateValidationState(node.Pipe, state)); err != nil {
		return err
	}
	return validateTemplateNodeWithState(node.ElseList, state)
}

func validateTemplateInvocationNode(node *parse.TemplateNode, state templateValidationState) error {
	if node == nil {
		return nil
	}
	if err := validatePipeNodeWithState(node.Pipe, state); err != nil {
		return err
	}
	path, ok := scopedTemplateFieldPath(node.Pipe, state)
	if !ok || len(path) != 0 {
		scope := "<none>"
		if node.Pipe != nil {
			scope = node.Pipe.String()
		}
		return fmt.Errorf(
			"unsupported template invocation %q scope %q; only root dot is supported",
			node.Name,
			scope,
		)
	}
	return nil
}

func validatePipeNodeWithState(pipe *parse.PipeNode, state templateValidationState) error {
	if pipe == nil {
		return nil
	}
	for _, cmd := range pipe.Cmds {
		if err := validateCommandNodeWithState(cmd, state); err != nil {
			return err
		}
	}
	return nil
}

func validateCommandNodeWithState(cmd *parse.CommandNode, state templateValidationState) error {
	if cmd == nil {
		return nil
	}
	if len(cmd.Args) == 0 {
		return nil
	}

	if ident, ok := cmd.Args[0].(*parse.IdentifierNode); ok && ident.Ident == "index" {
		if err := validateIndexArgs(cmd.Args[1:], state); err != nil {
			return err
		}
	}

	for _, arg := range cmd.Args {
		if err := validateTemplateArgWithState(arg, state); err != nil {
			return err
		}
	}

	return nil
}

func validateIndexArgs(args []parse.Node, state templateValidationState) error {
	if len(args) == 0 {
		return errors.New("index requires a target expression")
	}
	if expression, ok := variableRootExpression(args[0]); ok {
		return fmt.Errorf("unsupported index target %q; variable-rooted lookups are not supported", expression)
	}

	path, ok := scopedTemplateFieldPath(args[0], state)
	if !ok || len(path) == 0 {
		return fmt.Errorf("unsupported index target %q; only .Data is supported for dynamic lookups", args[0].String())
	}
	if path[0] != "Data" {
		return fmt.Errorf("unsupported index target %q; only .Data is supported for dynamic lookups", dottedPath(path))
	}
	return nil
}

func validateTemplateArgWithState(node parse.Node, state templateValidationState) error {
	switch n := node.(type) {
	case nil:
		return nil
	case *parse.FieldNode:
		path, ok := scopedTemplateFieldPath(n, state)
		if !ok {
			return fmt.Errorf("unsupported activation lookup %q; unresolved template scope", n.String())
		}
		return validateActivationFieldPath(path)
	case *parse.VariableNode:
		if len(n.Ident) > 1 {
			return fmt.Errorf("unsupported activation lookup %q; variable-rooted lookups are not supported", n.String())
		}
		return nil
	case *parse.ChainNode:
		if _, ok := variableRootExpression(n.Node); ok {
			return fmt.Errorf("unsupported activation lookup %q; variable-rooted lookups are not supported", n.String())
		}
		path, ok := scopedTemplateFieldPath(n, state)
		if !ok {
			return fmt.Errorf("unsupported activation lookup %q; unresolved template scope", n.String())
		}
		return validateActivationFieldPath(path)
	case *parse.PipeNode:
		return validatePipeNodeWithState(n, state)
	case *parse.CommandNode:
		return validateCommandNodeWithState(n, state)
	}

	return nil
}

func withTemplateValidationState(pipe *parse.PipeNode, state templateValidationState) templateValidationState {
	path, ok := scopedTemplateFieldPath(pipe, state)
	if !ok {
		return templateValidationState{}
	}
	return templateValidationState{
		dotPath:  append([]string(nil), path...),
		dotKnown: true,
	}
}

func scopedTemplateFieldPath(node parse.Node, state templateValidationState) ([]string, bool) {
	switch n := node.(type) {
	case *parse.FieldNode:
		if !state.dotKnown {
			return nil, false
		}
		return append(append([]string(nil), state.dotPath...), n.Ident...), true
	case *parse.ChainNode:
		base, ok := scopedTemplateFieldPath(n.Node, state)
		if !ok {
			return nil, false
		}
		return append(base, n.Field...), true
	case *parse.PipeNode:
		if n == nil || len(n.Cmds) != 1 {
			return nil, false
		}
		return scopedTemplateFieldPath(n.Cmds[0], state)
	case *parse.CommandNode:
		if n == nil || len(n.Args) != 1 {
			return nil, false
		}
		return scopedTemplateFieldPath(n.Args[0], state)
	case *parse.DotNode:
		if !state.dotKnown {
			return nil, false
		}
		return append([]string(nil), state.dotPath...), true
	default:
		return nil, false
	}
}

func variableRootExpression(node parse.Node) (string, bool) {
	switch n := node.(type) {
	case *parse.VariableNode:
		return n.String(), true
	case *parse.PipeNode:
		if n == nil || len(n.Cmds) != 1 {
			return "", false
		}
		return variableRootExpression(n.Cmds[0])
	case *parse.CommandNode:
		if n == nil || len(n.Args) != 1 {
			return "", false
		}
		return variableRootExpression(n.Args[0])
	case *parse.ChainNode:
		if expression, ok := variableRootExpression(n.Node); ok {
			return expression, true
		}
	}
	return "", false
}

func validateActivationFieldPath(path []string) error {
	if len(path) == 0 {
		return nil
	}

	switch path[0] {
	case "Kind", "Scope", "WorkspaceID", "Source":
		if len(path) > 1 {
			return fmt.Errorf("activation envelope field %q does not have child field %q", path[0], path[1])
		}
		return nil
	case "Data":
		return nil
	default:
		return fmt.Errorf("unknown activation envelope field %q", dottedPath(path))
	}
}

func dottedPath(path []string) string {
	if len(path) == 0 {
		return "."
	}
	return "." + strings.Join(path, ".")
}
