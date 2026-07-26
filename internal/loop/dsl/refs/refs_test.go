package refs_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/loop/dsl/refs"
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
	var refErr *refs.Error
	if !errors.As(err, &refErr) {
		t.Fatalf("error = %T %v, want *refs.Error", err, err)
	}
	if refErr.Code != code {
		t.Fatalf("error code = %s, want %s", refErr.Code, code)
	}
}

func pathString(path []string) string {
	return strings.Join(path, ".")
}
