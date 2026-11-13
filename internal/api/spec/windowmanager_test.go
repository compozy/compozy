// Suite: Window-manager OpenAPI and transport-neutral operation registry.
// Invariant: WM-TRANSPORT-002 documents the exact HTTP/UDS command schemas,
// statuses, frame discriminators, and JavaScript-safe revision bounds.
// Owning layer: internal/api/spec. Canonical suite: this file.
// Boundary IN: public paths, operation IDs, DTO shapes, and error responses.
// Boundary OUT: runtime handler behavior belongs to internal/api/core.
package spec

import (
	"maps"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/getkin/kin-openapi/openapi3"
)

func TestWindowManagerOpenAPIContract(t *testing.T) {
	t.Run("Should describe optional compatibility inputs and migration actions precisely", func(t *testing.T) {
		t.Parallel()
		document, err := Document()
		if err != nil {
			t.Fatal(err)
		}
		for component, fields := range map[string][]string{
			"WindowManagerWindowSpecPayload":     {"floating_rect", "insert_tiled"},
			"WindowManagerCloseWindowPayload":    {"minimize"},
			"WindowManagerResizeWindowPayload":   {"frame"},
			"WindowManagerArrangeLayoutPayload":  {"frame", "group_id"},
			"WindowManagerGroupFrameEditPayload": {"frame", "group_id"},
			"WindowManagerMigrationRequest":      {"replace"},
		} {
			for _, field := range fields {
				schema := componentSchema(t, document, component)
				if propertySchema(t, schema, field) == nil || slices.Contains(schema.Required, field) {
					t.Fatalf("%s.%s must be accepted but optional", component, field)
				}
			}
		}
		request := componentSchema(t, document, "WindowManagerMigrationRequest")
		assertEnumValues(t, propertySchema(t, request, "action"), "ack", "retry")
		status := componentSchema(t, document, "MigrationStatus")
		assertEnumValues(t, propertySchema(t, status, "state"), "none", "migrated", "failed")
		assertResponseStatus(t, operationFor(t, document, windowManagerPath+"/migration", http.MethodPost), 422)
	})
	t.Run("Should mark retired command and projection fields without changing live fields", func(t *testing.T) {
		t.Parallel()
		document, err := Document()
		if err != nil {
			t.Fatal(err)
		}
		for component, fields := range map[string][]string{
			"WindowManagerWindowSpecPayload":     {"floating", "floating_rect"},
			"WindowManagerOpenWindowPayload":     {"restore_window_id"},
			"WindowManagerCloseWindowPayload":    {"minimize"},
			"WindowManagerGroupWindowsPayload":   {"insert_index"},
			"WindowManagerMoveWindowPayload":     {"floating_rect"},
			"WindowManagerResizeWindowPayload":   {"frame"},
			"WindowManagerToggleFloatingPayload": {"floating_rect"},
			"WindowManagerArrangeLayoutPayload":  {"frame", "group_id", "keep_frames"},
			"WindowManagerGroupFrameEditPayload": {"group_id", "frame"},
			"WindowManagerBalanceLayoutPayload":  {"group_id"},
			"ChangeSet":                          {"group_ids", "stack_grouped", "stack_ungrouped"},
			"WindowManagerClientView":            {"stack_active"},
			"WindowManagerPaletteContext":        {"window_floating", "window_stacked"},
		} {
			for _, field := range fields {
				property := propertySchema(t, componentSchema(t, document, component), field)
				if !property.Deprecated || !strings.Contains(property.Description, "v0.8.0") {
					t.Fatalf("%s.%s lacks its removal metadata: %#v", component, field, property)
				}
			}
		}
		for component, fields := range map[string][]string{
			"WindowManagerWindowSpecPayload":    {"insert_tiled", "stack_target_window_id"},
			"WindowManagerMoveWindowPayload":    {"placement", "move_group", "target_tab_id"},
			"WindowManagerArrangeLayoutPayload": {"arrangement"},
		} {
			for _, field := range fields {
				if propertySchema(t, componentSchema(t, document, component), field).Deprecated {
					t.Fatalf("live field %s.%s deprecated", component, field)
				}
			}
		}
	})

	t.Run("Should expose only the exact window-manager routes on HTTP and UDS", func(t *testing.T) {
		t.Parallel()
		expected := map[string]string{
			http.MethodGet + " " + windowManagerPath:                                      "getWindowManagerSnapshot",
			http.MethodPost + " " + windowManagerPath + "/preview":                        "previewWindowManagerCommand",
			http.MethodPost + " " + windowManagerPath + "/commands":                       "executeWindowManagerCommand",
			http.MethodGet + " " + windowManagerPath + "/migration":                       "getWindowManagerMigration",
			http.MethodPost + " " + windowManagerPath + "/migration":                      "updateWindowManagerMigration",
			http.MethodGet + " " + windowManagerPath + "/clients":                         "listWindowManagerClients",
			http.MethodPost + " " + windowManagerPath + "/clients":                        "registerWindowManagerClient",
			http.MethodDelete + " " + windowManagerPath + "/clients/{client_id}":          "unregisterWindowManagerClient",
			http.MethodGet + " " + windowManagerPath + "/layout":                          "exportWindowManagerLayout",
			http.MethodPost + " " + windowManagerPath + "/layout/validate":                "validateWindowManagerLayout",
			http.MethodPut + " " + windowManagerPath + "/layout":                          "replaceWindowManagerLayout",
			http.MethodGet + " " + windowManagerPath + "/layout-profiles":                 "listWindowManagerLayoutProfiles",
			http.MethodPut + " " + windowManagerPath + "/layout-profiles/{profile_id}":    "putWindowManagerLayoutProfile",
			http.MethodDelete + " " + windowManagerPath + "/layout-profiles/{profile_id}": "deleteWindowManagerLayoutProfile",
			http.MethodGet + " " + windowManagerPath + "/stream":                          "streamWindowManager",
		}
		seen := make(map[string]struct{}, len(expected))
		for _, operation := range Operations() {
			if !strings.HasPrefix(operation.Path, windowManagerPath) {
				continue
			}
			key := operation.Method + " " + operation.Path
			operationID, exists := expected[key]
			if !exists {
				t.Fatalf("unexpected window-manager operation %s", key)
			}
			if operation.OperationID != operationID {
				t.Fatalf("%s operation ID = %q, want %q", key, operation.OperationID, operationID)
			}
			if !slices.Equal(operation.Transports, []Transport{TransportHTTP, TransportUDS}) {
				t.Fatalf("%s transports = %v", key, operation.Transports)
			}
			seen[key] = struct{}{}
		}
		if len(seen) != len(expected) {
			t.Fatalf("window-manager operations found = %d, want %d; seen = %v", len(seen), len(expected), seen)
		}
	})

	t.Run("Should describe closed discriminated command schemas and stable errors", func(t *testing.T) {
		t.Parallel()
		document, err := Document()
		if err != nil {
			t.Fatalf("Document() error = %v", err)
		}
		request := componentSchema(t, document, "WindowManagerCommandRequest")
		if request.Discriminator == nil || request.Discriminator.PropertyName != "command_id" {
			t.Fatalf("request discriminator = %#v, want command_id", request.Discriminator)
		}
		expectedVariants := []struct {
			commandID      contract.WindowManagerCommandID
			payloadSchema  string
			requiresClient bool
		}{
			{contract.WindowManagerCommandDesktopCreate, "WindowManagerCreateDesktopPayload", false},
			{contract.WindowManagerCommandDesktopUpdate, "WindowManagerUpdateDesktopPayload", false},
			{contract.WindowManagerCommandDesktopReorder, "WindowManagerReorderDesktopPayload", false},
			{contract.WindowManagerCommandDesktopSwitch, "WindowManagerSwitchDesktopPayload", true},
			{contract.WindowManagerCommandDesktopDelete, "WindowManagerDeleteDesktopPayload", false},
			{contract.WindowManagerCommandWindowOpen, "WindowManagerOpenWindowPayload", false},
			{contract.WindowManagerCommandWindowNavigate, "WindowManagerNavigateWindowPayload", false},
			{contract.WindowManagerCommandWindowClose, "WindowManagerCloseWindowPayload", false},
			{contract.WindowManagerCommandWindowStackGroup, "WindowManagerGroupWindowsPayload", false},
			{contract.WindowManagerCommandWindowStackReorder, "WindowManagerReorderStackPayload", false},
			{
				contract.WindowManagerCommandWindowStackSetActive,
				"WindowManagerSetStackActivePayload",
				false,
			},
			{contract.WindowManagerCommandWindowPin, "WindowManagerPinWindowPayload", false},
			{contract.WindowManagerCommandWindowTabActivate, "WindowManagerSetStackActivePayload", false},
			{contract.WindowManagerCommandWindowTabReorder, "WindowManagerReorderStackPayload", false},
			{contract.WindowManagerCommandWindowResize, "WindowManagerResizeWindowPayload", false},
			{contract.WindowManagerCommandLayoutFrameResize, "WindowManagerFrameResizeLayoutPayload", false},
			{contract.WindowManagerCommandWindowReopen, "WindowManagerReopenWindowPayload", false},
			{contract.WindowManagerCommandWindowFocus, "WindowManagerFocusWindowPayload", true},
			{contract.WindowManagerCommandWindowMove, "WindowManagerMoveWindowPayload", false},
			{contract.WindowManagerCommandWindowSwap, "WindowManagerSwapWindowsPayload", false},
			{
				contract.WindowManagerCommandWindowToggleFloating,
				"WindowManagerToggleFloatingPayload",
				false,
			},
			{contract.WindowManagerCommandWindowZoom, "WindowManagerZoomWindowPayload", false},
			{contract.WindowManagerCommandLayoutArrange, "WindowManagerArrangeLayoutPayload", false},
			{contract.WindowManagerCommandLayoutResize, "WindowManagerResizeLayoutPayload", false},
			{contract.WindowManagerCommandLayoutBalance, "WindowManagerBalanceLayoutPayload", false},
			{contract.WindowManagerCommandLayoutUndo, "WindowManagerUndoLayoutPayload", false},
			{contract.WindowManagerCommandLayoutRedo, "WindowManagerRedoLayoutPayload", false},
			{contract.WindowManagerCommandLayoutReplace, "WindowManagerReplaceLayoutPayload", false},
		}
		if len(request.OneOf) != len(expectedVariants) {
			t.Fatalf(
				"request oneOf variants = %d, want %d",
				len(request.OneOf),
				len(expectedVariants),
			)
		}
		variantsByCommand := make(map[contract.WindowManagerCommandID]*openapi3.Schema, len(request.OneOf))
		for _, variantRef := range request.OneOf {
			if variantRef == nil || variantRef.Value == nil {
				t.Fatalf("unresolved command request schema = %#v", variantRef)
			}
			variant := variantRef.Value
			assertSchemaHasAdditionalProperties(t, variant, false)
			assertRequired(
				t,
				variant,
				"workspace_id",
				"command_id",
				"expected_revision",
				"actor",
				"origin",
				"payload",
			)
			commandIDSchema := propertySchema(t, variant, "command_id")
			if len(commandIDSchema.Enum) != 1 {
				t.Fatalf("command request variant enum = %#v, want one value", commandIDSchema.Enum)
			}
			commandID, ok := commandIDSchema.Enum[0].(string)
			if !ok {
				t.Fatalf("command request variant enum = %#v, want string", commandIDSchema.Enum[0])
			}
			typedCommandID := contract.WindowManagerCommandID(commandID)
			if _, duplicate := variantsByCommand[typedCommandID]; duplicate {
				t.Fatalf("duplicate command request variant %q", commandID)
			}
			variantsByCommand[typedCommandID] = variant
			revision := propertySchema(t, variant, "expected_revision")
			if revision.Max == nil || *revision.Max != float64(contract.WindowManagerMaxSafeRevision) {
				t.Fatalf("%s expected_revision maximum = %v", commandID, revision.Max)
			}
		}
		for _, expected := range expectedVariants {
			variant := variantsByCommand[expected.commandID]
			if variant == nil {
				t.Fatalf("command request variant %q is missing", expected.commandID)
			}
			payload := variant.Properties["payload"]
			wantPayloadRef := "#/components/schemas/" + expected.payloadSchema
			if payload == nil || payload.Ref != wantPayloadRef || payload.Value == nil {
				t.Fatalf("%s payload = %#v, want %s", expected.commandID, payload, wantPayloadRef)
			}
			assertSchemaHasAdditionalProperties(t, payload.Value, false)
			if expected.requiresClient {
				assertRequired(t, variant, "client_id")
				client := propertySchema(t, variant, "client_id")
				if client.Nullable {
					t.Fatalf("%s client_id is nullable", expected.commandID)
				}
			} else {
				assertNotRequired(t, variant, "client_id")
			}
		}

		navigate := componentSchema(t, document, "WindowManagerNavigateWindowPayload")
		assertRequired(t, navigate, "window_id")
		assertNotRequired(t, navigate, "route")
		assertNotRequired(t, navigate, "mode")
		route := propertySchema(t, navigate, "route")
		assertRequired(t, route, "pathname", "search")
		assertSchemaHasAdditionalProperties(t, route, false)
		assertSchemaIncludesType(t, propertySchema(t, route, "pathname"), openapi3.TypeString)
		search := propertySchema(t, route, "search")
		assertSchemaIncludesType(t, search, openapi3.TypeObject)
		assertSchemaHasAdditionalProperties(t, search, true)
		assertEnumValues(
			t,
			propertySchema(t, navigate, "mode"),
			"replace",
			string(windowmanager.NavigatePush),
			string(windowmanager.NavigatePop),
		)
		if len(navigate.OneOf) != 2 {
			t.Fatalf("navigate oneOf branches = %d, want 2", len(navigate.OneOf))
		}
		var popBranch, routedBranch *openapi3.Schema
		for _, branchRef := range navigate.OneOf {
			if branchRef == nil || branchRef.Value == nil {
				t.Fatalf("navigate oneOf branch = %#v", branchRef)
			}
			branch := branchRef.Value
			mode := propertySchema(t, branch, "mode")
			if slices.ContainsFunc(mode.Enum, func(value any) bool {
				return value == string(windowmanager.NavigatePop)
			}) {
				popBranch = branch
			} else {
				routedBranch = branch
			}
		}
		if popBranch == nil || routedBranch == nil {
			t.Fatalf("navigate oneOf branches = %#v", navigate.OneOf)
		}
		assertRequired(t, popBranch, "mode")
		assertNotRequired(t, popBranch, "route")
		assertEnumValues(
			t,
			propertySchema(t, popBranch, "mode"),
			string(windowmanager.NavigatePop),
		)
		assertRequired(t, routedBranch, "route")
		assertNotRequired(t, routedBranch, "mode")
		assertEnumValues(
			t,
			propertySchema(t, routedBranch, "mode"),
			"replace",
			string(windowmanager.NavigatePush),
		)
		if err := navigate.VisitJSON(map[string]any{
			"window_id": "window-a",
			"mode":      string(windowmanager.NavigatePop),
			"route": map[string]any{
				"pathname": "/tasks",
				"search":   map[string]any{},
			},
		}); err == nil {
			t.Fatal("navigate pop payload with route validated successfully")
		}
		if err := navigate.VisitJSON(map[string]any{
			"window_id": "window-a",
			"mode":      string(windowmanager.NavigatePop),
		}); err != nil {
			t.Fatalf("navigate pop payload validation error = %v", err)
		}
		if err := navigate.VisitJSON(map[string]any{
			"window_id": "window-a",
			"route": map[string]any{
				"pathname": "/tasks",
				"search":   map[string]any{},
			},
		}); err != nil {
			t.Fatalf("navigate routed payload validation error = %v", err)
		}
		closePayload := componentSchema(t, document, "WindowManagerCloseWindowPayload")
		assertEnumValues(
			t,
			propertySchema(t, closePayload, "scope"),
			"tab",
			string(windowmanager.CloseScopeGroup),
			string(windowmanager.CloseScopeOthers),
			string(windowmanager.CloseScopeRight),
			string(windowmanager.CloseScopeLeft),
			string(windowmanager.CloseScopeAll),
		)
		openPayload := componentSchema(t, document, "WindowManagerOpenWindowPayload")
		windowSpec := propertySchema(t, openPayload, "window")
		assertNotRequired(t, windowSpec, "stack_target_window_id")

		for _, target := range []struct {
			path   string
			method string
		}{
			{path: windowManagerPath + "/preview", method: http.MethodPost},
			{path: windowManagerPath + "/commands", method: http.MethodPost},
			{path: windowManagerPath + "/layout", method: http.MethodPut},
		} {
			operation := operationFor(t, document, target.path, target.method)
			for _, status := range []int{
				http.StatusNotFound,
				http.StatusConflict,
				http.StatusUnprocessableEntity,
				http.StatusServiceUnavailable,
			} {
				assertResponseStatus(t, operation, status)
			}
		}
	})

	t.Run("Should describe recursive layout and snapshot-first stream frames", func(t *testing.T) {
		t.Parallel()
		document, err := Document()
		if err != nil {
			t.Fatalf("Document() error = %v", err)
		}
		node := componentSchema(t, document, "WindowManagerLayoutNode")
		assertSchemaHasAdditionalProperties(t, node, false)
		children := propertySchema(t, node, "children")
		if children.Items == nil || children.Items.Value != node {
			t.Fatalf("layout-node children do not resolve recursively: %#v", children.Items)
		}
		replacePayload := componentSchema(t, document, "WindowManagerReplaceLayoutPayload")
		replaceDocument := propertySchema(t, replacePayload, "document")
		desktops := propertySchema(t, replaceDocument, "desktops")
		if desktops.Items == nil || desktops.Items.Value == nil {
			t.Fatalf("replace-layout desktops items are unresolved: %#v", desktops.Items)
		}
		tabs := propertySchema(t, desktops.Items.Value, "tabs")
		if tabs.Items == nil || tabs.Items.Value == nil {
			t.Fatalf("replace-layout tabs items are unresolved: %#v", tabs.Items)
		}
		assertSchemaHasAdditionalProperties(t, propertySchema(t, tabs.Items.Value, "root"), false)

		for name, frameType := range map[string]string{
			"WindowManagerSnapshotFrame":  contract.WindowManagerFrameSnapshot,
			"WindowManagerEventFrame":     contract.WindowManagerFrameEvent,
			"WindowManagerClientFrame":    contract.WindowManagerFrameClient,
			"WindowManagerErrorFrame":     contract.WindowManagerFrameError,
			"WindowManagerHeartbeatFrame": contract.WindowManagerFrameHeartbeat,
		} {
			frame := componentSchema(t, document, name)
			assertEnumValues(t, propertySchema(t, frame, "type"), frameType)
		}

		stream := operationFor(t, document, windowManagerPath+"/stream", http.MethodGet)
		streamResponse := responseSchema(
			t,
			stream,
			http.StatusSwitchingProtocols,
			"application/json",
		)
		if len(streamResponse.OneOf) != 5 {
			t.Fatalf("101 response frame variants = %d, want 5", len(streamResponse.OneOf))
		}
		frameTypes := make([]string, 0, len(streamResponse.OneOf))
		for _, variant := range streamResponse.OneOf {
			if variant == nil || variant.Value == nil {
				t.Fatalf("unresolved 101 response frame schema = %#v", variant)
			}
			typeSchema := propertySchema(t, variant.Value, "type")
			if len(typeSchema.Enum) != 1 {
				t.Fatalf("101 response frame type enum = %v, want one value", typeSchema.Enum)
			}
			frameType, ok := typeSchema.Enum[0].(string)
			if !ok {
				t.Fatalf("101 response frame type = %#v, want string", typeSchema.Enum[0])
			}
			frameTypes = append(frameTypes, frameType)
		}
		slices.Sort(frameTypes)
		expectedFrameTypes := []string{
			contract.WindowManagerFrameClient,
			contract.WindowManagerFrameError,
			contract.WindowManagerFrameEvent,
			contract.WindowManagerFrameHeartbeat,
			contract.WindowManagerFrameSnapshot,
		}
		if !slices.Equal(frameTypes, expectedFrameTypes) {
			t.Fatalf("101 response frame types = %v", frameTypes)
		}
		afterRevision := parameterSchema(t, stream, "after_revision", openapi3.ParameterInQuery)
		if afterRevision.Max == nil || *afterRevision.Max != float64(contract.WindowManagerMaxSafeRevision) {
			t.Fatalf("after_revision maximum = %v", afterRevision.Max)
		}
		assertParameter(t, stream, "client_id", openapi3.ParameterInQuery, false)
		clientView := componentSchema(t, document, "WindowManagerClientView")
		presentationRevision := propertySchema(t, clientView, "presentation_revision")
		if presentationRevision.Max == nil ||
			*presentationRevision.Max != float64(contract.WindowManagerMaxSafeRevision) {
			t.Fatalf("presentation_revision maximum = %v", presentationRevision.Max)
		}
		snapshotFrame := componentSchema(t, document, "WindowManagerSnapshotFrame")
		if propertySchema(t, snapshotFrame, "client") == nil ||
			slices.Contains(snapshotFrame.Required, "client") {
			t.Fatalf("snapshot client must be optional: required = %v", snapshotFrame.Required)
		}
		for _, status := range []int{
			http.StatusSwitchingProtocols,
			http.StatusNotFound,
			http.StatusConflict,
			http.StatusUnprocessableEntity,
			http.StatusServiceUnavailable,
		} {
			assertResponseStatus(t, stream, status)
		}
	})

	t.Run("Should retain typed deprecated workspace override fields for current clients", func(t *testing.T) {
		t.Parallel()
		document, err := Document()
		if err != nil {
			t.Fatal(err)
		}
		overrides := componentSchema(t, document, "WorkspaceConfig")
		for _, name := range []string{
			"new_window_policy", "small_viewport_policy", "focus_policy", "focus_wrap", "focus_follows_pointer",
			"swap_modifier", "history_limit", "nav_stack_limit", "closed_entry_limit", "desktop_transition", "gaps", "shortcuts",
		} {
			if propertySchema(t, overrides, name) == nil {
				t.Fatalf("kept override %s has no schema", name)
			}
		}
		for _, name := range []string{"raise_on_focus", "drag_away_policy", "group_move_modifier"} {
			if !propertySchema(t, overrides, name).Deprecated {
				t.Fatalf("retired override %s is missing its deprecated schema", name)
			}
		}
		assertSchemaIncludesType(t, propertySchema(t, overrides, "raise_on_focus"), openapi3.TypeBoolean)
		marked := 0
		for _, entry := range []struct{ parent, property, component string }{
			{"WorkspaceConfig", "snap", "SnapConfig"},
			{"WorkspaceConfig", "bindings", "BindingsConfig"},
			{"SettingsWindowManagerConfigPayload", "snap", "SettingsWindowManagerSnapPayload"},
			{"SettingsWindowManagerConfigPayload", "bindings", "SettingsWindowManagerBindingPayload"},
		} {
			parent := componentSchema(t, document, entry.parent)
			propertySchema(t, parent, entry.property)
			property := parent.Properties[entry.property].Value
			component := componentSchema(t, document, entry.component)
			if !property.Deprecated || !strings.Contains(property.Description, "v0.8.0") || !component.Deprecated {
				t.Fatalf("retired %s.%s has no deprecated serialized component", entry.parent, entry.property)
			}
			marked++
		}
		if marked != 4 {
			t.Fatalf("deprecated snap/bindings references = %d, want 4", marked)
		}
		for _, parent := range []string{"WorkspaceConfig", "SettingsWindowManagerConfigPayload"} {
			policy := propertySchema(t, componentSchema(t, document, parent), "new_window_policy")
			if policy.Deprecated || policy.Description != `Deprecated value "floating": translated to "tab"; removed in v0.8.0.` {
				t.Fatalf("%s.new_window_policy = %+v, want a retired-value notice with an active field", parent, policy)
			}
		}
	})

	t.Run("Should describe tabs and window membership without retired topology", func(t *testing.T) {
		t.Parallel()
		document, err := Document()
		if err != nil {
			t.Fatalf("Document() error = %v", err)
		}
		snapshot := componentSchema(t, document, "WindowManagerSnapshot")
		if _, ok := snapshot.Properties["history"]; ok {
			t.Fatalf("WindowManagerSnapshot schema includes internal history")
		}
		assertRequired(t, snapshot, "closed_entry_count")
		desktops := propertySchema(t, snapshot, "desktops")
		if desktops.Items == nil || desktops.Items.Value == nil {
			t.Fatalf("WindowManagerSnapshot desktops are unresolved: %#v", desktops.Items)
		}
		assertRequired(t, desktops.Items.Value, "tabs", "active_tab_id")
		clientView := componentSchema(t, document, "WindowManagerClientView")
		assertRequired(t, clientView, "active_tab", "stack_active")
		windows := propertySchema(t, snapshot, "windows")
		windowRef := windows.AdditionalProperties.Schema
		if windowRef == nil || windowRef.Value == nil {
			t.Fatalf("window map value schema is unresolved: %#v", windows.AdditionalProperties)
		}
		assertRequired(t, windowRef.Value, "nav_stack", "tab_id", "desktop_id")
		for _, retired := range []string{"return_anchor", "pinned", "zoomed", "placement", "floating_rect"} {
			if _, exists := windowRef.Value.Properties[retired]; exists {
				t.Fatalf("retired window property %s remains in v6", retired)
			}
		}
		tabs := propertySchema(t, desktops.Items.Value, "tabs")
		assertRequired(t, tabs.Items.Value, "id", "pinned", "root")
		assertEnumValues(
			t,
			propertySchema(t, componentSchema(t, document, "WindowManagerLayoutNode"), "kind"),
			"leaf",
			"split",
		)
	})
}

func componentSchema(t *testing.T, document *openapi3.T, name string) *openapi3.Schema {
	t.Helper()
	if document.Components == nil {
		t.Fatal("OpenAPI components are nil")
	}
	schemaRef := document.Components.Schemas[name]
	if schemaRef == nil || schemaRef.Value == nil {
		t.Fatalf("component schema %q is missing or unresolved", name)
	}
	return schemaRef.Value
}

// Deprecations annotate the frozen v6 contract; the strict schema comparison still owns every field and type.
func deprecateRetiredWindowManagerWireFields(t *testing.T, baseline map[string]any) {
	t.Helper()
	counts := map[string]int{}
	var visit func(any)
	visit = func(value any) {
		switch node := value.(type) {
		case []any:
			for _, child := range node {
				visit(child)
			}
		case map[string]any:
			for _, child := range node {
				visit(child)
			}
			properties, _ := node["properties"].(map[string]any)
			if required, ok := node["required"].([]any); ok {
				node["required"] = slices.DeleteFunc(required, func(field any) bool {
					switch field {
					case "frame", "group_id", "minimize", "floating_rect", "insert_tiled":
						return true
					default:
						return false
					}
				})
				if len(node["required"].([]any)) == 0 {
					delete(node, "required")
				}
			}
			if property, ok := properties["new_window_policy"].(map[string]any); ok {
				property["description"] = `Deprecated value "floating": translated to "tab"; removed in v0.8.0.`
				counts["floating_policy"]++
			}
			if property, ok := properties["placement"].(map[string]any); ok {
				property["description"] = `Deprecated value "floating": translated to "tab"; removed in v0.8.0.`
				counts["floating_placement"]++
			}
			if property, ok := properties["arrangement"].(map[string]any); ok {
				property["description"] = `Deprecated value "stack": translated to adjacent tabs; removed in v0.8.0.`
				counts["stack_arrangement"]++
			}
			if _, status := properties["acknowledged"]; status {
				properties["state"].(map[string]any)["enum"] = []any{"none", "migrated", "failed"}
				properties["failure_revision"].(map[string]any)["description"] =
					"Topology revision installed on conversion failure; cleared after a successful retry."
				counts["migration_status"]++
			}
			if _, action := properties["action"]; action {
				properties["action"].(map[string]any)["enum"] = []any{"ack", "retry"}
				properties["replace"].(map[string]any)["description"] =
					"Replace the layout on retry when it changed after failure."
				node["required"] = []any{"action"}
				node["additionalProperties"] = false
				counts["migration_request"]++
			}
			for _, name := range []string{"floating", "floating_rect", "restore_window_id", "minimize", "insert_index", "frame", "group_id", "keep_frames", "group_ids", "stack_grouped", "stack_ungrouped", "stack_active", "window_floating", "window_stacked"} {
				if property, ok := properties[name].(map[string]any); ok {
					if alternatives, ok := property["oneOf"].([]any); ok {
						if len(alternatives) != 2 {
							t.Fatalf("retired %s has unexpected alternatives: %v", name, alternatives)
						}
						nullable, ok := alternatives[1].(map[string]any)
						if !ok || !reflect.DeepEqual(nullable["enum"], []any{nil}) {
							t.Fatalf("retired %s has no nullable alternative: %v", name, alternatives[1])
						}
						property = alternatives[0].(map[string]any)
					}
					property["deprecated"] = true
					property["description"] = "Deprecated: retained for compatibility; removed in v0.8.0. See the window-manager deprecation matrix for its replacement."
					// Reflection also carries field metadata onto collection elements.
					for _, key := range []string{"items", "additionalProperties"} {
						if child, ok := property[key].(map[string]any); ok {
							child["deprecated"] = true
							child["description"] = property["description"]
						}
					}
					counts[name]++
				}
			}
		}
	}
	paths := baseline["paths"].(map[string]any)
	for path, value := range paths {
		if strings.HasPrefix(path, windowManagerPath) {
			visit(value)
		}
	}
	components := baseline["components"].(map[string]any)["schemas"].(map[string]any)
	for name, value := range components {
		if strings.HasPrefix(name, "WindowManager") {
			visit(value)
		}
	}
	// Change-set fields occur in Event/Result/Preview components and commands/layout/preview responses (3+3).
	// stack_active occurs in six component views and seven inline client/command/layout/preview/stream views.
	want := map[string]int{
		"floating_policy":    16, // Eight component overrides and eight inline route overrides.
		"migration_request":  1,
		"floating_placement": 1,
		"stack_arrangement":  1,
		"migration_status":   11,
		"floating":           1,
		"floating_rect":      3,
		"restore_window_id":  1,
		"minimize":           1,
		"insert_index":       1,
		"frame":              3,
		"group_id":           3,
		"keep_frames":        1,
		"group_ids":          6,
		"stack_grouped":      6,
		"stack_ungrouped":    6,
		"stack_active":       13,
		"window_floating":    1,
		"window_stacked":     1,
	}
	if !maps.Equal(counts, want) {
		t.Fatalf("approved retired wire annotations = %v, want %v", counts, want)
	}
}
