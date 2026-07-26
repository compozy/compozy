// Suite: Window-manager OpenAPI and transport-neutral operation registry.
// Invariant: WM-TRANSPORT-002 documents the exact HTTP/UDS command schemas,
// statuses, frame discriminators, and JavaScript-safe revision bounds.
// Owning layer: internal/api/spec. Canonical suite: this file.
// Boundary IN: public paths, operation IDs, DTO shapes, and error responses.
// Boundary OUT: runtime handler behavior belongs to internal/api/core.
package spec

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/getkin/kin-openapi/openapi3"
)

func TestWindowManagerOpenAPIContract(t *testing.T) {
	t.Run("Should expose only the exact window-manager routes on HTTP and UDS", func(t *testing.T) {
		t.Parallel()
		expected := map[string]string{
			http.MethodGet + " " + windowManagerPath:                                      "getWindowManagerSnapshot",
			http.MethodPost + " " + windowManagerPath + "/preview":                        "previewWindowManagerCommand",
			http.MethodPost + " " + windowManagerPath + "/commands":                       "executeWindowManagerCommand",
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
			{contract.WindowManagerCommandWindowFocus, "WindowManagerFocusWindowPayload", true},
			{contract.WindowManagerCommandWindowMove, "WindowManagerMoveWindowPayload", false},
			{contract.WindowManagerCommandWindowSwap, "WindowManagerSwapWindowsPayload", false},
			{
				contract.WindowManagerCommandWindowToggleFloating,
				"WindowManagerToggleFloatingPayload",
				false,
			},
			{contract.WindowManagerCommandWindowZoom, "WindowManagerZoomWindowPayload", true},
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
		assertRequired(t, navigate, "window_id", "route")
		route := propertySchema(t, navigate, "route")
		assertRequired(t, route, "pathname", "search")
		assertSchemaHasAdditionalProperties(t, route, false)
		assertSchemaIncludesType(t, propertySchema(t, route, "pathname"), openapi3.TypeString)
		search := propertySchema(t, route, "search")
		assertSchemaIncludesType(t, search, openapi3.TypeObject)
		assertSchemaHasAdditionalProperties(t, search, true)

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
		groups := propertySchema(t, desktops.Items.Value, "groups")
		if groups.Items == nil || groups.Items.Value == nil {
			t.Fatalf("replace-layout groups items are unresolved: %#v", groups.Items)
		}
		assertSchemaHasAdditionalProperties(t, propertySchema(t, groups.Items.Value, "root"), false)

		for name, frameType := range map[string]string{
			"WindowManagerSnapshotFrame": contract.WindowManagerFrameSnapshot,
			"WindowManagerEventFrame":    contract.WindowManagerFrameEvent,
			"WindowManagerClientFrame":   contract.WindowManagerFrameClient,
			"WindowManagerErrorFrame":    contract.WindowManagerFrameError,
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
		if len(streamResponse.OneOf) != 4 {
			t.Fatalf("101 response frame variants = %d, want 4", len(streamResponse.OneOf))
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

	t.Run("Should describe optional exact source groups on return anchors", func(t *testing.T) {
		t.Parallel()
		document, err := Document()
		if err != nil {
			t.Fatalf("Document() error = %v", err)
		}
		snapshot := componentSchema(t, document, "WindowManagerSnapshot")
		windows := propertySchema(t, snapshot, "windows")
		windowRef := windows.AdditionalProperties.Schema
		if windowRef == nil || windowRef.Value == nil {
			t.Fatalf("window map value schema is unresolved: %#v", windows.AdditionalProperties)
		}
		returnAnchor := propertySchema(t, windowRef.Value, "return_anchor")
		assertNotRequired(t, returnAnchor, "source_group")
		sourceGroup := propertySchema(t, returnAnchor, "source_group")
		assertSchemaIncludesType(t, sourceGroup, openapi3.TypeObject)
		assertRequired(t, sourceGroup, "id", "frame", "root")
		root := propertySchema(t, sourceGroup, "root")
		children := propertySchema(t, root, "children")
		if children.Items == nil ||
			children.Items.Ref != "#/components/schemas/WindowManagerLayoutNode" ||
			children.Items.Value == nil {
			t.Fatalf("source-group root children do not resolve recursively: %#v", children.Items)
		}
		assertRequired(t, children.Items.Value, "id", "kind")
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
