package refs

import (
	"fmt"
	"strings"
	"text/template"
	"text/template/parse"
)

const shellQuoteTemplateFunction = "shellQuote"

type shellCommandQuote uint8

const (
	shellCommandUnquoted shellCommandQuote = iota
	shellCommandSingleQuoted
	shellCommandDoubleQuoted
)

type shellCommandContext struct {
	quote     shellCommandQuote
	escaped   bool
	comment   bool
	wordStart bool
	lastLess  bool
}

type shellCommandTemplateValidator struct {
	trees         map[string]*parse.Tree
	active        map[string]struct{}
	emitsDynamic  bool
	hasDoubleLess bool
}

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
	trees := make(map[string]*parse.Tree, len(tmpl.Templates()))
	for _, subtemplate := range tmpl.Templates() {
		if subtemplate.Tree != nil {
			trees[subtemplate.Name()] = subtemplate.Tree
		}
	}
	validator := shellCommandTemplateValidator{
		trees:  trees,
		active: make(map[string]struct{}),
	}
	state, err := validator.validateTemplate(tmpl.Name(), shellCommandContext{wordStart: true})
	if err != nil {
		return fmt.Errorf("validate command template %q: %w", tmpl.Name(), err)
	}
	if state.quote != shellCommandUnquoted || state.escaped {
		return unsafeCommandTemplateError("authored command must end outside shell quotes and escapes")
	}
	// Reject << anywhere when runtime data is present. This intentionally conservative rule keeps
	// heredoc and here-string parsing away from dynamic templates, even across unrelated branches.
	if validator.emitsDynamic && validator.hasDoubleLess {
		return unsafeCommandTemplateError(
			"dynamic command templates cannot use << shell constructs",
		)
	}
	return nil
}

func (v *shellCommandTemplateValidator) validateTemplate(
	name string,
	state shellCommandContext,
) (shellCommandContext, error) {
	tree, ok := v.trees[name]
	if !ok || tree == nil || tree.Root == nil {
		return state, unsafeCommandTemplateError(fmt.Sprintf("command subtemplate %q is unavailable", name))
	}
	if _, recursive := v.active[name]; recursive {
		return state, unsafeCommandTemplateError(fmt.Sprintf("recursive command subtemplate %q is not supported", name))
	}
	v.active[name] = struct{}{}
	defer delete(v.active, name)
	return v.validateShellCommandNode(tree.Root, state)
}

func (v *shellCommandTemplateValidator) validateShellCommandNode(
	node parse.Node,
	state shellCommandContext,
) (shellCommandContext, error) {
	switch typed := node.(type) {
	case nil:
		return state, nil
	case *parse.ListNode:
		return v.validateShellCommandList(typed, state)
	case *parse.TextNode:
		return v.scanShellCommandText(typed.Text, state), nil
	case *parse.ActionNode:
		return v.validateShellCommandAction(typed, state)
	case *parse.IfNode:
		return v.validateShellCommandBranches(state, typed.List, typed.ElseList)
	case *parse.RangeNode:
		return v.validateShellCommandRange(typed, state)
	case *parse.WithNode:
		return v.validateShellCommandBranches(state, typed.List, typed.ElseList)
	case *parse.TemplateNode:
		return v.validateTemplate(typed.Name, state)
	case *parse.CommentNode, *parse.BreakNode, *parse.ContinueNode:
		return state, nil
	default:
		return state, unsafeCommandTemplateError(fmt.Sprintf(
			"unsupported command template node %T", node,
		))
	}
}

func (v *shellCommandTemplateValidator) validateShellCommandList(
	node *parse.ListNode,
	state shellCommandContext,
) (shellCommandContext, error) {
	if node == nil {
		return state, nil
	}
	for _, child := range node.Nodes {
		var err error
		state, err = v.validateShellCommandNode(child, state)
		if err != nil {
			return state, err
		}
	}
	return state, nil
}

func (v *shellCommandTemplateValidator) validateShellCommandAction(
	node *parse.ActionNode,
	state shellCommandContext,
) (shellCommandContext, error) {
	if node.Pipe == nil || len(node.Pipe.Decl) > 0 {
		return state, nil
	}
	if !commandPipeEndsWithShellQuote(node.Pipe) {
		return state, unsafeCommandTemplateError(fmt.Sprintf(
			"rendered command values must end with | %s", shellQuoteTemplateFunction,
		))
	}
	if state.quote != shellCommandUnquoted || state.escaped || state.comment {
		return state, unsafeCommandTemplateError(
			"shellQuote interpolation must appear outside authored shell quotes, escapes, and comments",
		)
	}
	v.emitsDynamic = true
	state.wordStart = false
	state.lastLess = false
	return state, nil
}

func (v *shellCommandTemplateValidator) validateShellCommandRange(
	node *parse.RangeNode,
	state shellCommandContext,
) (shellCommandContext, error) {
	bodyState, err := v.validateShellCommandNode(node.List, state)
	if err != nil {
		return state, err
	}
	elseState, err := v.validateShellCommandNode(node.ElseList, state)
	if err != nil {
		return state, err
	}
	if bodyState != state || elseState != state {
		return state, unsafeCommandTemplateError(
			"command range branches must preserve their shell quote context",
		)
	}
	return state, nil
}

func (v *shellCommandTemplateValidator) validateShellCommandBranches(
	state shellCommandContext,
	list *parse.ListNode,
	elseList *parse.ListNode,
) (shellCommandContext, error) {
	left, err := v.validateShellCommandNode(list, state)
	if err != nil {
		return state, err
	}
	right, err := v.validateShellCommandNode(elseList, state)
	if err != nil {
		return state, err
	}
	if left != state || right != state {
		return state, unsafeCommandTemplateError(
			"command template branches must preserve their entry shell quote context",
		)
	}
	return state, nil
}

func (v *shellCommandTemplateValidator) scanShellCommandText(
	text []byte,
	state shellCommandContext,
) shellCommandContext {
	for _, character := range text {
		if state.comment {
			if state.escaped {
				state.escaped = false
				continue
			}
			if character == '\\' {
				state.escaped = true
				continue
			}
			if character == '\n' {
				state.comment = false
				state.wordStart = true
				state.lastLess = false
			}
			continue
		}
		if state.escaped {
			state.escaped = false
			state.lastLess = false
			if character != '\n' {
				state.wordStart = false
			}
			continue
		}
		switch state.quote {
		case shellCommandSingleQuoted:
			if character == '\'' {
				state.quote = shellCommandUnquoted
			}
			state.wordStart = false
			state.lastLess = false
		case shellCommandDoubleQuoted:
			switch character {
			case '\\':
				state.escaped = true
			case '"':
				state.quote = shellCommandUnquoted
			}
			state.wordStart = false
			state.lastLess = false
		default:
			state = v.scanUnquotedShellCharacter(character, state)
		}
	}
	return state
}

func (v *shellCommandTemplateValidator) scanUnquotedShellCharacter(
	character byte,
	state shellCommandContext,
) shellCommandContext {
	switch character {
	case '\\':
		state.escaped = true
		state.lastLess = false
	case '\'':
		state.quote = shellCommandSingleQuoted
		state.wordStart = false
		state.lastLess = false
	case '"':
		state.quote = shellCommandDoubleQuoted
		state.wordStart = false
		state.lastLess = false
	case '#':
		if state.wordStart {
			state.comment = true
		} else {
			state.wordStart = false
		}
		state.lastLess = false
	case '<':
		if state.lastLess {
			v.hasDoubleLess = true
		}
		state.wordStart = true
		state.lastLess = true
	default:
		state.lastLess = false
		state.wordStart = isShellWordBoundary(character)
	}
	return state
}

func isShellWordBoundary(character byte) bool {
	switch character {
	case ' ', '\t', '\r', '\n', ';', '|', '&', '(', ')', '>':
		return true
	default:
		return false
	}
}

func unsafeCommandTemplateError(message string) *Error {
	return &Error{Code: CodeUnsafeCommandInterpolation, Message: message}
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
