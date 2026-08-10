package refs_test

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/loop/dsl/refs"
)

func TestTemplateShouldValidateReferencesAgainstNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		template  string
		namespace refs.Namespace
		wantCode  string
	}{
		{
			name:      "Should accept declared input and node output references",
			template:  `{{ default "fallback" .inputs.slug }} {{ json .nodes.load.output.tasks }}`,
			namespace: namespace(false),
		},
		{
			name:      "Should accept toJson as a curated JSON alias",
			template:  `{{ toJson .nodes.load.output.tasks }}`,
			namespace: namespace(false),
		},
		{
			name:      "Should reject unknown input references",
			template:  `{{ .inputs.missing }}`,
			namespace: namespace(false),
			wantCode:  refs.CodeUnknownReference,
		},
		{
			name:      "Should reject unresolvable output paths",
			template:  `{{ .nodes.load.output.missing }}`,
			namespace: namespace(false),
			wantCode:  refs.CodeUnresolvablePath,
		},
		{
			name:      "Should reject item outside fanout",
			template:  `{{ .item.title }}`,
			namespace: namespace(false),
			wantCode:  refs.CodeItemOutsideFanout,
		},
		{
			name:      "Should accept item inside fanout",
			template:  `{{ .item.title }}`,
			namespace: namespace(true),
		},
		{
			name:      "Should accept the len builtin inside fanout",
			template:  `Batch of {{ len .item }} issues: {{ .item.title }}`,
			namespace: namespace(true),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			compiled, err := refs.CompileTemplate("test", tt.template, tt.namespace)
			if tt.wantCode != "" {
				requireRefCode(t, err, tt.wantCode)
				return
			}
			if err != nil {
				t.Fatalf("CompileTemplate() error = %v", err)
			}
			if compiled.Parsed == nil {
				t.Fatal("CompileTemplate().Parsed is nil")
			}
			if len(compiled.References) == 0 {
				t.Fatal("CompileTemplate().References is empty")
			}
		})
	}

	t.Run("Should report the deepest valid schema segment and available fields", func(t *testing.T) {
		t.Parallel()

		path := []string{"nodes", "load", "output", "tasks", "summary"}
		err := namespace(false).ValidatePath(path)
		if err == nil {
			t.Fatal("ValidatePath() error = nil, want missing deep field")
		}
		if err.Code != refs.CodeUnresolvablePath ||
			!reflect.DeepEqual(err.DeepestValidPath, []string{"nodes", "load", "output", "tasks"}) ||
			!reflect.DeepEqual(err.AvailableFields, []string{"title"}) {
			t.Fatalf("ValidatePath() error = %#v, want deepest tasks and available title", err)
		}
		for _, fragment := range []string{"nodes.load.output.tasks.summary", "nodes.load.output.tasks", "title"} {
			if !strings.Contains(err.Error(), fragment) {
				t.Fatalf("ValidatePath() message = %q, want %q", err.Error(), fragment)
			}
		}
	})

	t.Run("Should list existing nodes for a stale node reference", func(t *testing.T) {
		t.Parallel()

		err := namespace(false).ValidatePath([]string{"nodes", "lod", "output", "tasks"})
		if err == nil {
			t.Fatal("ValidatePath() error = nil, want stale node failure")
		}
		wantNodes := []string{"gate", "load", "review"}
		if err.Code != refs.CodeUnknownReference || !reflect.DeepEqual(err.AvailableNodes, wantNodes) {
			t.Fatalf("ValidatePath() error = %#v, want nodes %#v", err, wantNodes)
		}
		for _, nodeID := range wantNodes {
			if !strings.Contains(err.Error(), nodeID) {
				t.Fatalf("ValidatePath() message = %q, want node %q", err.Error(), nodeID)
			}
		}
	})

	t.Run("Should preserve the same diagnostic shape across template and condition compilation", func(t *testing.T) {
		t.Parallel()

		compiler, err := refs.NewConditionCompiler(namespace(false))
		if err != nil {
			t.Fatalf("NewConditionCompiler() error = %v", err)
		}
		_, templateErr := refs.CompileTemplate(
			"test",
			"{{ .nodes.load.output.tasks.summary }}",
			namespace(false),
		)
		_, conditionErr := compiler.Compile("nodes.load.output.tasks.summary == 'missing'")
		for label, compileErr := range map[string]error{"template": templateErr, "condition": conditionErr} {
			var refErr *refs.Error
			if !errors.As(compileErr, &refErr) {
				t.Fatalf("%s compile error = %v, want refs.Error", label, compileErr)
			}
			if refErr.Code != refs.CodeUnresolvablePath ||
				!reflect.DeepEqual(refErr.AvailableFields, []string{"title"}) {
				t.Fatalf("%s diagnostic = %#v, want unresolvable path with title", label, refErr)
			}
		}
	})
}

func TestCommandTemplateShouldQuoteRuntimeValuesAsShellData(t *testing.T) {
	t.Parallel()

	t.Run("Should reject an unquoted runtime value", func(t *testing.T) {
		t.Parallel()

		_, err := refs.CompileCommandTemplate(
			"command",
			`test -f .compozy/tasks/{{ .inputs.slug }}/task.md`,
			namespace(false),
		)
		if err == nil || !strings.Contains(err.Error(), "must end with | shellQuote") {
			t.Fatalf("CompileCommandTemplate() error = %v, want shellQuote requirement", err)
		}
	})

	t.Run("Should reject shellQuote inside authored quotes or escapes", func(t *testing.T) {
		t.Parallel()

		for _, test := range []struct {
			name string
			raw  string
		}{
			{name: "double quote", raw: `echo "{{ .inputs.slug | shellQuote }}"`},
			{name: "single quote", raw: `echo '{{ .inputs.slug | shellQuote }}'`},
			{name: "escape", raw: `echo \{{ .inputs.slug | shellQuote }}`},
			{name: "comment", raw: `echo ready # {{ .inputs.slug | shellQuote }}`},
			{
				name: "continued comment",
				raw:  "echo ready # continued \\\n{{ .inputs.slug | shellQuote }}",
			},
			{
				name: "heredoc body",
				raw:  "cat <<EOF\n{{ .inputs.slug | shellQuote }}\nEOF",
			},
			{name: "dynamic heredoc delimiter", raw: `cat <<{{ .inputs.slug | shellQuote }}`},
			{
				name: "conditional branch",
				raw:  `{{ if .inputs.done }}echo "{{ .inputs.slug | shellQuote }}"{{ end }}`,
			},
			{
				name: "subtemplate",
				raw:  `{{ define "value" }}{{ .inputs.slug | shellQuote }}{{ end }}echo "{{ template "value" . }}"`,
			},
		} {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				_, err := refs.CompileCommandTemplate("command", test.raw, namespace(false))
				var refErr *refs.Error
				if !errors.As(err, &refErr) || refErr.Code != refs.CodeUnsafeCommandInterpolation {
					t.Fatalf("CompileCommandTemplate() error = %v, want unsafe command interpolation", err)
				}
			})
		}
	})

	t.Run("Should preserve authored shell operators around plain quoted values", func(t *testing.T) {
		t.Parallel()

		raw := "# static comment with \\\"quotes\\\"\n" +
			`test -f {{ .inputs.slug | shellQuote }} | tee {{ .inputs.slug | shellQuote }} > output.log`
		if _, err := refs.CompileCommandTemplate("command", raw, namespace(false)); err != nil {
			t.Fatalf("CompileCommandTemplate() error = %v", err)
		}
	})

	t.Run("Should reject conditional shell quote context changes", func(t *testing.T) {
		t.Parallel()

		raw := `{{ if .inputs.done }}'{{ else }}'{{ end }}value{{ if .inputs.done }}'{{ else }}'{{ end }}`
		_, err := refs.CompileCommandTemplate("command", raw, namespace(false))
		var refErr *refs.Error
		if !errors.As(err, &refErr) || refErr.Code != refs.CodeUnsafeCommandInterpolation {
			t.Fatalf("CompileCommandTemplate() error = %v, want conditional shell quote rejection", err)
		}
	})

	t.Run("Should render shell metacharacters inside one quoted value", func(t *testing.T) {
		t.Parallel()

		raw := `test -f .compozy/tasks/{{ .inputs.slug | shellQuote }}/task.md`
		if _, err := refs.CompileCommandTemplate("command", raw, namespace(false)); err != nil {
			t.Fatalf("CompileCommandTemplate() error = %v", err)
		}
		rendered, err := refs.RenderCommandTemplateString("command", raw, map[string]any{
			"inputs": map[string]any{"slug": `weather'; touch /tmp/injected; #'`},
		})
		if err != nil {
			t.Fatalf("RenderCommandTemplateString() error = %v", err)
		}
		want := `test -f .compozy/tasks/'weather'"'"'; touch /tmp/injected; #'"'"''/task.md`
		if rendered != want {
			t.Fatalf("RenderCommandTemplateString() = %q, want %q", rendered, want)
		}
	})
}

func TestTemplateShouldRejectNonAllowlistedFunctions(t *testing.T) {
	t.Parallel()

	t.Run("Should reject builtins outside the allowlist", func(t *testing.T) {
		t.Parallel()

		_, err := refs.CompileTemplate("test", `{{ printf "%s" .item.title }}`, namespace(true))
		if err == nil || !strings.Contains(err.Error(), `unsupported template function "printf"`) {
			t.Fatalf("CompileTemplate() error = %v, want unsupported template function", err)
		}
	})
}

func TestConditionShouldCompileCacheAndValidateBoolContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		expr      string
		namespace refs.Namespace
		wantCode  string
	}{
		{
			name:      "Should compile bool CEL over namespace",
			expr:      `inputs.done == true && nodes.load.output.tasks != null`,
			namespace: namespace(false),
		},
		{
			name:      "Should reject non bool CEL",
			expr:      `inputs.slug`,
			namespace: namespace(false),
			wantCode:  refs.CodeConditionNotBool,
		},
		{
			name:      "Should reject item outside fanout in CEL",
			expr:      `item.title == "x"`,
			namespace: namespace(false),
			wantCode:  refs.CodeItemOutsideFanout,
		},
		{
			name:      "Should accept item inside fanout in CEL",
			expr:      `item.title == "x" || index >= 0`,
			namespace: namespace(true),
		},
		{
			name:      "Should accept CEL macros with local variables",
			expr:      `nodes.load.output.tasks.exists(task, task.title == "Task A")`,
			namespace: namespace(false),
		},
		{
			name: "Should accept CEL member functions without treating methods as paths",
			expr: `nodes.review.output.verdict.startsWith("approve") &&
				nodes.review.output.verdict.contains("proof") &&
				nodes.review.output.verdict.matches("approve.*")`,
			namespace: namespace(false),
		},
		{
			name:      "Should accept CEL indexed output field paths",
			expr:      `nodes.gate.output.blocking_issues[0].id == "issue-1"`,
			namespace: namespace(false),
		},
		{
			name:      "Should accept event paths when event is allowed",
			expr:      `event.task_id == inputs.slug && event.payload.to_status == "blocked"`,
			namespace: namespaceWithEvent(false),
		},
		{
			name:      "Should reject event paths outside watch-events filters",
			expr:      `event.task_id == "task-1"`,
			namespace: namespace(false),
			wantCode:  refs.CodeUnknownReference,
		},
		{
			name:      "Should reject CEL indexed output fields that do not resolve",
			expr:      `nodes.gate.output.blocking_issues[0].missing == "issue-1"`,
			namespace: namespace(false),
			wantCode:  refs.CodeUnresolvablePath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			compiler, err := refs.NewConditionCompiler(tt.namespace)
			if err != nil {
				t.Fatalf("NewConditionCompiler() error = %v", err)
			}
			compiled, err := compiler.Compile(tt.expr)
			if tt.wantCode != "" {
				requireRefCode(t, err, tt.wantCode)
				return
			}
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			again, err := compiler.Compile(tt.expr)
			if err != nil {
				t.Fatalf("Compile() second error = %v", err)
			}
			if compiled != again {
				t.Fatal("Compile() did not return cached condition")
			}
			if compiled.Program == nil {
				t.Fatal("Compile().Program is nil")
			}
		})
	}
}

func TestReferencesShouldUseSameNamespaceForTemplateAndCEL(t *testing.T) {
	t.Parallel()

	t.Run("Should use same namespace for template and CEL", func(t *testing.T) {
		t.Parallel()

		ns := namespace(false)
		tmpl, err := refs.CompileTemplate("template", `{{ .nodes.load.output.tasks }}`, ns)
		if err != nil {
			t.Fatalf("CompileTemplate() error = %v", err)
		}
		compiler, err := refs.NewConditionCompiler(ns)
		if err != nil {
			t.Fatalf("NewConditionCompiler() error = %v", err)
		}
		condition, err := compiler.Compile(`nodes.load.output.tasks != null`)
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		if got, want := pathString(tmpl.References[0].Path), pathString(condition.References[0].Path); got != want {
			t.Fatalf("template path = %q, CEL path = %q", got, want)
		}
	})
}

func TestHistoryReferencesShouldUseIdenticalGrammarForPathsTemplatesAndCEL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     []string
		wantCode string
	}{
		{name: "Should accept the previous generation", path: []string{"previous", "generation"}},
		{
			name: "Should accept previous node output",
			path: []string{"previous", "nodes", "load", "output", "tasks", "title"},
		},
		{
			name: "Should accept keyed previous verdict diagnostics",
			path: []string{"previous", "verdicts", "quality", "blocking_issues"},
		},
		{name: "Should accept the best score", path: []string{"best", "score"}},
		{name: "Should accept best node output", path: []string{"best", "nodes", "load", "output", "tasks", "title"}},
		{
			name:     "Should reject a singular previous verdict",
			path:     []string{"previous", "verdict"},
			wantCode: refs.CodeUnknownReference,
		},
		{
			name:     "Should reject the best verdict",
			path:     []string{"best", "verdict"},
			wantCode: refs.CodeUnknownReference,
		},
		{
			name:     "Should reject status below the best projection",
			path:     []string{"best", "nodes", "load", "status"},
			wantCode: refs.CodeUnknownReference,
		},
		{
			name:     "Should reject a child below previous node status",
			path:     []string{"previous", "nodes", "load", "status", "code"},
			wantCode: refs.CodeUnresolvablePath,
		},
		{
			name:     "Should reject a child below previous verdict outcome",
			path:     []string{"previous", "verdicts", "quality", "outcome", "code"},
			wantCode: refs.CodeUnresolvablePath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			namespace := namespace(false)
			assertHistoryReferenceCode(t, namespace.ValidatePath(tt.path), tt.wantCode)
			template, err := refs.CompileTemplate("history", "{{ ."+pathString(tt.path)+" }}", namespace)
			if tt.wantCode == "" {
				if err != nil {
					t.Fatalf("CompileTemplate() error = %v", err)
				}
				if len(template.References) != 1 {
					t.Fatalf("CompileTemplate() references = %#v, want one path", template.References)
				}
			} else {
				requireRefCode(t, err, tt.wantCode)
			}
			compiler, err := refs.NewConditionCompiler(namespace)
			if err != nil {
				t.Fatalf("NewConditionCompiler() error = %v", err)
			}
			_, err = compiler.Compile(pathString(tt.path) + " != null")
			if tt.wantCode == "" {
				if err != nil {
					t.Fatalf("Compile() error = %v", err)
				}
			} else {
				requireRefCode(t, err, tt.wantCode)
			}
		})
	}
}

func TestTemplateShouldExecuteCuratedFunctionsAndStructuredActions(t *testing.T) {
	t.Parallel()

	t.Run("Should execute curated functions and structured actions", func(t *testing.T) {
		t.Parallel()

		compiled, err := refs.CompileTemplate(
			"exec",
			`{{ if .inputs.slug }}{{ default "fallback" .inputs.empty }}{{ end }}|{{ join "," .inputs.tags }}|{{ range .nodes.load.output.tasks }}{{ .title }}{{ end }}|{{ with .nodes.load.output.tasks }}{{ json . }}|{{ toJson . }}{{ end }}`,
			namespace(false),
		)
		if err != nil {
			t.Fatalf("CompileTemplate() error = %v", err)
		}

		var out bytes.Buffer
		err = compiled.Parsed.Execute(&out, map[string]any{
			"inputs": map[string]any{
				"slug":  "present",
				"empty": "",
				"tags":  []string{"one", "two"},
			},
			"nodes": map[string]any{
				"load": map[string]any{
					"output": map[string]any{
						"tasks": []map[string]any{{"title": "Task A"}},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		want := `fallback|one,two|Task A|[{"title":"Task A"}]|[{"title":"Task A"}]`
		if out.String() != want {
			t.Fatalf("Execute() = %q, want %q", out.String(), want)
		}
		if len(compiled.References) < 4 {
			t.Fatalf("References len = %d, want at least 4", len(compiled.References))
		}
	})
}

func TestTemplateShouldRejectUnsupportedTemplateInvocationScopes(t *testing.T) {
	t.Parallel()

	t.Run("Should reject unsupported template invocation scopes", func(t *testing.T) {
		t.Parallel()

		_, err := refs.CompileTemplate(
			"template-invocation",
			`{{ define "partial" }}x{{ end }}{{ template "partial" .inputs.slug }}`,
			namespace(false),
		)
		if err == nil {
			t.Fatal("CompileTemplate() error = nil, want unsupported template invocation scope")
		}
		if !strings.Contains(err.Error(), "unsupported template invocation") {
			t.Fatalf("CompileTemplate() error = %v, want unsupported template invocation scope", err)
		}
	})
}

func TestNamespaceShouldValidateRootsAndSchemaShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		path      []string
		namespace refs.Namespace
		wantCode  string
	}{
		{
			name:      "Should accept empty paths",
			namespace: namespace(false),
		},
		{
			name:      "Should accept node status scalar",
			path:      []string{"nodes", "load", "status"},
			namespace: namespace(false),
		},
		{
			name:      "Should reject child path below node status",
			path:      []string{"nodes", "load", "status", "code"},
			namespace: namespace(false),
			wantCode:  refs.CodeUnresolvablePath,
		},
		{
			name:      "Should reject unknown node field",
			path:      []string{"nodes", "load", "state"},
			namespace: namespace(false),
			wantCode:  refs.CodeUnknownReference,
		},
		{
			name:      "Should resolve array item properties",
			path:      []string{"nodes", "load", "output", "tasks", "title"},
			namespace: namespace(false),
		},
		{
			name:      "Should reject trigger outside trigger starts",
			path:      []string{"trigger", "event"},
			namespace: namespace(false),
			wantCode:  refs.CodeUnknownReference,
		},
		{
			name: "Should accept trigger when allowed",
			path: []string{"trigger", "event"},
			namespace: refs.Namespace{
				Inputs:       namespace(false).Inputs,
				Nodes:        namespace(false).Nodes,
				AllowTrigger: true,
			},
		},
		{
			name:      "Should reject event outside watch-events filters",
			path:      []string{"event", "payload", "to_status"},
			namespace: namespace(false),
			wantCode:  refs.CodeUnknownReference,
		},
		{
			name:      "Should accept event when allowed",
			path:      []string{"event", "payload", "to_status"},
			namespace: namespaceWithEvent(false),
		},
		{
			name:      "Should reject generation children",
			path:      []string{"generation", "value"},
			namespace: namespace(false),
			wantCode:  refs.CodeUnresolvablePath,
		},
		{
			name:      "Should reject unknown roots",
			path:      []string{"workspace", "id"},
			namespace: namespace(false),
			wantCode:  refs.CodeUnknownReference,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.namespace.ValidatePath(tt.path)
			if tt.wantCode != "" {
				if err == nil {
					t.Fatalf("ValidatePath(%v) error = nil, want code %s", tt.path, tt.wantCode)
					return
				}
				if err.Code != tt.wantCode {
					t.Fatalf("ValidatePath(%v) code = %s, want %s", tt.path, err.Code, tt.wantCode)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidatePath(%v) error = %v", tt.path, err)
			}
		})
	}
}

func TestConditionShouldAcceptCustomCostLimit(t *testing.T) {
	t.Parallel()

	t.Run("Should accept custom cost limit", func(t *testing.T) {
		t.Parallel()

		compiler, err := refs.NewConditionCompiler(namespace(false), refs.WithCostLimit(1))
		if err != nil {
			t.Fatalf("NewConditionCompiler() error = %v", err)
		}
		condition, err := compiler.Compile("true")
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		if condition.Program == nil {
			t.Fatal("Compile().Program is nil")
		}
	})

	t.Run("Should track evaluation cost and warn at the inclusive threshold", func(t *testing.T) {
		t.Parallel()

		if !refs.CostWarningThresholdReached(8, 10) {
			t.Fatal("CostWarningThresholdReached(8, 10) = false, want true")
		}
		if refs.CostWarningThresholdReached(7, 10) {
			t.Fatal("CostWarningThresholdReached(7, 10) = true, want false")
		}
		maximum := ^uint64(0)
		if !refs.CostWarningThresholdReached(maximum, maximum) {
			t.Fatal("CostWarningThresholdReached(max, max) = false, want true without overflow")
		}
		compiler, err := refs.NewConditionCompiler(namespace(false), refs.WithCostLimit(100))
		if err != nil {
			t.Fatalf("NewConditionCompiler() error = %v", err)
		}
		condition, err := compiler.Compile("inputs.slug == 'ready'")
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		evaluation, err := condition.Evaluate(map[string]any{
			"inputs": map[string]any{"slug": "ready"},
		})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if !evaluation.Value || evaluation.Cost == 0 {
			t.Fatalf("Evaluate() = %#v, want true with tracked cost", evaluation)
		}
	})

	t.Run("Should treat absent first-generation history as data", func(t *testing.T) {
		t.Parallel()

		compiler, err := refs.NewConditionCompiler(namespace(false))
		if err != nil {
			t.Fatalf("NewConditionCompiler() error = %v", err)
		}
		condition, err := compiler.Compile("!has(previous.nodes.load.status)")
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		evaluation, err := condition.Evaluate(map[string]any{
			"previous": map[string]any{"nodes": map[string]any{"load": map[string]any{}}},
		})
		if err != nil {
			t.Fatalf("Evaluate() error = %v, want absent history value", err)
		}
		if !evaluation.Value {
			t.Fatalf("Evaluate().Value = false, want absent previous.nodes.load.status")
		}
	})
}

func namespace(allowFanout bool) refs.Namespace {
	return refs.Namespace{
		Inputs: map[string]refs.Schema{
			"slug":  {"type": "string"},
			"empty": {"type": "string"},
			"tags":  {"type": "array", "items": map[string]any{"type": "string"}},
			"done":  {"type": "boolean"},
		},
		Nodes: map[string]refs.NodeSchema{
			"load": {
				HasOutput: true,
				Output: refs.Schema{
					"tasks": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"title": map[string]any{"type": "string"},
							},
						},
					},
				},
			},
			"review": {
				HasOutput: true,
				Output: refs.Schema{
					"verdict": "string",
				},
			},
			"gate": {
				HasOutput: true,
				Output: refs.Schema{
					"blocking_issues": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"id": map[string]any{"type": "string"},
							},
						},
					},
				},
			},
		},
		AllowFanout: allowFanout,
	}
}

func namespaceWithEvent(allowFanout bool) refs.Namespace {
	ns := namespace(allowFanout)
	ns.AllowEvent = true
	return ns
}

func requireRefCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want code %s", code)
	}

	refErr, refErrMatched := errors.AsType[*refs.Error](err)
	if !refErrMatched {
		t.Fatalf("error = %T %v, want *refs.Error", err, err)
	}
	if refErr.Code != code {
		t.Fatalf("error code = %s, want %s", refErr.Code, code)
	}
}

func assertHistoryReferenceCode(t *testing.T, err *refs.Error, wantCode string) {
	t.Helper()
	if wantCode == "" {
		if err != nil {
			t.Fatalf("ValidatePath() error = %v", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("ValidatePath() error = nil, want code %s", wantCode)
	}
	if err.Code != wantCode {
		t.Fatalf("ValidatePath() code = %s, want %s", err.Code, wantCode)
	}
}

func pathString(path []string) string {
	return strings.Join(path, ".")
}
