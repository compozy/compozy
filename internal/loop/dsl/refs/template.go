package refs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"text/template"
	"text/template/parse"
)

// Template is a parsed value interpolation template plus validated references.
type Template struct {
	Raw        string
	Parsed     *template.Template
	References []Reference
}

// RenderTemplateString parses and executes one runtime interpolation template with
// the same curated functions and missing-key behavior used by CompileTemplate.
func RenderTemplateString(name string, raw string, data any) (string, error) {
	tmpl, err := template.New(name).
		Option("missingkey=error").
		Funcs(funcMap()).
		Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse template %q: %w", name, err)
	}
	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data); err != nil {
		return "", fmt.Errorf("execute template %q: %w", name, err)
	}
	return rendered.String(), nil
}

// CompileTemplate parses and validates one Go text/template string.
func CompileTemplate(name string, raw string, namespace Namespace) (*Template, error) {
	tmpl, err := template.New(name).
		Option("missingkey=error").
		Funcs(funcMap()).
		Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse template %q: %w", name, err)
	}
	walker := templateWalker{namespace: namespace}
	for _, subtemplate := range tmpl.Templates() {
		if subtemplate.Tree == nil || subtemplate.Root == nil {
			continue
		}
		if err := walker.validate(subtemplate.Root, templateState{dotKnown: true}); err != nil {
			return nil, fmt.Errorf("validate template %q: %w", subtemplate.Name(), err)
		}
	}
	return &Template{
		Raw:        raw,
		Parsed:     tmpl,
		References: walker.references,
	}, nil
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"json":    templateJSON,
		"toJson":  templateJSON,
		"join":    templateJoin,
		"default": templateDefault,
	}
}

// allowedBuiltinFunctions lists text/template builtins the walker accepts.
// Builtins are always available at render time, so rejecting them here would
// make validation stricter than the runtime contract; only side-effect-free
// builtins loops rely on are allowed.
func allowedBuiltinFunctions() map[string]struct{} {
	return map[string]struct{}{
		"len": {},
	}
}

func templateJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal template value as json: %w", err)
	}
	return string(data), nil
}

func templateJoin(separator string, value any) (string, error) {
	switch typed := value.(type) {
	case []string:
		return strings.Join(typed, separator), nil
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, fmt.Sprint(item))
		}
		return strings.Join(parts, separator), nil
	case string:
		return typed, nil
	default:
		reflected := reflect.ValueOf(value)
		if reflected.IsValid() && reflected.Kind() == reflect.Slice {
			parts := make([]string, 0, reflected.Len())
			for i := 0; i < reflected.Len(); i++ {
				parts = append(parts, fmt.Sprint(reflected.Index(i).Interface()))
			}
			return strings.Join(parts, separator), nil
		}
		return "", fmt.Errorf("join expects a slice or string, got %T", value)
	}
}

func templateDefault(fallback any, value any) any {
	if isZeroTemplateValue(value) {
		return fallback
	}
	return value
}

func isZeroTemplateValue(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	return reflected.IsValid() && reflected.IsZero()
}

type templateWalker struct {
	namespace  Namespace
	references []Reference
}

type templateState struct {
	dotPath  []string
	dotKnown bool
}

func (w *templateWalker) validate(node parse.Node, state templateState) error {
	switch typed := node.(type) {
	case nil:
		return nil
	case *parse.ListNode:
		return w.validateList(typed, state)
	case *parse.ActionNode:
		return w.validatePipe(typed.Pipe, state)
	case *parse.IfNode:
		return w.validateConditional(typed.Pipe, typed.List, typed.ElseList, state)
	case *parse.RangeNode:
		return w.validateRange(typed, state)
	case *parse.WithNode:
		return w.validateWith(typed, state)
	case *parse.TemplateNode:
		return w.validateTemplateInvocation(typed, state)
	case *parse.TextNode, *parse.CommentNode, *parse.BreakNode, *parse.ContinueNode:
		return nil
	default:
		return nil
	}
}

func (w *templateWalker) validateList(node *parse.ListNode, state templateState) error {
	if node == nil {
		return nil
	}
	for _, child := range node.Nodes {
		if err := w.validate(child, state); err != nil {
			return err
		}
	}
	return nil
}

func (w *templateWalker) validateConditional(
	pipe *parse.PipeNode,
	list *parse.ListNode,
	elseList *parse.ListNode,
	state templateState,
) error {
	if err := w.validatePipe(pipe, state); err != nil {
		return err
	}
	if err := w.validate(list, state); err != nil {
		return err
	}
	return w.validate(elseList, state)
}

func (w *templateWalker) validateRange(node *parse.RangeNode, state templateState) error {
	if node == nil {
		return nil
	}
	if err := w.validatePipe(node.Pipe, state); err != nil {
		return err
	}
	rangeState := templateState{}
	if path, ok := scopedTemplatePath(node.Pipe, state); ok {
		rangeState = templateState{dotPath: path, dotKnown: true}
	}
	if err := w.validate(node.List, rangeState); err != nil {
		return err
	}
	return w.validate(node.ElseList, state)
}

func (w *templateWalker) validateWith(node *parse.WithNode, state templateState) error {
	if node == nil {
		return nil
	}
	if err := w.validatePipe(node.Pipe, state); err != nil {
		return err
	}
	withState := templateState{}
	if path, ok := scopedTemplatePath(node.Pipe, state); ok {
		withState = templateState{dotPath: path, dotKnown: true}
	}
	if err := w.validate(node.List, withState); err != nil {
		return err
	}
	return w.validate(node.ElseList, state)
}

func (w *templateWalker) validateTemplateInvocation(
	node *parse.TemplateNode,
	state templateState,
) error {
	if node == nil {
		return nil
	}
	if err := w.validatePipe(node.Pipe, state); err != nil {
		return err
	}
	path, ok := scopedTemplatePath(node.Pipe, state)
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

func (w *templateWalker) validatePipe(pipe *parse.PipeNode, state templateState) error {
	if pipe == nil {
		return nil
	}
	for _, cmd := range pipe.Cmds {
		if err := w.validateCommand(cmd, state); err != nil {
			return err
		}
	}
	return nil
}

func (w *templateWalker) validateCommand(cmd *parse.CommandNode, state templateState) error {
	if cmd == nil {
		return nil
	}
	if len(cmd.Args) == 0 {
		return nil
	}
	if ident, ok := cmd.Args[0].(*parse.IdentifierNode); ok {
		_, curated := funcMap()[ident.Ident]
		_, builtin := allowedBuiltinFunctions()[ident.Ident]
		if !curated && !builtin {
			return fmt.Errorf("unsupported template function %q", ident.Ident)
		}
	}
	for _, arg := range cmd.Args {
		if err := w.validateArg(arg, state); err != nil {
			return err
		}
	}
	return nil
}

func (w *templateWalker) validateArg(node parse.Node, state templateState) error {
	switch typed := node.(type) {
	case nil:
		return nil
	case *parse.FieldNode:
		return w.validatePathNode(typed, state)
	case *parse.VariableNode:
		if len(typed.Ident) > 1 {
			return fmt.Errorf("unsupported template variable lookup %q", typed.String())
		}
		return nil
	case *parse.ChainNode:
		return w.validatePathNode(typed, state)
	case *parse.PipeNode:
		return w.validatePipe(typed, state)
	case *parse.CommandNode:
		return w.validateCommand(typed, state)
	default:
		return nil
	}
}

func (w *templateWalker) validatePathNode(node parse.Node, state templateState) error {
	path, ok := scopedTemplatePath(node, state)
	if !ok {
		return fmt.Errorf("unsupported template lookup %q; unresolved scope", node.String())
	}
	if err := w.namespace.ValidatePath(path); err != nil {
		return err
	}
	w.references = append(w.references, Reference{
		Raw:  node.String(),
		Path: append([]string(nil), path...),
	})
	return nil
}

func scopedTemplatePath(node parse.Node, state templateState) ([]string, bool) {
	switch typed := node.(type) {
	case *parse.FieldNode:
		if !state.dotKnown {
			return nil, false
		}
		return append(append([]string(nil), state.dotPath...), typed.Ident...), true
	case *parse.ChainNode:
		base, ok := scopedTemplatePath(typed.Node, state)
		if !ok {
			return nil, false
		}
		return append(base, typed.Field...), true
	case *parse.PipeNode:
		if typed == nil || len(typed.Cmds) != 1 {
			return nil, false
		}
		return scopedTemplatePath(typed.Cmds[0], state)
	case *parse.CommandNode:
		if typed == nil || len(typed.Args) != 1 {
			return nil, false
		}
		return scopedTemplatePath(typed.Args[0], state)
	case *parse.DotNode:
		if !state.dotKnown {
			return nil, false
		}
		return append([]string(nil), state.dotPath...), true
	default:
		return nil, false
	}
}
