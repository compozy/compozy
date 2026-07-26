package tools

import (
	"encoding/json"
	"testing"

	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/testutil"
)

func mustToolResourceCodec(t *testing.T) resources.KindCodec[Tool] {
	t.Helper()

	codec, err := NewResourceCodec()
	if err != nil {
		t.Fatalf("NewResourceCodec() error = %v", err)
	}
	return codec
}

func TestToolResourceCodecCanonicalizesInputSchema(t *testing.T) {
	t.Parallel()

	t.Run("Should canonicalize and trim final tool resource specs", func(t *testing.T) {
		t.Parallel()

		codec := mustToolResourceCodec(t)

		scope := resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
		spec, err := codec.DecodeAndValidate(testutil.Context(t), scope, []byte(`{
			"id": "ext__linear__search",
			"display_title": " Search ",
			"friendly_verb": " Searching ",
			"preview": " arg:query ",
			"description": " search files ",
			"backend": {
				"kind": "extension_host",
				"extension_id": " linear ",
				"handler": " search ",
				"requires_capabilities": [" files.read "]
			},
			"input_schema": {
				"properties": {"path": {"type": "string"}},
				"type": "object"
			},
			"output_schema": null,
			"source": {
				"kind": "extension",
				"owner": " linear ",
				"raw_tool_name": " search "
			},
			"visibility": "operator",
			"risk": "read",
			"read_only": true,
			"concurrency_safe": true,
			"toolsets": [" linear__read "],
			"tags": [" search "],
			"search_hints": [" files "]
		}`))
		if err != nil {
			t.Fatalf("codec.DecodeAndValidate() error = %v", err)
		}

		if got, want := spec.ID, ToolID("ext__linear__search"); got != want {
			t.Fatalf("spec.ID = %q, want %q", got, want)
		}
		if got, want := spec.DisplayTitle, "Search"; got != want {
			t.Fatalf("spec.DisplayTitle = %q, want %q", got, want)
		}
		presentation := spec.Presentation()
		if got, want := presentation.FriendlyVerb, "Searching"; got != want {
			t.Fatalf("spec presentation friendly verb = %q, want %q", got, want)
		}
		if got, want := presentation.Preview, "arg:query"; got != want {
			t.Fatalf("spec presentation preview = %q, want %q", got, want)
		}
		if got, want := spec.Description, "search files"; got != want {
			t.Fatalf("spec.Description = %q, want %q", got, want)
		}
		if got, want := string(
			spec.InputSchema,
		), `{"properties":{"path":{"type":"string"}},"type":"object"}`; got != want {
			t.Fatalf("spec.InputSchema = %s, want %s", got, want)
		}
		if spec.OutputSchema != nil {
			t.Fatalf("spec.OutputSchema = %s, want nil", spec.OutputSchema)
		}
		if got, want := spec.Backend.ExtensionID, "linear"; got != want {
			t.Fatalf("spec.Backend.ExtensionID = %q, want %q", got, want)
		}
		if got, want := spec.Backend.RequiresCapabilities[0], "files.read"; got != want {
			t.Fatalf("spec.Backend.RequiresCapabilities[0] = %q, want %q", got, want)
		}
		if got, want := spec.Source.Owner, "linear"; got != want {
			t.Fatalf("spec.Source.Owner = %q, want %q", got, want)
		}
		if got, want := spec.Source.RawToolName, "search"; got != want {
			t.Fatalf("spec.Source.RawToolName = %q, want %q", got, want)
		}
		if got, want := spec.Toolsets[0], ToolsetID("linear__read"); got != want {
			t.Fatalf("spec.Toolsets[0] = %q, want %q", got, want)
		}

		encoded, err := codec.Encode(spec)
		if err != nil {
			t.Fatalf("codec.Encode() error = %v", err)
		}
		var flat map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &flat); err != nil {
			t.Fatalf("json.Unmarshal(encoded tool) error = %v", err)
		}
		if _, ok := flat["friendly_verb"]; !ok {
			t.Fatalf("encoded tool = %s, want flat friendly_verb", encoded)
		}
		if _, ok := flat["preview"]; !ok {
			t.Fatalf("encoded tool = %s, want flat preview", encoded)
		}
		if _, nested := flat["tool_presentation"]; nested {
			t.Fatalf("encoded tool = %s, want no nested tool_presentation", encoded)
		}
	})
}

func TestToolResourceCodecRejectsInvalidSchema(t *testing.T) {
	t.Parallel()

	t.Run("Should reject invalid input schemas", func(t *testing.T) {
		t.Parallel()

		codec := mustToolResourceCodec(t)
		_, err := codec.DecodeAndValidate(
			testutil.Context(t),
			resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			[]byte(`{
				"id": "ext__linear__search",
				"backend": {"kind": "extension_host", "extension_id": "linear", "handler": "search"},
				"input_schema": "{not-json",
				"source": {"kind": "extension", "owner": "linear"},
				"visibility": "operator",
				"risk": "read",
				"read_only": true
			}`),
		)
		if err == nil {
			t.Fatal("codec.DecodeAndValidate() error = nil, want invalid input_schema failure")
		}
	})

	t.Run("Should reject missing input schemas", func(t *testing.T) {
		t.Parallel()

		codec := mustToolResourceCodec(t)
		_, err := codec.DecodeAndValidate(
			testutil.Context(t),
			resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			[]byte(`{
				"id": "ext__linear__search",
				"backend": {"kind": "extension_host", "extension_id": "linear", "handler": "search"},
				"source": {"kind": "extension", "owner": "linear"},
				"visibility": "operator",
				"risk": "read",
				"read_only": true
			}`),
		)
		if err == nil {
			t.Fatal("codec.DecodeAndValidate() error = nil, want missing input_schema failure")
		}
	})

	t.Run("Should reject invalid output schemas", func(t *testing.T) {
		t.Parallel()

		codec := mustToolResourceCodec(t)
		_, err := codec.DecodeAndValidate(
			testutil.Context(t),
			resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			[]byte(`{
				"id": "ext__linear__search",
				"backend": {"kind": "extension_host", "extension_id": "linear", "handler": "search"},
				"input_schema": {"type": "object"},
				"output_schema": "invalid",
				"source": {"kind": "extension", "owner": "linear"},
				"visibility": "operator",
				"risk": "read",
				"read_only": true
			}`),
		)
		if err == nil {
			t.Fatal("codec.DecodeAndValidate() error = nil, want invalid output_schema failure")
		}
	})
}
