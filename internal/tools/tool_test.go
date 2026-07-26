package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type testProvider struct {
	source SourceRef
}

var _ Provider = testProvider{}

func (p testProvider) ID() SourceRef {
	return p.source
}

func (p testProvider) List(_ context.Context, _ Scope) ([]Descriptor, error) {
	return nil, nil
}

func (p testProvider) Resolve(_ context.Context, _ Scope, _ ToolID) (Handle, bool, error) {
	return nil, false, nil
}

type testHandle struct {
	descriptor Descriptor
}

var _ Handle = testHandle{}

func (h testHandle) Descriptor() Descriptor {
	return h.descriptor
}

func (h testHandle) Availability(_ context.Context, _ Scope) Availability {
	return Availability{
		Registered: true,
		Enabled:    true,
		Available:  true,
		Authorized: true,
		Executable: true,
	}
}

func (h testHandle) Call(_ context.Context, _ CallRequest) (ToolResult, error) {
	return ToolResult{Content: []ToolContent{{Type: "text", Text: "ok"}}}, nil
}

func validDescriptor() Descriptor {
	return Descriptor{
		ID:           "agh__skill_view",
		DisplayTitle: "Skill View",
		Description:  "View one skill",
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
		Backend: BackendRef{
			Kind:       BackendNativeGo,
			NativeName: "skill_view",
		},
		Source: SourceRef{
			Kind:  SourceBuiltin,
			Owner: "daemon",
		},
		Visibility:      VisibilityModel,
		Risk:            RiskRead,
		ReadOnly:        true,
		ConcurrencySafe: true,
		MaxResultBytes:  1024,
		Toolsets:        []ToolsetID{"agh__bootstrap"},
		Tags:            []string{"skills"},
		SearchHints:     []string{"skill body"},
	}
}

func requireReason(t *testing.T, err error, want ReasonCode) {
	t.Helper()

	if err == nil {
		t.Fatalf("error = nil, want reason %q", want)
	}
	got, ok := ReasonOf(err)
	if !ok {
		t.Fatalf("ReasonOf(%v) ok = false, want true", err)
	}
	if got != want {
		t.Fatalf("ReasonOf(%v) = %q, want %q", err, got, want)
	}
}

func TestToolIDValidation(t *testing.T) {
	t.Parallel()

	valid := []ToolID{
		"agh__skill_view",
		"mcp__github__create_issue",
		"ext__linear__search",
		"a__b2_c3",
	}
	for _, id := range valid {
		t.Run("Should accept "+id.String(), func(t *testing.T) {
			t.Parallel()

			if err := id.Validate(); err != nil {
				t.Fatalf("ToolID(%q).Validate() error = %v", id, err)
			}
		})
	}

	tooLong := ToolID("agh__" + strings.Repeat("a", 60))
	invalid := []struct {
		name   string
		id     ToolID
		reason ReasonCode
	}{
		{name: "Should reject empty ids", id: "", reason: ReasonIDEmpty},
		{name: "Should reject dotted ids", id: "agh.skill_view", reason: ReasonIDInvalidFormat},
		{name: "Should reject hyphenated ids", id: "agh__skill-view", reason: ReasonIDInvalidFormat},
		{name: "Should reject uppercase ids", id: "agh__Skill_view", reason: ReasonIDInvalidFormat},
		{name: "Should reject empty segments", id: "agh__", reason: ReasonIDEmptySegment},
		{name: "Should reject reserved separator ambiguity", id: "agh___skill", reason: ReasonIDReservedConflict},
		{name: "Should reject over length ids", id: tooLong, reason: ReasonIDTooLong},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireReason(t, tt.id.Validate(), tt.reason)
		})
	}
}

func TestDescriptorValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should accept valid native descriptors", func(t *testing.T) {
		t.Parallel()

		if err := validDescriptor().Validate(); err != nil {
			t.Fatalf("Descriptor.Validate() error = %v", err)
		}
	})

	tests := []struct {
		name   string
		mutate func(*Descriptor)
		reason ReasonCode
	}{
		{
			name: "Should reject non object input schemas",
			mutate: func(d *Descriptor) {
				d.InputSchema = json.RawMessage(`[]`)
			},
			reason: ReasonSchemaInvalid,
		},
		{
			name: "Should reject unsupported string schema type names",
			mutate: func(d *Descriptor) {
				d.InputSchema = json.RawMessage(`{
					"type":"object",
					"properties":{"query":{"type":"strng"}}
				}`)
			},
			reason: ReasonSchemaInvalid,
		},
		{
			name: "Should reject unsupported array schema type names",
			mutate: func(d *Descriptor) {
				d.InputSchema = json.RawMessage(`{
					"type":"object",
					"properties":{"query":{"type":["string","strng"]}}
				}`)
			},
			reason: ReasonSchemaInvalid,
		},
		{
			name: "Should reject missing extension handlers",
			mutate: func(d *Descriptor) {
				d.Backend = BackendRef{Kind: BackendExtensionHost, ExtensionID: "linear"}
				d.Source = SourceRef{Kind: SourceExtension, Owner: "linear"}
			},
			reason: ReasonHandlerMissing,
		},
		{
			name: "Should reject missing mcp raw provenance",
			mutate: func(d *Descriptor) {
				d.ID = "mcp__github__create_issue"
				d.Backend = BackendRef{Kind: BackendMCP, MCPServer: "github", MCPTool: "create_issue"}
				d.Source = SourceRef{Kind: SourceMCP, Owner: "github"}
			},
			reason: ReasonMCPUnreachable,
		},
		{
			name: "Should reject read only destructive descriptors",
			mutate: func(d *Descriptor) {
				d.Destructive = true
			},
			reason: ReasonPolicyDenied,
		},
		{
			name: "Should reject bridge backend descriptors",
			mutate: func(d *Descriptor) {
				d.Backend = BackendRef{Kind: BackendBridge}
			},
			reason: ReasonBackendNotExecutable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			descriptor := validDescriptor()
			tt.mutate(&descriptor)
			requireReason(t, descriptor.Validate(), tt.reason)
		})
	}
}

func TestShouldValidateIdentifierHelpers(t *testing.T) {
	t.Parallel()

	t.Run("Should expose tool id namespace and segments", func(t *testing.T) {
		t.Parallel()

		id := ToolID("mcp__github__create_issue")
		segments, err := id.Segments()
		if err != nil {
			t.Fatalf("ToolID.Segments() error = %v", err)
		}
		if got, want := strings.Join(segments, ","), "mcp,github,create_issue"; got != want {
			t.Fatalf("ToolID.Segments() = %s, want %s", got, want)
		}
		namespace, err := id.Namespace()
		if err != nil {
			t.Fatalf("ToolID.Namespace() error = %v", err)
		}
		if got, want := namespace, "mcp"; got != want {
			t.Fatalf("ToolID.Namespace() = %q, want %q", got, want)
		}
	})

	t.Run("Should marshal and unmarshal validated ids", func(t *testing.T) {
		t.Parallel()

		encoded, err := ToolID("agh__skill_view").MarshalText()
		if err != nil {
			t.Fatalf("ToolID.MarshalText() error = %v", err)
		}
		if got, want := string(encoded), "agh__skill_view"; got != want {
			t.Fatalf("ToolID.MarshalText() = %q, want %q", got, want)
		}
		var decoded ToolID
		if err := decoded.UnmarshalText([]byte(" agh__skill_view ")); err != nil {
			t.Fatalf("ToolID.UnmarshalText() error = %v", err)
		}
		if decoded != "agh__skill_view" {
			t.Fatalf("decoded ToolID = %q, want agh__skill_view", decoded)
		}
		requireReason(t, decoded.UnmarshalText([]byte("Bad")), ReasonIDInvalidFormat)
	})

	t.Run("Should marshal and unmarshal validated toolsets", func(t *testing.T) {
		t.Parallel()

		encoded, err := ToolsetID("agh__core").MarshalText()
		if err != nil {
			t.Fatalf("ToolsetID.MarshalText() error = %v", err)
		}
		if got, want := string(encoded), "agh__core"; got != want {
			t.Fatalf("ToolsetID.MarshalText() = %q, want %q", got, want)
		}
		var decoded ToolsetID
		if err := decoded.UnmarshalText([]byte(" agh__core ")); err != nil {
			t.Fatalf("ToolsetID.UnmarshalText() error = %v", err)
		}
		if decoded.String() != "agh__core" {
			t.Fatalf("decoded ToolsetID = %q, want agh__core", decoded)
		}
		requireReason(t, decoded.UnmarshalText([]byte("agh.core")), ReasonIDInvalidFormat)
	})

	t.Run("Should canonicalize raw external segments", func(t *testing.T) {
		t.Parallel()

		segment, err := CanonicalIDSegment(" GitHub Create Issue! ")
		if err != nil {
			t.Fatalf("CanonicalIDSegment() error = %v", err)
		}
		if got, want := segment, "github_create_issue"; got != want {
			t.Fatalf("CanonicalIDSegment() = %q, want %q", got, want)
		}
		requireReason(t, fmtErrorFromCanonicalSegment("123"), ReasonIDInvalidFormat)
		requireReason(t, fmtErrorFromCanonicalSegment(""), ReasonIDEmpty)
	})

	t.Run("Should build canonical tool ids without truncation", func(t *testing.T) {
		t.Parallel()

		id, err := CanonicalToolID("mcp", "GitHub", "Create Issue")
		if err != nil {
			t.Fatalf("CanonicalToolID() error = %v", err)
		}
		if got, want := id, ToolID("mcp__github__create_issue"); got != want {
			t.Fatalf("CanonicalToolID() = %q, want %q", got, want)
		}
		_, err = CanonicalToolID("mcp", strings.Repeat("a", 62))
		requireReason(t, err, ReasonIDTooLong)
	})

	t.Run("Should validate json object schemas", func(t *testing.T) {
		t.Parallel()

		if err := ValidateJSONObject("schema", nil, false); err != nil {
			t.Fatalf("ValidateJSONObject(optional nil) error = %v", err)
		}
		if err := ValidateJSONObject("schema", json.RawMessage(`{"type":"object"}`), true); err != nil {
			t.Fatalf("ValidateJSONObject(object) error = %v", err)
		}
		requireReason(t, ValidateJSONObject("schema", nil, true), ReasonSchemaInvalid)
		requireReason(t, ValidateJSONObject("schema", json.RawMessage(`null`), true), ReasonSchemaInvalid)
		requireReason(t, ValidateJSONObject("schema", json.RawMessage(`[]`), true), ReasonSchemaInvalid)
	})
}

func TestShouldValidateDescriptorAndRefBranches(t *testing.T) {
	t.Parallel()

	t.Run("Should convert descriptors to cold tools with cloned schemas", func(t *testing.T) {
		t.Parallel()

		descriptor := validDescriptor()
		descriptor.ToolPresentation = NewToolPresentation("Reading", "arg:name")
		sourcePresentation := descriptor.ToolPresentation
		cold := descriptor.Tool()
		if err := cold.Validate(); err != nil {
			t.Fatalf("Tool.Validate() error = %v", err)
		}
		descriptor.InputSchema[0] = '['
		sourcePresentation.FriendlyVerb = "Changed"
		if got, want := string(cold.InputSchema), `{"type":"object"}`; got != want {
			t.Fatalf("cold.InputSchema = %s, want %s", got, want)
		}
		coldPresentation := cold.Presentation()
		if got, want := coldPresentation.FriendlyVerb, "Reading"; got != want {
			t.Fatalf("cold presentation friendly verb = %q, want %q", got, want)
		}
		roundTrip := cold.Descriptor()
		if got, want := roundTrip.Source.Kind.String(), "builtin"; got != want {
			t.Fatalf("SourceKind.String() = %q, want %q", got, want)
		}
		roundTripPresentation := roundTrip.Presentation()
		if got, want := roundTripPresentation.FriendlyVerb, "Reading"; got != want {
			t.Fatalf("round-trip friendly verb = %q, want %q", got, want)
		}
		if got, want := roundTripPresentation.Preview, "arg:name"; got != want {
			t.Fatalf("round-trip preview = %q, want %q", got, want)
		}
		if got := (Descriptor{}).Presentation(); got != (ToolPresentation{}) {
			t.Fatalf("zero descriptor presentation = %#v, want zero value", got)
		}
	})

	tests := []struct {
		name   string
		mutate func(*Descriptor)
		reason ReasonCode
	}{
		{
			name: "Should reject missing native handler names",
			mutate: func(d *Descriptor) {
				d.Backend = BackendRef{Kind: BackendNativeGo}
			},
			reason: ReasonDependencyMissing,
		},
		{
			name: "Should reject missing extension ids",
			mutate: func(d *Descriptor) {
				d.Backend = BackendRef{Kind: BackendExtensionHost, Handler: "lookup"}
				d.Source = SourceRef{Kind: SourceExtension, Owner: "linear"}
			},
			reason: ReasonExtensionInactive,
		},
		{
			name: "Should reject missing mcp servers",
			mutate: func(d *Descriptor) {
				d.ID = "mcp__github__create_issue"
				d.Backend = BackendRef{Kind: BackendMCP, MCPTool: "create_issue"}
				d.Source = SourceRef{
					Kind:          SourceMCP,
					Owner:         "github",
					RawServerName: "github",
					RawToolName:   "create_issue",
				}
			},
			reason: ReasonMCPUnreachable,
		},
		{
			name: "Should reject missing mcp tool names",
			mutate: func(d *Descriptor) {
				d.ID = "mcp__github__create_issue"
				d.Backend = BackendRef{Kind: BackendMCP, MCPServer: "github"}
				d.Source = SourceRef{
					Kind:          SourceMCP,
					Owner:         "github",
					RawServerName: "github",
					RawToolName:   "create_issue",
				}
			},
			reason: ReasonDependencyMissing,
		},
		{
			name: "Should reject missing source owners",
			mutate: func(d *Descriptor) {
				d.Source = SourceRef{Kind: SourceBuiltin}
			},
			reason: ReasonSourceDisabled,
		},
		{
			name: "Should reject unsupported visibility values",
			mutate: func(d *Descriptor) {
				d.Visibility = Visibility("public")
			},
			reason: ReasonPolicyDenied,
		},
		{
			name: "Should reject unsupported risk values",
			mutate: func(d *Descriptor) {
				d.Risk = RiskClass("unknown")
			},
			reason: ReasonPolicyDenied,
		},
		{
			name: "Should reject negative result budgets",
			mutate: func(d *Descriptor) {
				d.MaxResultBytes = -1
			},
			reason: ReasonResultBudgetExceeded,
		},
		{
			name: "Should reject non object output schemas",
			mutate: func(d *Descriptor) {
				d.OutputSchema = json.RawMessage(`[]`)
			},
			reason: ReasonSchemaInvalid,
		},
		{
			name: "Should wrap invalid toolset ids with their field",
			mutate: func(d *Descriptor) {
				d.Toolsets = []ToolsetID{"agh.bad"}
			},
			reason: ReasonIDInvalidFormat,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			descriptor := validDescriptor()
			tt.mutate(&descriptor)
			requireReason(t, descriptor.Validate(), tt.reason)
		})
	}
}

func TestAvailabilityValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should accept executable availability", func(t *testing.T) {
		t.Parallel()

		availability := Availability{
			Registered: true,
			Enabled:    true,
			Available:  true,
			Authorized: true,
			Executable: true,
		}
		if err := availability.Validate(); err != nil {
			t.Fatalf("Availability.Validate() error = %v", err)
		}
	})

	t.Run("Should reject conflicted availability without conflict reason", func(t *testing.T) {
		t.Parallel()

		requireReason(t, Availability{Conflicted: true}.Validate(), ReasonConflictedID)
	})

	t.Run("Should reject executable unavailable state", func(t *testing.T) {
		t.Parallel()

		requireReason(t, Availability{Executable: true}.Validate(), ReasonBackendNotExecutable)
	})

	t.Run("Should reject available state without prerequisites", func(t *testing.T) {
		t.Parallel()

		requireReason(t, Availability{Available: true}.Validate(), ReasonBackendUnhealthy)
	})

	t.Run("Should reject unknown reason codes", func(t *testing.T) {
		t.Parallel()

		availability := Availability{ReasonCodes: []ReasonCode{"future_reason"}}
		requireReason(t, availability.Validate(), ReasonPolicyDenied)
	})

	t.Run("Should accept conflicted state with deterministic conflict reason", func(t *testing.T) {
		t.Parallel()

		availability := Availability{
			Conflicted:  true,
			ReasonCodes: []ReasonCode{ReasonConflictedSanitizedName},
		}
		if err := availability.Validate(); err != nil {
			t.Fatalf("Availability.Validate() error = %v", err)
		}
	})
}

func TestToolResultValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve safe content metadata", func(t *testing.T) {
		t.Parallel()

		result := ToolResult{
			Content: []ToolContent{{
				Type:     "text",
				Text:     "hello",
				Metadata: map[string]json.RawMessage{"format": json.RawMessage(`"plain"`)},
			}},
			Metadata: map[string]json.RawMessage{"source": json.RawMessage(`"unit"`)},
			Bytes:    5,
		}
		if err := result.Validate(64); err != nil {
			t.Fatalf("ToolResult.Validate() error = %v", err)
		}
		if got := string(result.Content[0].Metadata["format"]); got != `"plain"` {
			t.Fatalf("content metadata was not preserved: %s", got)
		}
	})

	t.Run("Should require truncation for oversized results", func(t *testing.T) {
		t.Parallel()

		requireReason(t, ToolResult{Bytes: 65}.Validate(64), ReasonResultBudgetExceeded)
	})

	t.Run("Should accept truncated oversized results", func(t *testing.T) {
		t.Parallel()

		if err := (ToolResult{Bytes: 65, Truncated: true}).Validate(64); err != nil {
			t.Fatalf("ToolResult.Validate() error = %v", err)
		}
	})

	t.Run("Should reject secret metadata keys", func(t *testing.T) {
		t.Parallel()

		for _, key := range []string{"access_token", "PKCE", "Bearer"} {
			result := ToolResult{
				Metadata: map[string]json.RawMessage{key: json.RawMessage(`"secret"`)},
			}
			requireReason(t, result.Validate(64), ReasonSecretMetadata)
		}
	})

	resultTests := []struct {
		name   string
		result ToolResult
		reason ReasonCode
	}{
		{name: "Should reject negative byte counts", result: ToolResult{Bytes: -1}, reason: ReasonResultBudgetExceeded},
		{name: "Should reject negative durations", result: ToolResult{DurationMS: -1}, reason: ReasonBackendUnhealthy},
		{
			name:   "Should reject content without types",
			result: ToolResult{Content: []ToolContent{{Text: "missing type"}}},
			reason: ReasonSchemaInvalid,
		},
		{
			name:   "Should reject invalid structured JSON",
			result: ToolResult{Structured: json.RawMessage(`{bad`)},
			reason: ReasonSchemaInvalid,
		},
		{
			name: "Should reject invalid content data JSON",
			result: ToolResult{
				Content: []ToolContent{{Type: "json", Data: json.RawMessage(`{bad`)}},
			},
			reason: ReasonSchemaInvalid,
		},
		{
			name:   "Should reject invalid result metadata JSON",
			result: ToolResult{Metadata: map[string]json.RawMessage{"safe": json.RawMessage(`{bad`)}},
			reason: ReasonSchemaInvalid,
		},
		{
			name: "Should reject invalid content metadata JSON",
			result: ToolResult{
				Content: []ToolContent{{
					Type:     "text",
					Metadata: map[string]json.RawMessage{"safe": json.RawMessage(`{bad`)},
				}},
			},
			reason: ReasonSchemaInvalid,
		},
		{
			name: "Should reject secret content metadata",
			result: ToolResult{
				Content: []ToolContent{
					{Type: "text", Metadata: map[string]json.RawMessage{"refresh-token": json.RawMessage(`"secret"`)}},
				},
			},
			reason: ReasonSecretMetadata,
		},
		{
			name:   "Should reject negative artifact bytes",
			result: ToolResult{Artifacts: []ArtifactRef{{URI: "file://artifact", Bytes: -1}}},
			reason: ReasonResultBudgetExceeded,
		},
		{
			name:   "Should reject redactions without paths",
			result: ToolResult{Redactions: []Redaction{{Reason: ReasonSecretMetadata}}},
			reason: ReasonSecretMetadata,
		},
		{
			name:   "Should reject unsupported redaction reasons",
			result: ToolResult{Redactions: []Redaction{{Path: "$.token", Reason: "future_reason"}}},
			reason: ReasonPolicyDenied,
		},
	}
	for _, tt := range resultTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireReason(t, tt.result.Validate(64), tt.reason)
		})
	}
}

func TestProviderAndHandleValidation(t *testing.T) {
	t.Parallel()

	validSource := SourceRef{Kind: SourceBuiltin, Owner: "daemon"}

	t.Run("Should accept complete providers and handles", func(t *testing.T) {
		t.Parallel()

		if err := ValidateProvider(testProvider{source: validSource}); err != nil {
			t.Fatalf("ValidateProvider() error = %v", err)
		}
		if err := ValidateHandle(testHandle{descriptor: validDescriptor()}); err != nil {
			t.Fatalf("ValidateHandle() error = %v", err)
		}
	})

	t.Run("Should reject nil providers", func(t *testing.T) {
		t.Parallel()

		requireReason(t, ValidateProvider(nil), ReasonDependencyMissing)
	})

	t.Run("Should reject typed nil providers", func(t *testing.T) {
		t.Parallel()

		var provider *testProvider
		requireReason(t, ValidateProvider(provider), ReasonDependencyMissing)
	})

	t.Run("Should reject incomplete providers", func(t *testing.T) {
		t.Parallel()

		requireReason(t, ValidateProvider(testProvider{}), ReasonSourceDisabled)
	})

	t.Run("Should reject nil handles", func(t *testing.T) {
		t.Parallel()

		requireReason(t, ValidateHandle(nil), ReasonBackendNotExecutable)
	})

	t.Run("Should reject typed nil handles", func(t *testing.T) {
		t.Parallel()

		var handle *testHandle
		requireReason(t, ValidateHandle(handle), ReasonBackendNotExecutable)
	})

	t.Run("Should reject incomplete handles", func(t *testing.T) {
		t.Parallel()

		requireReason(t, ValidateHandle(testHandle{}), ReasonIDEmpty)
	})
}

func TestToolErrorReasonExtraction(t *testing.T) {
	t.Parallel()

	t.Run("Should extract reasons from tool errors", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("permission source")
		err := NewToolError(
			ErrorCodeDenied,
			"agh__network_send",
			"denied",
			cause,
			ReasonPolicyDenied,
			ReasonSessionDenied,
		)
		if !errors.Is(err, cause) {
			t.Fatalf("errors.Is(ToolError, cause) = false, want true")
		}
		if got, want := err.Error(), "denied"; got != want {
			t.Fatalf("ToolError.Error() = %q, want %q", got, want)
		}
		requireReason(t, err, ReasonPolicyDenied)
	})

	t.Run("Should fall back to wrapped cause and code messages", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("backend down")
		withCause := NewToolError(ErrorCodeBackendFailed, "agh__skill_view", "", cause)
		if got, want := withCause.Error(), "backend down"; got != want {
			t.Fatalf("ToolError.Error() = %q, want %q", got, want)
		}
		codeOnly := NewToolError(ErrorCodeNotFound, "agh__skill_view", "", nil)
		if got, want := codeOnly.Error(), string(ErrorCodeNotFound); got != want {
			t.Fatalf("ToolError.Error() = %q, want %q", got, want)
		}
	})

	t.Run("Should format nil and validation errors deterministically", func(t *testing.T) {
		t.Parallel()

		var nilToolErr *ToolError
		if got, want := nilToolErr.Error(), nilErrorText; got != want {
			t.Fatalf("nil ToolError.Error() = %q, want %q", got, want)
		}
		if unwrapped := nilToolErr.Unwrap(); unwrapped != nil {
			t.Fatalf("nil ToolError.Unwrap() = %v, want nil", unwrapped)
		}
		var nilValidation *ValidationError
		if got, want := nilValidation.Error(), nilErrorText; got != want {
			t.Fatalf("nil ValidationError.Error() = %q, want %q", got, want)
		}
		err := NewValidationError("tool_id", ReasonIDInvalidFormat, "bad format")
		if got, want := err.Error(), "tools: validation failed: tool_id: id_invalid_format: bad format"; got != want {
			t.Fatalf("ValidationError.Error() = %q, want %q", got, want)
		}
	})

	t.Run("Should report no reason for generic errors", func(t *testing.T) {
		t.Parallel()

		if reason, ok := ReasonOf(errors.New("plain")); ok || reason != "" {
			t.Fatalf("ReasonOf(generic) = %q, %v; want empty false", reason, ok)
		}
	})
}

func TestSourceKindJSON(t *testing.T) {
	t.Parallel()

	t.Run("Should encode and decode source aliases as strings", func(t *testing.T) {
		t.Parallel()

		data, err := json.Marshal(ToolSourceExtension)
		if err != nil {
			t.Fatalf("json.Marshal(ToolSourceExtension) error = %v", err)
		}
		if got, want := string(data), `"extension"`; got != want {
			t.Fatalf("json.Marshal(ToolSourceExtension) = %s, want %s", got, want)
		}

		var decoded ToolSource
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("json.Unmarshal(ToolSource) error = %v", err)
		}
		if decoded != ToolSourceExtension {
			t.Fatalf("decoded ToolSource = %s, want %s", decoded, ToolSourceExtension)
		}
	})
}

func fmtErrorFromCanonicalSegment(raw string) error {
	_, err := CanonicalIDSegment(raw)
	return err
}
