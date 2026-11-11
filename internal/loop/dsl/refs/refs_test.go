package refs_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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
			refErr, ok := errors.AsType[*refs.Error](compileErr)
			if !ok {
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
				refErr, ok := errors.AsType[*refs.Error](err)
				if !ok || refErr.Code != refs.CodeUnsafeCommandInterpolation {
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
		refErr, ok := errors.AsType[*refs.Error](err)
		if !ok || refErr.Code != refs.CodeUnsafeCommandInterpolation {
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

	tests := []struct {
		name     string
		template string
		function string
	}{
		{name: "Should reject printf", template: `{{ printf "%s" .item.title }}`, function: "printf"},
		{name: "Should reject print", template: `{{ print .item.title }}`, function: "print"},
		{name: "Should reject println", template: `{{ println .item.title }}`, function: "println"},
		{name: "Should reject call", template: `{{ call .item.title }}`, function: "call"},
		{name: "Should reject html", template: `{{ html .item.title }}`, function: "html"},
		{name: "Should reject js", template: `{{ js .item.title }}`, function: "js"},
		{name: "Should reject urlquery", template: `{{ urlquery .item.title }}`, function: "urlquery"},
		{name: "Should reject index", template: `{{ index .item "title" }}`, function: "index"},
		{name: "Should reject slice", template: `{{ slice .item.title 0 1 }}`, function: "slice"},
		{
			name:     "Should reject a disallowed builtin nested in a comparison",
			template: `{{ if eq (printf "%s" .item.title) "x" }}y{{ end }}`,
			function: "printf",
		},
		{
			name:     "Should reject a disallowed builtin in argument position",
			template: `{{ if eq html "" }}y{{ end }}`,
			function: "html",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := refs.CompileTemplate("test", tt.template, namespace(true))
			want := `unsupported template function "` + tt.function + `"`
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("CompileTemplate() error = %v, want %s", err, want)
			}
		})
	}
}

func TestTemplateShouldRouteOnComparisonAndLogicBuiltins(t *testing.T) {
	t.Parallel()

	raw := `{{ if eq .nodes.review.output.verdict "fail" }}fail{{ else if ne .nodes.review.output.verdict "" }}other{{ else }}none{{ end }}` +
		`|{{ if and (not .inputs.done) (or (lt (len .inputs.tags) 3) (ge (len .inputs.tags) 9)) }}open{{ end }}` +
		`|{{ if and (le (len .inputs.tags) 2) (gt (len .inputs.tags) 1) }}pair{{ end }}`
	compiled, err := refs.CompileTemplate("compare", raw, namespace(false))
	if err != nil {
		t.Fatalf("CompileTemplate() error = %v", err)
	}
	if len(compiled.References) < 4 {
		t.Fatalf("References len = %d, want at least 4", len(compiled.References))
	}

	tests := []struct {
		name    string
		verdict string
		want    string
	}{
		{name: "Should route a failing verdict", verdict: "fail", want: "fail|open|pair"},
		{name: "Should route another verdict", verdict: "pass", want: "other|open|pair"},
		{name: "Should route the zero-value verdict of a node that did not run", verdict: "", want: "none|open|pair"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rendered, err := refs.RenderTemplateString("compare", raw, map[string]any{
				"inputs": map[string]any{"done": false, "tags": []string{"one", "two"}},
				"nodes":  map[string]any{"review": map[string]any{"output": map[string]any{"verdict": tt.verdict}}},
			})
			if err != nil {
				t.Fatalf("RenderTemplateString() error = %v", err)
			}
			if rendered != tt.want {
				t.Fatalf("RenderTemplateString() = %q, want %q", rendered, tt.want)
			}
		})
	}
}

func TestTemplateShouldCompareNumbersByValueAcrossDecodedKinds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		template string
		value    any
		want     string
	}{
		{
			name:     "Should order json.Number against an integer literal",
			template: `{{ lt .n 3 }}`,
			value:    json.Number("2"),
			want:     "true",
		},
		{
			name:     "Should order json.Number numerically, not lexically",
			template: `{{ lt .n 9 }}`,
			value:    json.Number("10"),
			want:     "false",
		},
		{
			name:     "Should order json.Number against a float literal",
			template: `{{ ge .n 2.5 }}`,
			value:    json.Number("2.5"),
			want:     "true",
		},
		{
			name:     "Should equate float64 with an integer literal",
			template: `{{ eq .n 3 }}`,
			value:    float64(3),
			want:     "true",
		},
		{
			name:     "Should equate a schema zero value with zero",
			template: `{{ eq .n 0 }}`,
			value:    json.Number("0"),
			want:     "true",
		},
		{
			name:     "Should compare matching decimal with eq",
			template: `{{ eq .n 0.1 }}`,
			value:    json.Number("0.1"),
			want:     "true",
		},
		{
			name:     "Should compare matching decimal with ne",
			template: `{{ ne .n 0.1 }}`,
			value:    json.Number("0.1"),
			want:     "false",
		},
		{
			name:     "Should compare matching decimal with lt",
			template: `{{ lt .n 0.1 }}`,
			value:    json.Number("0.1"),
			want:     "false",
		},
		{
			name:     "Should compare matching decimal with le",
			template: `{{ le .n 0.1 }}`,
			value:    json.Number("0.1"),
			want:     "true",
		},
		{
			name:     "Should compare matching decimal with gt",
			template: `{{ gt .n 0.1 }}`,
			value:    json.Number("0.1"),
			want:     "false",
		},
		{
			name:     "Should compare matching decimal with ge",
			template: `{{ ge .n 0.1 }}`,
			value:    json.Number("0.1"),
			want:     "true",
		},
		{
			name:     "Should compare float32 using its decimal representation",
			template: `{{ eq .n 0.1 }}`,
			value:    float32(0.1),
			want:     "true",
		},
		{
			name:     "Should equate an exact whole float with an integer",
			template: `{{ eq .n 1152921504606846976 }}`,
			value:    float64(1152921504606846976),
			want:     "true",
		},
		{
			name:     "Should equate a JSON integer with an exact whole float literal",
			template: `{{ eq .n 1152921504606846976.0 }}`,
			value:    json.Number("1152921504606846976"),
			want:     "true",
		},
		{
			name:     "Should order an exact whole float literal",
			template: `{{ lt .n 1152921504606846976.0 }}`,
			value:    json.Number("1152921504606846975"),
			want:     "true",
		},
		{
			name:     "Should preserve a negative whole float",
			template: `{{ eq .n -1152921504606846976 }}`,
			value:    float64(-1152921504606846976),
			want:     "true",
		},
		{
			name:     "Should equate exponent notation",
			template: `{{ eq .n 100 }}`,
			value:    json.Number("1e2"),
			want:     "true",
		},
		{
			name:     "Should accept the upper exponent bound",
			template: `{{ gt .n 1 }}`,
			value:    json.Number("1e4096"),
			want:     "true",
		},
		{
			name:     "Should accept the lower exponent bound",
			template: `{{ lt .n 1 }}`,
			value:    json.Number("1E-4096"),
			want:     "true",
		},
		{
			name:     "Should preserve decimal digits beyond float precision",
			template: `{{ gt .n 0.1 }}`,
			value: json.Number(
				"0.10000000000000000000000000000000000000000000000000000000000000000000000000000000001",
			),
			want: "true",
		},
		{name: "Should match any of several eq operands", template: `{{ eq .n 1 2 3 }}`, value: int64(2), want: "true"},
		{name: "Should compare unequal mixed numeric kinds", template: `{{ ne .n 3 }}`, value: uint8(4), want: "true"},
		{name: "Should order strings lexically", template: `{{ gt .n "a" }}`, value: "b", want: "true"},
		{name: "Should compare booleans for equality", template: `{{ eq .n true }}`, value: true, want: "true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rendered, err := refs.RenderTemplateString("numeric", tt.template, map[string]any{"n": tt.value})
			if err != nil {
				t.Fatalf("RenderTemplateString() error = %v", err)
			}
			if rendered != tt.want {
				t.Fatalf("RenderTemplateString() = %q, want %q", rendered, tt.want)
			}
		})
	}

	for _, builtin := range []string{"eq", "ne", "lt", "le", "gt", "ge"} {
		t.Run("Should compare exact JSON numbers with "+builtin, func(t *testing.T) {
			t.Parallel()
			data := map[string]any{
				"n": json.Number("115792089237316195423570985008687907853269984665640564039457584007913129639936"),
				"m": json.Number("115792089237316195423570985008687907853269984665640564039457584007913129639937"),
			}
			want := builtin == "ne" || builtin == "lt" || builtin == "le"
			rendered, err := refs.RenderTemplateString("numeric", "{{ "+builtin+" .n .m }}", data)
			if err != nil || rendered != fmt.Sprint(want) {
				t.Fatalf("RenderTemplateString() = %q, %v, want %v", rendered, err, want)
			}
		})
		t.Run("Should order two JSON numbers numerically with "+builtin, func(t *testing.T) {
			t.Parallel()
			data := map[string]any{"n": json.Number("10"), "m": json.Number("9")}
			want := builtin == "ne" || builtin == "gt" || builtin == "ge"
			rendered, err := refs.RenderTemplateString("numeric", "{{ "+builtin+" .n .m }}", data)
			if err != nil || rendered != fmt.Sprint(want) {
				t.Fatalf("RenderTemplateString() = %q, %v, want %v", rendered, err, want)
			}
		})
		for _, value := range []string{"invalid", "1e4097", "1e-4097", "1e99999999999999999999"} {
			t.Run("Should reject invalid numeric comparison "+builtin+" "+value, func(t *testing.T) {
				t.Parallel()
				_, err := refs.RenderTemplateString(
					"numeric",
					"{{ "+builtin+" .n .n }}",
					map[string]any{"n": json.Number(value)},
				)
				if err == nil || !strings.Contains(err.Error(), "incompatible types for comparison") {
					t.Fatalf("RenderTemplateString() error = %v, want incompatible types for comparison", err)
				}
			})
		}

		for _, value := range []any{float64(3), json.Number("3")} {
			for _, expression := range []string{builtin + ` .n "3"`, builtin + ` "3" .n`} {
				t.Run(
					fmt.Sprintf("Should reject numeric text comparison %T %v %s", value, value, expression),
					func(t *testing.T) {
						t.Parallel()
						_, err := refs.RenderTemplateString(
							"numeric",
							"{{ "+expression+" }}",
							map[string]any{"n": value},
						)
						if err == nil || !strings.Contains(err.Error(), "incompatible types for comparison") {
							t.Fatalf("RenderTemplateString() error = %v, want incompatible types for comparison", err)
						}
					},
				)
			}
		}
	}
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
		{name: "Should accept the previous nodes collection", path: []string{"previous", "nodes"}},
		{name: "Should accept a previous node guard", path: []string{"previous", "nodes", "load"}},
		{name: "Should accept a previous verdict guard", path: []string{"previous", "verdicts", "quality"}},
		{name: "Should accept the previous generation", path: []string{"previous", "generation"}},
		{
			name:     "Should reject an unknown previous gate",
			path:     []string{"previous", "verdicts", "missing"},
			wantCode: refs.CodeUnknownReference,
		},
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
		{
			name: "Should accept qualified fan-out progress",
			path: []string{"nodes", "fan", "progress", "success_rate"},
			namespace: refs.Namespace{Nodes: map[string]refs.NodeSchema{
				"fan": {HasProgress: true},
			}},
		},
		{
			name:      "Should accept bare progress in fan-out body",
			path:      []string{"progress", "pending"},
			namespace: refs.Namespace{AllowProgress: true},
		},
		{
			name:      "Should reject bare progress outside fan-out body",
			path:      []string{"progress", "pending"},
			namespace: refs.Namespace{},
			wantCode:  refs.CodeItemOutsideFanout,
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
				HasOutput: true, OutputJSONSchema: true,
				Output: refs.Schema{"type": "object", "properties": map[string]any{
					"tasks": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"title": map[string]any{"type": "string"},
							},
						},
					},
				}},
			},
			"review": {
				HasOutput: true, OutputJSONSchema: true,
				Output: refs.Schema{"type": "object", "properties": map[string]any{
					"verdict": map[string]any{"type": "string"},
				}},
			},
			"gate": {
				HasOutput: true, OutputJSONSchema: true,
				Output: refs.Schema{"type": "object", "properties": map[string]any{
					"blocking_issues": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"id": map[string]any{"type": "string"},
							},
						},
					},
				}},
			},
		},
		Gates:       map[string]struct{}{"quality": {}},
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

func TestNamespaceSchemaNamedFields(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		jsonSchema     bool
		schema         refs.Schema
		valid, invalid []string
	}{
		{name: "Should resolve shorthand fields named type", schema: refs.Schema{"type": "string", "id": "string"}, valid: []string{"type", "id"}, invalid: []string{"missing"}},
		{name: "Should resolve nested shorthand type maps", schema: refs.Schema{"item": map[string]any{"type": map[string]any{"name": "string"}, "id": "string"}}, valid: []string{"item.type.name", "item.id"}, invalid: []string{"item.missing"}},
		{name: "Should resolve transform any fields", schema: refs.Schema{"type": "any", "id": "any"}, valid: []string{"type", "id"}},
		{jsonSchema: true, name: "Should reject JSON Schema keywords as fields", schema: refs.Schema{"type": "object", "additionalProperties": true}, invalid: []string{"type", "additionalProperties", "missing"}},
		{jsonSchema: true, name: "Should follow declared JSON Schema properties", schema: refs.Schema{"type": "object", "properties": map[string]any{"type": map[string]any{"type": "string"}}, "additionalProperties": false}, valid: []string{"type"}, invalid: []string{"properties", "type.type", "missing"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ns := refs.Namespace{
				Nodes: map[string]refs.NodeSchema{
					"a": {Output: tc.schema, HasOutput: true, OutputJSONSchema: tc.jsonSchema},
				},
			}
			for _, field := range tc.valid {
				if err := ns.ValidatePath(
					append([]string{"nodes", "a", "output"}, strings.Split(field, ".")...),
				); err != nil {
					t.Fatalf("%s: %v", field, err)
				}
			}
			for _, field := range tc.invalid {
				err := ns.ValidatePath(append([]string{"nodes", "a", "output"}, strings.Split(field, ".")...))
				if err == nil || err.Code != refs.CodeUnresolvablePath {
					t.Fatalf("%s: %v", field, err)
				}
			}
		})
	}
}
