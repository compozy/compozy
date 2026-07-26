package sdkts

import (
	"encoding/json"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/hooks"
)

type EmbeddedJSONFields struct {
	Embedded string `json:"embedded,omitempty"`
	Ignored  string `json:"-"`
}

type ContainerJSONFields struct {
	EmbeddedJSONFields
	Alias    string          `json:",omitempty"`
	Optional *string         `json:"optional,omitempty"`
	Raw      json.RawMessage `json:"raw"`
}

type JSONTagFixture struct {
	DefaultName string
	Ignored     string `json:"-"`
	Explicit    string `json:"explicit,omitempty"`
	BlankName   string `json:",omitempty"`
}

func TestGenerateDeterministicAndStructured(t *testing.T) {
	t.Parallel()

	t.Run("Should generate deterministic structured contracts", func(t *testing.T) {
		t.Parallel()

		first, err := Generate()
		if err != nil {
			t.Fatalf("Generate() first call error = %v", err)
		}
		second, err := Generate()
		if err != nil {
			t.Fatalf("Generate() second call error = %v", err)
		}
		if first != second {
			t.Fatal("Generate() output changed between calls")
		}

		requiredSnippets := []string{
			generatedHeader,
			"export interface BridgeInstance {\n",
			"export interface BridgeInstanceTargetParams {\n",
			"export interface BridgesInstancesReportStateParams {\n",
			"export interface BridgesMessagesIngestResult {\n",
			"export interface InboundMessageEnvelope {\n",
			`export type DeliveryEventType = "start" | "delta" | "final" | "error" | "resume" | "delete" | "progress";`,
			`export type DeliveryAckOutcome = "success" | "committed_result_unavailable";`,
			"export interface DeliveryAck {\n  delivery_id: string;\n  seq: number;\n",
			`export type InboundEventFamily = "message" | "command" | "action" | "reaction" | "edit";`,
			`export type InboundEditOperation = "updated" | "deleted";`,
			"export interface InboundEdit {\n",
			"edit?: InboundEdit;",
			"reply_to_text?: string;",
			"reply_to_author_id?: string;",
			"reply_to_author_name?: string;",
			`export type ReasoningEffort = "none" | "minimal" | "low" | "medium" | "high" | "xhigh" | "max";`,
			`export type CostStatus = "actual" | "estimated" | "included" | "unknown";`,
			`export type CostSource = "agent_reported" | "catalog_config" | "models_dev" | "builtin" | "none";`,
			`export type ToolProgressPhase = "started" | "completed" | "failed";`,
			`export type BridgeRuntimePurpose = "service" | "control";`,
			`export type ControlMethod = "bridges/check" | "bridges/webhook/register";`,
			`export type BridgeCheckStatus = "pass" | "warn" | "fail" | "skipped";`,
			"export interface BridgeCheckRecord {\n",
			"export interface BridgeWebhookRegistrationResponse {\n",
			"reasoning_effort?: ReasoningEffort;",
			"reasoning_efforts?: ReasoningEffort[];",
			"cost_status?: CostStatus;",
			"cost_source?: CostSource;",
			"export interface HookPayloadByEvent {\n",
			"export interface HookPatchByEvent {\n",
			"export interface HostAPIMethodMap {\n",
			"export interface HookDecl {\n",
		}
		for _, snippet := range requiredSnippets {
			if !strings.Contains(first, snippet) {
				t.Fatalf("Generate() output missing %q", snippet)
			}
		}
		if !strings.HasSuffix(first, "\n") {
			t.Fatal("Generate() output must end with a single trailing newline")
		}
		if strings.HasSuffix(first, "\n\n") {
			t.Fatal("Generate() output must not end with multiple trailing newlines")
		}
	})
}

func TestGeneratePreservesParticipationDiscriminatedUnions(t *testing.T) {
	t.Parallel()

	source, err := Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	requiredSnippets := []string{
		`export type NetworkParticipationMode = "local" | "live";`,
		`export type NetworkParticipationChannelStrategy = "named" | "run" | "loop_run";`,
		`export type NetworkParticipationSource = "explicit_request" | "task_profile" | "workspace_coordination" | "loop_definition" | "automation_job" | "built_in_local";`,
		`export type NetworkParticipationOwnerKind = "session" | "task_run" | "loop_run" | "automation_run";`,
		"export type NetworkParticipationRequest = \n  | {\n      mode?: \"local\";",
		"mode: \"live\";\n      channel_strategy: \"named\";\n      channel_id: string;",
		"mode: \"live\";\n      channel_strategy: \"run\";\n      channel_id?: never;",
		"mode: \"live\";\n      channel_strategy: \"loop_run\";\n      channel_id?: never;",
		"export type NetworkParticipationSpec = \n  | {\n      version: \"network-participation/v1\";\n      mode: \"local\";",
		"mode: \"live\";\n      workspace_id: string;\n      channel_strategy: \"named\" | \"run\" | \"loop_run\";",
		"export interface NetworkParticipationOwnerRef {\n  workspace_id: string;\n  kind: NetworkParticipationOwnerKind;\n  id: string;\n}",
		"export interface NetworkParticipationStatus {\n  owner: NetworkParticipationOwnerRef;",
		"export type Status = string;",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("Generate() participation output missing %q", snippet)
		}
	}
	for _, forbidden := range []string{
		"export interface NetworkParticipationRequest {",
		"export interface NetworkParticipationSpec {",
		"participation_status: Status;",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("Generate() participation output contains flattened contract %q", forbidden)
		}
	}
}

func TestNamedBaseType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value any
		want  reflect.Type
	}{
		{name: "Should return nil for nil value", value: nil, want: nil},
		{
			name:  "Should return struct type for struct value",
			value: EmbeddedJSONFields{},
			want:  reflect.TypeFor[EmbeddedJSONFields](),
		},
		{
			name:  "Should unwrap pointer value",
			value: &EmbeddedJSONFields{},
			want:  reflect.TypeFor[EmbeddedJSONFields](),
		},
		{
			name:  "Should unwrap slice element value",
			value: []EmbeddedJSONFields{},
			want:  reflect.TypeFor[EmbeddedJSONFields](),
		},
		{
			name:  "Should unwrap array element value",
			value: [1]EmbeddedJSONFields{},
			want:  reflect.TypeFor[EmbeddedJSONFields](),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := namedBaseType(tt.value); got != tt.want {
				t.Fatalf("namedBaseType(%T) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestJSONFieldName(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeFor[JSONTagFixture]()
	tests := []struct {
		name          string
		field         reflect.StructField
		wantName      string
		wantOmitEmpty bool
	}{
		{
			name:          "Should use default field name",
			field:         typ.Field(0),
			wantName:      "DefaultName",
			wantOmitEmpty: false,
		},
		{
			name:          "Should ignore skipped field",
			field:         typ.Field(1),
			wantName:      "",
			wantOmitEmpty: false,
		},
		{
			name:          "Should use explicit JSON name",
			field:         typ.Field(2),
			wantName:      "explicit",
			wantOmitEmpty: true,
		},
		{
			name:          "Should use field name for blank JSON name",
			field:         typ.Field(3),
			wantName:      "BlankName",
			wantOmitEmpty: true,
		},
		{
			name: "Should treat omitzero as an optional JSON field",
			field: reflect.StructField{
				Name: "Since",
				Tag:  reflect.StructTag(`json:"since,omitzero"`),
			},
			wantName:      "since",
			wantOmitEmpty: true,
		},
		{
			name: "Should preserve untrimmed JSON tag options",
			field: reflect.StructField{
				Name: "SpacedName",
				Tag:  reflect.StructTag(`json:" spaced_name , omitempty ,string"`),
			},
			wantName:      " spaced_name ",
			wantOmitEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotName, gotOmitEmpty := jsonFieldName(tt.field)
			if gotName != tt.wantName || gotOmitEmpty != tt.wantOmitEmpty {
				t.Fatalf(
					"jsonFieldName(%s) = (%q, %t), want (%q, %t)",
					tt.field.Name,
					gotName,
					gotOmitEmpty,
					tt.wantName,
					tt.wantOmitEmpty,
				)
			}
		})
	}
}

func TestStructFieldsFlattensEmbeddedAndRespectsTags(t *testing.T) {
	t.Parallel()

	t.Run("Should flatten embedded fields and respect JSON tags", func(t *testing.T) {
		t.Parallel()

		gen, err := newGenerator()
		if err != nil {
			t.Fatalf("newGenerator() error = %v", err)
		}
		gen.prepare()

		fields, err := gen.structFields(reflect.TypeFor[ContainerJSONFields]())
		if err != nil {
			t.Fatalf("structFields() error = %v", err)
		}

		want := []fieldSpec{
			{Name: "embedded", Type: "string", Optional: true},
			{Name: "Alias", Type: "string", Optional: true},
			{Name: "optional", Type: "string", Optional: true},
			{Name: "raw", Type: "JSONValue", Optional: false},
		}

		if !reflect.DeepEqual(fields, want) {
			t.Fatalf("structFields() = %#v, want %#v", fields, want)
		}
	})
}

func TestTSTypeCoversCompositeShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		typ  reflect.Type
		want string
	}{
		{
			name: "Should unwrap pointer string type",
			typ:  reflect.TypeFor[*string](),
			want: "string",
		},
		{
			name: "Should render raw message slice type",
			typ:  reflect.TypeFor[[]json.RawMessage](),
			want: "JSONValue[]",
		},
		{
			name: "Should render time map type",
			typ:  reflect.TypeFor[map[string]time.Time](),
			want: "Record<string, ISODateTime>",
		},
		{
			name: "Should render inline struct object type",
			typ: reflect.TypeOf(struct {
				Name  string `json:"name"`
				Count *int   `json:"count,omitempty"`
			}{}),
			want: "{ name: string; count?: number }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gen, err := newGenerator()
			if err != nil {
				t.Fatalf("newGenerator() error = %v", err)
			}
			gen.prepare()

			got, err := gen.tsType(tt.typ)
			if err != nil {
				t.Fatalf("tsType() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("tsType(%v) = %q, want %q", tt.typ, got, tt.want)
			}
		})
	}
}

func TestResultIsList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{name: "Should reject nil value", value: nil, want: false},
		{name: "Should accept slice value", value: []string{"a"}, want: true},
		{name: "Should accept array value", value: [1]string{"a"}, want: true},
		{name: "Should reject struct value", value: EmbeddedJSONFields{}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := resultIsList(tt.value); got != tt.want {
				t.Fatalf("resultIsList(%T) = %t, want %t", tt.value, got, tt.want)
			}
		})
	}
}

func TestHookEventFamilyValues(t *testing.T) {
	t.Parallel()

	t.Run("Should match runtime hook event families", func(t *testing.T) {
		t.Parallel()

		got := hookEventFamilyValues()
		want := expectedHookEventFamilyValues()
		if !slices.Equal(got, want) {
			t.Fatalf("hookEventFamilyValues() = %#v, want %#v", got, want)
		}
	})

	t.Run("Should include runtime hook event families in generated union", func(t *testing.T) {
		t.Parallel()

		source, err := Generate()
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		got := typeAliasUnionValues(t, source, "HookEventFamily")
		want := expectedHookEventFamilyValues()
		if !slices.Equal(got, want) {
			t.Fatalf("generated HookEventFamily = %#v, want %#v", got, want)
		}
	})
}

func expectedHookEventFamilyValues() []string {
	seen := map[hooks.HookEventFamily]struct{}{}
	events := hooks.AllHookEvents()
	values := make([]string, 0, len(events))
	for _, event := range events {
		family := event.Family()
		if family == "" {
			continue
		}
		if _, ok := seen[family]; ok {
			continue
		}
		seen[family] = struct{}{}
		values = append(values, string(family))
	}
	return values
}

func typeAliasUnionValues(t *testing.T, source string, typeName string) []string {
	t.Helper()

	prefix := "export type " + typeName + " = "
	_, after, ok := strings.Cut(source, prefix)
	if !ok {
		t.Fatalf("generated source missing %s type alias", typeName)
	}
	body := after
	before, _, ok := strings.Cut(body, ";")
	if !ok {
		t.Fatalf("generated %s type alias missing semicolon", typeName)
	}

	parts := strings.Split(before, "|")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		token := strings.TrimSpace(part)
		value, err := strconv.Unquote(token)
		if err != nil {
			t.Fatalf("generated %s union token %q is not a quoted string: %v", typeName, token, err)
		}
		values = append(values, value)
	}
	return values
}
