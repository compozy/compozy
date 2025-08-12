package tplengine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"html"
	html_template "html/template"
)

func TestHasTemplate(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"no_markers", "plain text", false},
		{"with_delims", "Hello {{ .name }}", true},
		{"with_trim_marker", "Hello {{- .name -}}", true},
		{"brace_like_not_template", "Hello {not tmpl}", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasTemplate(tt.in); got != tt.want {
				t.Fatalf("HasTemplate(%q)=%v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestNewEngine_Defaults(t *testing.T) {
	e := NewEngine(FormatText)
	if e == nil {
		t.Fatal("NewEngine returned nil")
	}
	// Ensure fluent setters mutate the engine and are safe to chain
	e = e.WithFormat(FormatJSON).WithPrecisionPreservation(true)
	// RenderString without templates should return input as-is
	out, err := e.RenderString("no templates here", nil)
	if err != nil {
		t.Fatalf("RenderString returned error: %v", err)
	}
	if out != "no templates here" {
		t.Fatalf("RenderString unexpected: %q", out)
	}
}

func TestAddTemplateAndRender_Basic(t *testing.T) {
	e := NewEngine(FormatText)
	if err := e.AddTemplate("hello", "Hello {{ .name }}"); err != nil {
		t.Fatalf("AddTemplate error: %v", err)
	}
	got, err := e.Render("hello", map[string]any{"name": "World"})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if got != "Hello World" {
		t.Fatalf("Render got %q, want %q", got, "Hello World")
	}
}

func TestAddTemplate_MissingKeyErrorsOnExecute(t *testing.T) {
	e := NewEngine(FormatText)
	if err := e.AddTemplate("needs_name", "Hi {{ .name }}"); err != nil {
		t.Fatalf("AddTemplate error: %v", err)
	}
	_, err := e.Render("needs_name", map[string]any{}) // missing .name
	if err == nil {
		t.Fatalf("expected error for missing key, got nil")
	}
	if !strings.Contains(err.Error(), "map has no entry for key") && !strings.Contains(err.Error(), "missing key") {
		t.Fatalf("expected missingkey error, got %v", err)
	}
}

func TestRender_TemplateNotFound(t *testing.T) {
	e := NewEngine(FormatText)
	_, err := e.Render("not-there", nil)
	if err == nil || !strings.Contains(err.Error(), "template not found") {
		t.Fatalf("expected 'template not found' error, got %v", err)
	}
}

func TestRenderString_HtmlSafetyFuncs(t *testing.T) {
	e := NewEngine(FormatText)
	// Validate our exposed helpers behave like html.EscapeString and html_template.JSEscapeString
	in := `<script>alert("x")</script>`
	esc := html.EscapeString(in)
	jesc := html_template.JSEscapeString(in)

	out, err := e.RenderString(`{{ .val | htmlEscape }}`, map[string]any{"val": in})
	if err != nil {
		t.Fatalf("RenderString error: %v", err)
	}
	if out != esc {
		t.Fatalf("htmlEscape mismatch: got %q want %q", out, esc)
	}

	out, err = e.RenderString(`{{ .val | htmlAttrEscape }}`, map[string]any{"val": in})
	if err != nil {
		t.Fatalf("RenderString error: %v", err)
	}
	if out != esc {
		t.Fatalf("htmlAttrEscape mismatch: got %q want %q", out, esc)
	}

	out, err = e.RenderString(`{{ .val | jsEscape }}`, map[string]any{"val": in})
	if err != nil {
		t.Fatalf("RenderString error: %v", err)
	}
	if out != jesc {
		t.Fatalf("jsEscape mismatch: got %q want %q", out, jesc)
	}
}

func TestRenderString_SprigFunctionAvailable(t *testing.T) {
	e := NewEngine(FormatText)
	out, err := e.RenderString(`{{ "hello" | upper }}`, nil)
	if err != nil {
		t.Fatalf("RenderString error: %v", err)
	}
	if out != "HELLO" {
		t.Fatalf("sprig upper mismatch: got %q want %q", out, "HELLO")
	}
}

func TestRenderString_HyphenatedKeys(t *testing.T) {
	e := NewEngine(FormatText)
	tmpl := `Hi {{ .user-name.first_name }}, id={{ .user-name.id }}`
	ctx := map[string]any{
		"user-name": map[string]any{
			"first_name": "Ada",
			"id":         42,
		},
	}
	out, err := e.RenderString(tmpl, ctx)
	if err != nil {
		t.Fatalf("RenderString error: %v", err)
	}
	want := "Hi Ada, id=42"
	if out != want {
		t.Fatalf("got %q want %q", out, want)
	}
}

func TestRenderString_BooleanStringPreserved(t *testing.T) {
	e := NewEngine(FormatText)
	out, err := e.RenderString(`{{ eq 1 1 }}`, nil)
	if err != nil {
		t.Fatalf("RenderString error: %v", err)
	}
	if out != "true" {
		t.Fatalf("got %q want %q", out, "true")
	}
}

func TestProcessString_SuccessAndNonStringResult(t *testing.T) {
	e := NewEngine(FormatText)
	// Happy path returning string
	out, err := e.ProcessString("Hello {{ .who }}", map[string]any{"who": "World"})
	if err != nil {
		t.Fatalf("ProcessString error: %v", err)
	}
	if out != "Hello World" {
		t.Fatalf("got %q want %q", out, "Hello World")
	}

	// Template renders JSON which renderAndProcessTemplate parses to map, causing ProcessString to error
	jsonTmpl := `{{ "{\"a\":1}" }}`
	_, err = e.ProcessString(jsonTmpl, nil)
	if err == nil {
		t.Fatalf("expected error when result is not a string")
	}
	// Error text is wrapped; just ensure it's the right function producing it
	if !strings.Contains(err.Error(), "failed to parse template string") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProcessFile_DetectsFormatAndProcesses(t *testing.T) {
	dir := t.TempDir()
	// YAML-like file; content is plain text for our purposes
	yamlPath := filepath.Join(dir, "x.yaml")
	if err := os.WriteFile(yamlPath, []byte("Val: {{ .v }}"), 0o600); err != nil {
		t.Fatalf("write temp yaml: %v", err)
	}
	e := NewEngine("") // no format set, should detect from extension
	got, err := e.ProcessFile(yamlPath, map[string]any{"v": 7})
	if err != nil {
		t.Fatalf("ProcessFile error: %v", err)
	}
	if got != "Val: 7" {
		t.Fatalf("got %q want %q", got, "Val: 7")
	}

	// Nonexistent file returns clear error
	_, err = e.ProcessFile(filepath.Join(dir, "missing.json"), nil)
	if err == nil || !strings.Contains(err.Error(), "failed to read template file") {
		t.Fatalf("expected file read error, got %v", err)
	}
}

func TestParseAny_Types(t *testing.T) {
	e := NewEngine(FormatText)

	// nil
	if v, err := e.ParseAny(nil, nil); err != nil || v != nil {
		t.Fatalf("nil parse got %v,%v", v, err)
	}

	// string without template stays string
	if v, err := e.ParseAny("abc", nil); err != nil || v != "abc" {
		t.Fatalf("string parse got %v,%v", v, err)
	}

	// []any recursively processed
	inArr := []any{"x {{ .y }}", 2}
	outArr, err := e.ParseAny(inArr, map[string]any{"y": "Y"})
	if err != nil {
		t.Fatalf("ParseAny arr error: %v", err)
	}
	arr, ok := outArr.([]any)
	if !ok || len(arr) != 2 || arr[0] != "x Y" || arr[1] != 2 {
		t.Fatalf("array parse mismatch: %#v", outArr)
	}

	// map[string]any recursively processed
	inMap := map[string]any{"a": "hi {{ .b }}", "c": 3}
	outMapVal, err := e.ParseAny(inMap, map[string]any{"b": "B"})
	if err != nil {
		t.Fatalf("ParseAny map error: %v", err)
	}
	outMap, ok := outMapVal.(map[string]any)
	if !ok || outMap["a"] != "hi B" || outMap["c"] != 3 {
		t.Fatalf("map parse mismatch: %#v", outMapVal)
	}
}

func TestContainsRuntimeReferences(t *testing.T) {
	if !containsRuntimeReferences("{{ .tasks.t1.output }}") {
		t.Fatalf("expected runtime reference detection")
	}
	if containsRuntimeReferences("{{ .input.x }}") {
		t.Fatalf("did not expect runtime reference detection")
	}
}

func TestExtractTaskReferences(t *testing.T) {
	in := "pre {{ .tasks.alpha.output }} mid {{ .tasks.beta-id[0] }} post {{ .tasks.gamma }} done"
	got := extractTaskReferences(in)
	// Order should be alpha, beta-id, gamma
	want := []string{"alpha", "beta-id", "gamma"}
	if len(got) != len(want) {
		t.Fatalf("len mismatch: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("idx %d got %q want %q", i, got[i], want[i])
		}
	}
}

func TestAreAllTasksAvailable(t *testing.T) {
	tasks := map[string]any{"a": 1, "b": 2}
	if !areAllTasksAvailable([]string{"a"}, tasks) {
		t.Fatalf("expected available")
	}
	if areAllTasksAvailable([]string{"a", "c"}, tasks) {
		t.Fatalf("expected unavailable")
	}
}

func TestParseStringWithFilter_DeferredResolution(t *testing.T) {
	e := NewEngine(FormatText)
	// Reference to tasks; tasks missing => should be deferred (string unchanged)
	v := "{{ .tasks.t1.output }}"
	got, err := e.parseStringWithFilter(v, map[string]any{"input": map[string]any{"x": 1}})
	if err != nil {
		t.Fatalf("parseStringWithFilter error: %v", err)
	}
	if s, ok := got.(string); !ok || s != v {
		t.Fatalf("expected unresolved template to pass through, got %#v", got)
	}

	// When tasks map includes t1, resolve to concrete value
	ctx := map[string]any{
		"tasks": map[string]any{
			"t1": map[string]any{
				"output": "OK",
			},
		},
	}
	got, err = e.parseStringWithFilter(v, ctx)
	if err != nil {
		t.Fatalf("parseStringWithFilter error: %v", err)
	}
	if got != "OK" {
		t.Fatalf("expected resolved value, got %#v", got)
	}
}

func TestCanResolveTaskReferencesNow_VariousTaskTypes(t *testing.T) {
	e := NewEngine(FormatText)
	v := "{{ .tasks.x.output }}"
	// nil context
	if e.canResolveTaskReferencesNow(v, nil) {
		t.Fatalf("nil context should not resolve")
	}
	// missing tasks
	if e.canResolveTaskReferencesNow(v, map[string]any{"input": 1}) {
		t.Fatalf("missing tasks should not resolve")
	}
	// wrong type
	if e.canResolveTaskReferencesNow(v, map[string]any{"tasks": 123}) {
		t.Fatalf("unsupported tasks type should not resolve")
	}
	// map value present
	if !e.canResolveTaskReferencesNow(v, map[string]any{"tasks": map[string]any{"x": map[string]any{}}}) {
		t.Fatalf("map tasks should resolve")
	}
	// pointer to map
	tasksMap := map[string]any{"x": 1}
	if !e.canResolveTaskReferencesNow(v, map[string]any{"tasks": &tasksMap}) {
		t.Fatalf("ptr map tasks should resolve")
	}
}

func TestParseMapWithFilter_AndSliceWithFilter(t *testing.T) {
	e := NewEngine(FormatText)
	input := map[string]any{
		"keep":   "raw {{ .x }}", // filtered out, left unprocessed
		"proc":   "hey {{ .x }}",
		"nested": map[string]any{"skip": "raw {{ .x }}", "go": "ok {{ .x }}"},
		"arr":    []any{"raw0 {{ .x }}", "raw1 {{ .x }}", "go2 {{ .x }}"},
	}
	filter := func(k string) bool {
		return k == "keep" || k == "skip" || k == "1" // skip arr[1]
	}
	gotAny, err := e.ParseMapWithFilter(input, map[string]any{"x": "X"}, filter)
	if err != nil {
		t.Fatalf("ParseMapWithFilter error: %v", err)
	}
	got := gotAny.(map[string]any)

	if got["keep"] != "raw {{ .x }}" {
		t.Fatalf("expected keep unchanged, got %#v", got["keep"])
	}
	if got["proc"] != "hey X" {
		t.Fatalf("expected proc processed, got %#v", got["proc"])
	}
	nested := got["nested"].(map[string]any)
	if nested["skip"] != "raw {{ .x }}" {
		t.Fatalf("expected nested.skip unchanged, got %#v", nested["skip"])
	}
	if nested["go"] != "ok X" {
		t.Fatalf("expected nested.go processed, got %#v", nested["go"])
	}
	arr := got["arr"].([]any)
	if arr[0] != "raw0 X" { // processed (not skipped)
		t.Fatalf("arr[0] got %#v", arr[0])
	}
	if arr[1] != "raw1 {{ .x }}" { // skipped
		t.Fatalf("arr[1] got %#v", arr[1])
	}
	if arr[2] != "go2 X" {
		t.Fatalf("arr[2] got %#v", arr[2])
	}
}

func TestParseWithJSONHandling(t *testing.T) {
	e := NewEngine(FormatText)

	// String that looks like JSON (no templates) parses then processes nested templates
	jsonStr := `{"a":"{{ .x }}","b":[1,2]}`
	got, err := e.ParseWithJSONHandling(jsonStr, map[string]any{"x": "X"})
	if err != nil {
		t.Fatalf("ParseWithJSONHandling error: %v", err)
	}
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %#v", got)
	}
	if m["a"] != "X" {
		t.Fatalf("nested template not processed, got %#v", m["a"])
	}

	// Template that resolves to JSON string
	got, err = e.ParseWithJSONHandling(`{{ "{\"k\":\"v\"}" }}`, nil)
	if err != nil {
		t.Fatalf("ParseWithJSONHandling error: %v", err)
	}
	m, ok = got.(map[string]any)
	if !ok || m["k"] != "v" {
		t.Fatalf("expected parsed json map, got %#v", got)
	}

	// Plain string non-JSON unchanged
	got, err = e.ParseWithJSONHandling("abc", nil)
	if err != nil {
		t.Fatalf("ParseWithJSONHandling error: %v", err)
	}
	if got.(string) != "abc" {
		t.Fatalf("expected unchanged string, got %#v", got)
	}
}

func TestIsSimpleObjectReference(t *testing.T) {
	e := NewEngine(FormatText)
	cases := []struct {
		in   string
		want bool
	}{
		{"{{ .tasks.x.output }}", true},
		{" {{ .tasks.x.output }} ", true},
		{"{{ .x | upper }}", false},       // has filter
		{"{{ .x .y }}", false},            // space
		{"{{ .x}} trailing", false},       // extra text
		{".x", false},                     // not wrapped
		{"{{ not_a_ref }}", false},        // no leading dot inside
	}
	for _, tc := range cases {
		if got := e.isSimpleObjectReference(tc.in); got != tc.want {
			t.Fatalf("isSimpleObjectReference(%q) = %v want %v", tc.in, got, tc.want)
		}
	}
}

func TestExtractObjectFromContext_TraversalAndTypes(t *testing.T) {
	e := NewEngine(FormatText)

	// Plain maps
	ctx := map[string]any{"a": map[string]any{"b": map[string]any{"c": 5}}}
	if got := e.extractObjectFromContext("{{ .a.b.c }}", ctx); got != 5 {
		t.Fatalf("extractObjectFromContext got %#v", got)
	}

	// Pointer to map
	m := map[string]any{"b": map[string]any{"c": "ok"}}
	ctx2 := map[string]any{"a": &m}
	if got := e.extractObjectFromContext("{{ .a.b.c }}", ctx2); got != "ok" {
		t.Fatalf("extractObjectFromContext ptr map got %#v", got)
	}
}

func TestPrepareValueForTemplate_CoreOutputHandling(t *testing.T) {
	e := NewEngine(FormatText)

	// Emulate core.Output as map[string]any. If the project defines a real type alias,
	// these lines still compile; otherwise we simulate with map casting via the interface used.
	type coreOutput map[string]any
	// The engine's prepareValueForTemplate has explicit handling for core.Output,
	// but in tests we cannot import internal core package without knowing module path.
	// We can still ensure that normal objects are returned as-is.
	obj := map[string]any{"x": 1}
	got, err := e.prepareValueForTemplate(obj)
	if err != nil {
		t.Fatalf("prepareValueForTemplate error: %v", err)
	}
	if got.(map[string]any)["x"] != 1 {
		t.Fatalf("prepareValueForTemplate mismatch: %#v", got)
	}

	// Also ensure that when used through isSimpleObjectReference -> extractObjectFromContext path,
	// the type is preserved (e.g., []any stays slice).
	ctx := map[string]any{"obj": []any{1, 2, 3}}
	res, err := e.parseStringValue("{{ .obj }}", ctx)
	if err != nil {
		t.Fatalf("parseStringValue error: %v", err)
	}
	slice, ok := res.([]any)
	if !ok || len(slice) != 3 || slice[2] != 3 {
		t.Fatalf("expected slice preservation, got %#v", res)
	}

	_ = coreOutput{} // avoid unused type warning if alias not used
}

func TestRenderAndProcessTemplate_JSONAutoParsing(t *testing.T) {
	e := NewEngine(FormatText)
	// Template produces JSON; should auto-parse to map
	val, err := e.renderAndProcessTemplate(`{{ "{\"a\":1,\"b\":[true,false]}" }}`, nil)
	if err != nil {
		t.Fatalf("renderAndProcessTemplate error: %v", err)
	}
	m, ok := val.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %#v", val)
	}
	b, _ := json.Marshal(m)
	if !strings.Contains(string(b), `"a":1`) {
		t.Fatalf("unexpected parsed json: %s", string(b))
	}
}

func TestPreprocessContext_DefaultsPresent(t *testing.T) {
	e := NewEngine(FormatText)
	got := e.preprocessContext(map[string]any{"custom": 1})
	required := []string{"env", "input", "output", "trigger", "tools", "tasks", "agents"}
	for _, k := range required {
		if _, ok := got[k]; !ok {
			t.Fatalf("missing default key %q in context", k)
		}
	}
	if got["custom"] != 1 {
		t.Fatalf("custom key lost")
	}
}

func TestRenderTemplate_MergesGlobalValues(t *testing.T) {
	e := NewEngine(FormatText)
	e.AddGlobalValue("greeting", "Hello")
	out, err := e.RenderString("{{ .greeting }} {{ .name }}", map[string]any{"name": "Ada"})
	if err != nil {
		t.Fatalf("RenderString error: %v", err)
	}
	if out != "Hello Ada" {
		t.Fatalf("expected merged globals, got %q", out)
	}
}

func TestPreprocessTemplateForHyphens_ComplexPathsInsideBlock(t *testing.T) {
	e := NewEngine(FormatText)
	// Validate hyphen handling inside conditionals and mixed content
	tmpl := `
{{- if .user-profile.enabled -}}
User: {{ .user-profile.name.first }} {{ .user-profile.name.last }}
{{- else -}}
Disabled
{{- end -}}`
	ctx := map[string]any{
		"user-profile": map[string]any{
			"enabled": true,
			"name": map[string]any{
				"first": "Grace",
				"last":  "Hopper",
			},
		},
	}
	out, err := e.RenderString(tmpl, ctx)
	if err != nil {
		t.Fatalf("RenderString error: %v", err)
	}
	want := "User: Grace Hopper"
	if !strings.Contains(out, want) {
		t.Fatalf("output %q does not contain %q", out, want)
	}
}

func TestErrorWrapsFromExecution(t *testing.T) {
	e := NewEngine(FormatText)
	// Invalid template syntax should be caught on Parse
	err := e.AddTemplate("bad", "{{ if .x }} unclosed ")
	if err == nil {
		t.Fatalf("expected parse error for invalid template")
	}
	// Execution error bubbles with clear wrapping message through RenderString path
	_, err = e.RenderString("{{ .missing }}", nil)
	if err == nil {
		t.Fatalf("expected execution error for missing key")
	}
	if !strings.Contains(err.Error(), "template execution error") && !strings.Contains(err.Error(), "map has no entry for key") {
		t.Fatalf("unexpected exec error: %v", err)
	}
}

// Note: This test file uses the Go standard testing package.
// No external testing framework is introduced to align with repository conventions.
