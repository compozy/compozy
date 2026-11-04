package spec

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/tools"
)

// alwaysOnHardeningAdditions lists the response fields the always-on hardening spec adds
// (`_dx.md`). Each is additive: existing fields, types and required members stay unchanged.
var alwaysOnHardeningAdditions = []struct {
	pathPrefix string
	keys       []string
}{
	// Check-in sources add only these reviewed fields to the named wire components.
	{"#/components/schemas/HeartbeatContextProjectionPayload", []string{"sources"}},
	{"#/components/schemas/AgentInstancePayload", []string{"sources"}},
	{"#/components/schemas/AgentInstanceCheckResponse", []string{"sources"}},
	{"#/components/schemas/AgentInstanceCheckDigest", []string{"source"}},
	{"#/components/schemas/AgentInstanceProposalSource", []string{"source_id", "source_ref", "item_key", "url"}},
	{"#/components/schemas/AgentInstanceReportSource", []string{"source_id", "source_ref", "item_key", "url"}},
	// Every session payload: the mode a session runs at, and whether the reviewer fell back.
	{"#/", []string{"effective_permissions", "permission_mode_degraded"}},
	// Queued input rows: input held while an always-on lineage is paused.
	{"#/", []string{"held"}},
	// Agent definitions: operator-only approval reach and exceptions.
	{"#/", []string{"session_approval", "permission_rules"}},
	// Steer and queue results name the instance main a message reached.
	{"#/", []string{"target_session_id"}},
	// Triggers record the scope of the always-on agent they target.
	{"#/", []string{"target_instance_scope"}},
	// Agent instances: the spend brake counter, its limit, held wakes and the next wake.
	{"#/", []string{"unattended_turns", "unattended_turn_limit", "held_wakes", "next_wake"}},
	// Provider summaries: the reviewer mode the `auto` level selects.
	{"#/", []string{"reviewer_mode"}},
	// Session catalog stream notifications carry the permission request they announce.
	{"#/paths/~1api~1sessions~1catalog-stream", []string{"action", "kind", "resource"}},
}

// addAlwaysOnHardeningSchemas copies exactly the approved fields from the current document into
// the historical baseline, including their required membership, and fails on any other key.
func addAlwaysOnHardeningSchemas(t *testing.T, baseline map[string]any) {
	t.Helper()
	doc, err := Document()
	if err != nil {
		t.Fatalf("Document: %v", err)
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var current map[string]any
	if err := json.Unmarshal(data, &current); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// A message to an agent with no instance, or one still starting, now has its own code.
	for _, code := range []tools.ErrorCode{
		tools.ErrorCodeAgentInstanceNotFound,
		tools.ErrorCodeAgentInstanceStarting,
	} {
		if addToolErrorCode(baseline, string(code)) == 0 {
			t.Fatal("approved agent instance tool errors: no historical tool error enums found")
		}
	}
	relaxTriggerCreatePrompt(t, baseline)
	components := current["components"].(map[string]any)["schemas"].(map[string]any)
	walker := approvedAdditionWalker{
		oldComponents: baseline["components"].(map[string]any)["schemas"].(map[string]any),
		components:    components,
		seen:          map[string]bool{},
	}
	walker.copy(baseline["paths"], current["paths"], "#/paths")
	walker.copy(baseline["components"].(map[string]any)["schemas"], components, "#/components/schemas")
}

func approvedAdditionKeys(pointer string) map[string]bool {
	keys := map[string]bool{}
	for _, addition := range alwaysOnHardeningAdditions {
		if strings.HasPrefix(pointer, addition.pathPrefix) {
			for _, key := range addition.keys {
				keys[key] = true
			}
		}
	}
	return keys
}

// approvedAdditionWalker walks the inlined baseline beside the current document, resolving the
// current side's component references so both sides compare the same schema.
type approvedAdditionWalker struct {
	oldComponents map[string]any
	components    map[string]any
	seen          map[string]bool
}

// alwaysOnHardeningEnumValues are the enum members the spec adds; no existing member is removed.
var alwaysOnHardeningEnumValues = map[string]bool{
	"agent_instance":        true, // trigger target_kind
	"approve-edits":         true, // permission level
	"auto":                  true, // permission level
	"goal_on_instance_main": true, // Goal refusal on an always-on main
	"unattended":            true, // daemon-set pause reason
	"source_poll_in_flight": true, // check-in source collection is pending
}

// copyApprovedEnum extends a baseline enum with approved members the current document added.
func copyApprovedEnum(expected, actual map[string]any) {
	oldEnum, okOld := expected["enum"].([]any)
	newEnum, okNew := actual["enum"].([]any)
	if !okOld || !okNew || len(newEnum) <= len(oldEnum) {
		return
	}
	for _, value := range oldEnum {
		if !slices.Contains(newEnum, value) {
			return
		}
	}
	for _, value := range newEnum {
		text, _ := value.(string)
		if !slices.Contains(oldEnum, value) && !alwaysOnHardeningEnumValues[text] {
			return // an unapproved member stays a diff the comparison reports
		}
	}
	expected["enum"] = slices.Clone(newEnum)
}

// underlyingSchema follows a component reference or a nullable oneOf to the schema object the
// comparison normalizes, returning the original map so edits persist.
func underlyingSchema(value any, components map[string]any) (any, string) {
	object, ok := value.(map[string]any)
	if !ok {
		return value, ""
	}
	reference, _ := object["$ref"].(string)
	if reference != "" {
		resolved, found := components[strings.TrimPrefix(reference, "#/components/schemas/")].(map[string]any)
		if !found {
			return nil, reference
		}
		object = resolved
	}
	if alternatives, ok := object["oneOf"].([]any); ok && len(alternatives) == 2 {
		if null, ok := alternatives[1].(map[string]any); ok && reflect.DeepEqual(null["enum"], []any{nil}) {
			target, ref := underlyingSchema(alternatives[0], components)
			if reference == "" {
				reference = ref
			}
			return target, reference
		}
	}
	return object, reference
}

// carryComponents copies the components an approved field references into the baseline, so the
// comparison can resolve them; a component the baseline already has is left unchanged.
func (w approvedAdditionWalker) carryComponents(schema any) {
	switch node := schema.(type) {
	case map[string]any:
		if ref, ok := node["$ref"].(string); ok {
			name := strings.TrimPrefix(ref, "#/components/schemas/")
			if _, exists := w.oldComponents[name]; !exists {
				w.oldComponents[name] = w.components[name]
				w.carryComponents(w.components[name])
			}
		}
		for _, child := range node {
			w.carryComponents(child)
		}
	case []any:
		for _, child := range node {
			w.carryComponents(child)
		}
	}
}

func (w approvedAdditionWalker) copy(oldValue, nextValue any, pointer string) {
	old, oldRef := underlyingSchema(oldValue, w.oldComponents)
	next, newRef := underlyingSchema(nextValue, w.components)
	if old == nil || next == nil {
		return
	}
	if oldRef != "" || newRef != "" {
		// Inlined baselines repeat a schema per path, so dedupe by object identity, not by name.
		key := fmt.Sprintf("%p|%p", old, next)
		if w.seen[key] {
			return
		}
		w.seen[key] = true
	}
	switch expected := old.(type) {
	case map[string]any:
		actual, ok := next.(map[string]any)
		if !ok {
			return
		}
		copyApprovedEnum(expected, actual)
		approved := approvedAdditionKeys(pointer)
		// Inlined historical responses still use the current component's exact allowlist.
		for key := range approvedAdditionKeys(newRef) {
			approved[key] = true
		}
		oldProperties, hasOld := expected["properties"].(map[string]any)
		newProperties, hasNew := actual["properties"].(map[string]any)
		if hasOld && hasNew {
			added := []string{}
			for key, schema := range newProperties {
				if _, exists := oldProperties[key]; !exists && approved[key] {
					oldProperties[key] = schema
					w.carryComponents(schema)
					added = append(added, key)
				}
			}
			copyApprovedRequired(expected, actual, added)
		}
		for key, value := range expected {
			w.copy(value, actual[key], pointer+"/"+escapePointerToken(key))
		}
	case []any:
		actual, ok := next.([]any)
		if !ok || len(actual) != len(expected) {
			return
		}
		for index := range expected {
			w.copy(expected[index], actual[index], pointer)
		}
	}
}

func copyApprovedRequired(expected, actual map[string]any, added []string) {
	newRequired, ok := actual["required"].([]any)
	if !ok || len(added) == 0 {
		return
	}
	oldRequired, _ := expected["required"].([]any)
	for _, key := range added {
		if slices.Contains(newRequired, any(key)) && !slices.Contains(oldRequired, any(key)) {
			oldRequired = append(oldRequired, key)
		}
	}
	slices.SortFunc(oldRequired, func(left, right any) int {
		return strings.Compare(left.(string), right.(string))
	})
	expected["required"] = oldRequired
}

func escapePointerToken(token string) string {
	return strings.ReplaceAll(strings.ReplaceAll(token, "~", "~0"), "/", "~1")
}

// relaxTriggerCreatePrompt records the one approved loosening: an agent_instance trigger may omit
// its prompt, so the create request no longer lists it as required. Agent targets still need one,
// which the automation model enforces.
func relaxTriggerCreatePrompt(t *testing.T, baseline map[string]any) {
	t.Helper()
	node := baseline
	for _, key := range []string{
		"paths", "/api/automation/triggers", "post", "requestBody", "content", "application/json", "schema",
	} {
		next, ok := node[key].(map[string]any)
		if !ok {
			t.Fatalf("approved trigger prompt relaxation: missing object %q", key)
		}
		node = next
	}
	required, ok := node["required"].([]any)
	if !ok || !slices.Contains(required, any("prompt")) {
		t.Fatal("approved trigger prompt relaxation: historical create request does not require prompt")
	}
	node["required"] = slices.DeleteFunc(slices.Clone(required), func(value any) bool { return value == "prompt" })
}

// TestAgentKindRequestSchemas owns the UT-023 schema rows: request kinds accept the deprecated
// stays_on alias through v0.5.1, while response kinds stay a plain string.
func TestAgentKindRequestSchemas(t *testing.T) {
	t.Parallel()

	doc, err := Document()
	if err != nil {
		t.Fatalf("Document: %v", err)
	}
	schemas := doc.Components.Schemas
	for _, name := range []string{"CreateAgentPayload", "DuplicateAgentRequest", "DuplicateAgentOverrides"} {
		t.Run("Should list the request kinds on "+name, func(t *testing.T) {
			t.Parallel()

			ref := schemas[name]
			if ref == nil || ref.Value == nil || ref.Value.Properties["kind"] == nil {
				t.Fatalf("%s.kind is missing", name)
			}
			kind := ref.Value.Properties["kind"].Value
			if !reflect.DeepEqual(kind.Enum, []any{"on_demand", "always_on", "stays_on"}) ||
				!strings.Contains(kind.Description, "stays_on is the deprecated") ||
				!strings.Contains(kind.Description, "v0.5.2") {
				t.Fatalf(
					"%s.kind = %v %q, want the three kinds and the deprecation text",
					name,
					kind.Enum,
					kind.Description,
				)
			}
		})
	}
	for _, name := range []string{"AgentPayload", "GroupedAgentCatalogItemPayload"} {
		t.Run("Should keep a plain string kind on "+name, func(t *testing.T) {
			t.Parallel()

			ref := schemas[name]
			if ref == nil || ref.Value == nil || ref.Value.Properties["kind"] == nil {
				t.Fatalf("%s.kind is missing", name)
			}
			kind := ref.Value.Properties["kind"].Value
			if len(kind.Enum) != 0 || !kind.Type.Is("string") {
				t.Fatalf("%s.kind = %v %v, want a plain string", name, kind.Type, kind.Enum)
			}
		})
	}
}
