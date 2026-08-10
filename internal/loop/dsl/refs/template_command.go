package refs

import (
	"fmt"
	"strings"
	"text/template"
	"text/template/parse"
)

const shellQuoteTemplateFunction = "shellQuote"

// CompileCommandTemplate validates a shell command template before runtime materialization.
// Every action that emits runtime data must end in shellQuote so data cannot become shell syntax.
func CompileCommandTemplate(name string, raw string, namespace Namespace) (*Template, error) {
	tmpl, err := parseTemplate(name, raw)
	if err != nil {
		return nil, err
	}
	if err := validateShellCommandTemplate(tmpl); err != nil {
		return nil, err
	}
	return compileParsedTemplate(raw, tmpl, namespace)
}

// RenderCommandTemplateString materializes one validated shell command template.
func RenderCommandTemplateString(name string, raw string, data any) (string, error) {
	tmpl, err := parseTemplate(name, raw)
	if err != nil {
		return "", err
	}
	if err := validateShellCommandTemplate(tmpl); err != nil {
		return "", err
	}
	return executeTemplate(name, tmpl, data)
}

func validateShellCommandTemplate(tmpl *template.Template) error {
	for _, subtemplate := range tmpl.Templates() {
		if subtemplate.Tree == nil || subtemplate.Root == nil {
			continue
		}
		if err := validateShellCommandNode(subtemplate.Root); err != nil {
			return fmt.Errorf("validate command template %q: %w", subtemplate.Name(), err)
		}
	}
	return nil
}

func validateShellCommandNode(node parse.Node) error {
	switch typed := node.(type) {
	case nil:
		return nil
	case *parse.ListNode:
		if typed == nil {
			return nil
		}
		for _, child := range typed.Nodes {
			if err := validateShellCommandNode(child); err != nil {
				return err
			}
		}
		return nil
	case *parse.ActionNode:
		if typed.Pipe == nil || len(typed.Pipe.Decl) > 0 || commandPipeEndsWithShellQuote(typed.Pipe) {
			return nil
		}
		return &Error{
			Code: CodeUnsafeCommandInterpolation,
			Message: fmt.Sprintf(
				"rendered command values must end with | %s",
				shellQuoteTemplateFunction,
			),
		}
	case *parse.IfNode:
		return validateShellCommandBranches(typed.List, typed.ElseList)
	case *parse.RangeNode:
		return validateShellCommandBranches(typed.List, typed.ElseList)
	case *parse.WithNode:
		return validateShellCommandBranches(typed.List, typed.ElseList)
	default:
		return nil
	}
}

func validateShellCommandBranches(list *parse.ListNode, elseList *parse.ListNode) error {
	if err := validateShellCommandNode(list); err != nil {
		return err
	}
	return validateShellCommandNode(elseList)
}

func commandPipeEndsWithShellQuote(pipe *parse.PipeNode) bool {
	if pipe == nil || len(pipe.Cmds) == 0 {
		return false
	}
	command := pipe.Cmds[len(pipe.Cmds)-1]
	if command == nil || len(command.Args) == 0 {
		return false
	}
	identifier, ok := command.Args[0].(*parse.IdentifierNode)
	return ok && identifier.Ident == shellQuoteTemplateFunction
}

func templateShellQuote(value any) string {
	raw := fmt.Sprint(value)
	return "'" + strings.ReplaceAll(raw, "'", `'"'"'`) + "'"
}
