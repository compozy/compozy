package httpapi_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/httpapi"
)

var ginPathParamPattern = regexp.MustCompile(`:([A-Za-z_][A-Za-z0-9_]*)`)

var openAPIOperationMethods = map[string]struct{}{
	"delete":  {},
	"get":     {},
	"head":    {},
	"options": {},
	"patch":   {},
	"post":    {},
	"put":     {},
	"trace":   {},
}

var browserRouteExclusions = map[string]struct{}{
	"DELETE /api/workspaces/{id}":            {},
	"GET /api/reviews/{slug}/rounds/{round}": {},
	"GET /api/runs/{run_id}/events":          {},
	"GET /api/tasks/{slug}/items":            {},
	"GET /api/workspaces/{id}":               {},
	"PATCH /api/workspaces/{id}":             {},
	"POST /api/daemon/stop":                  {},
	"POST /api/exec":                         {},
	"POST /api/reviews/{slug}/fetch":         {},
	"POST /api/tasks/{slug}/validate":        {},
	"POST /api/workspaces":                   {},
}

func TestBrowserOpenAPIContractMatchesRegisteredBrowserRoutes(t *testing.T) {
	t.Parallel()

	spec := loadBrowserOpenAPISpec(t)
	expected := []string{
		"GET /api/daemon/health",
		"GET /api/daemon/metrics",
		"GET /api/daemon/status",
		"GET /api/reviews/{slug}",
		"GET /api/reviews/{slug}/rounds/{round}/issues",
		"GET /api/reviews/{slug}/rounds/{round}/issues/{issue_id}",
		"GET /api/runs",
		"GET /api/runs/{run_id}",
		"GET /api/runs/{run_id}/snapshot",
		"GET /api/runs/{run_id}/stream",
		"GET /api/tasks",
		"GET /api/tasks/{slug}",
		"GET /api/tasks/{slug}/board",
		"GET /api/tasks/{slug}/items/{task_id}",
		"GET /api/tasks/{slug}/memory",
		"GET /api/tasks/{slug}/memory/files/{file_id}",
		"GET /api/tasks/{slug}/spec",
		"GET /api/ui/dashboard",
		"GET /api/workspaces",
		"POST /api/reviews/{slug}/rounds/{round}/runs",
		"POST /api/runs/{run_id}/cancel",
		"POST /api/sync",
		"POST /api/tasks/{slug}/archive",
		"POST /api/tasks/{slug}/runs",
		"POST /api/workspaces/resolve",
	}
	sort.Strings(expected)

	specRoutes := openAPIContractRouteKeys(t, spec)
	if diff := diffRoutes(expected, specRoutes); diff != "" {
		t.Fatalf("browser OpenAPI path set drifted:\n%s", diff)
	}

	gin.SetMode(gin.TestMode)
	handlers := core.NewHandlers(&core.HandlerConfig{TransportName: "test"})
	engine := gin.New()
	httpapi.RegisterRoutes(engine, handlers)

	registered := registeredBrowserRouteKeys(engine.Routes())
	if diff := diffRoutes(expected, registered); diff != "" {
		t.Fatalf("registered browser route set drifted:\n%s", diff)
	}
}

func TestBrowserOpenAPIContractKeepsWorkspaceContextAndProblemSemantics(t *testing.T) {
	t.Parallel()

	spec := loadBrowserOpenAPISpec(t)
	components := getMap(t, spec, "components")
	parameters := getMap(t, components, "parameters")
	activeWorkspaceHeader := getMap(t, parameters, "ActiveWorkspaceHeader")
	if activeWorkspaceHeader["in"] != "header" {
		t.Fatalf("ActiveWorkspaceHeader.in = %v, want header", activeWorkspaceHeader["in"])
	}
	if activeWorkspaceHeader["required"] != true {
		t.Fatalf("ActiveWorkspaceHeader.required = %v, want true", activeWorkspaceHeader["required"])
	}

	workspaceScopedRoutes := []string{
		"GET /api/ui/dashboard",
		"GET /api/tasks",
		"GET /api/tasks/{slug}",
		"GET /api/tasks/{slug}/spec",
		"GET /api/tasks/{slug}/memory",
		"GET /api/tasks/{slug}/memory/files/{file_id}",
		"GET /api/tasks/{slug}/board",
		"GET /api/tasks/{slug}/items/{task_id}",
		"GET /api/reviews/{slug}",
		"GET /api/reviews/{slug}/rounds/{round}/issues",
		"GET /api/reviews/{slug}/rounds/{round}/issues/{issue_id}",
		"POST /api/tasks/{slug}/runs",
		"POST /api/tasks/{slug}/archive",
		"POST /api/reviews/{slug}/rounds/{round}/runs",
		"POST /api/sync",
	}
	for _, routeKey := range workspaceScopedRoutes {
		operation := getOperation(t, spec, routeKey)
		if !hasParameterRef(operation, "#/components/parameters/ActiveWorkspaceHeader") {
			t.Fatalf("%s is missing ActiveWorkspaceHeader", routeKey)
		}
		if hasParameterRef(operation, "#/components/parameters/WorkspaceQuery") {
			t.Fatalf("%s unexpectedly advertises WorkspaceQuery", routeKey)
		}
		if !hasResponse(operation, "412") {
			t.Fatalf("%s is missing 412 stale-workspace response", routeKey)
		}
	}

	postBodies := map[string]string{
		"POST /api/tasks/{slug}/runs":                  "#/components/schemas/TaskRunRequest",
		"POST /api/tasks/{slug}/archive":               "#/components/schemas/WorkflowRefRequest",
		"POST /api/reviews/{slug}/rounds/{round}/runs": "#/components/schemas/ReviewRunRequest",
		"POST /api/sync":                               "#/components/schemas/SyncRequest",
		"POST /api/workspaces/resolve":                 "#/components/schemas/WorkspaceResolveRequest",
	}
	for routeKey, wantRef := range postBodies {
		operation := getOperation(t, spec, routeKey)
		if got := requestBodySchemaRef(t, operation); got != wantRef {
			t.Fatalf("%s request body ref = %q, want %q", routeKey, got, wantRef)
		}
	}

	taskRunSchema := getSchema(t, spec, "TaskRunRequest")
	if schemaRequires(taskRunSchema, "workspace") {
		t.Fatal("TaskRunRequest must not require workspace")
	}
	reviewRunSchema := getSchema(t, spec, "ReviewRunRequest")
	if schemaRequires(reviewRunSchema, "workspace") {
		t.Fatal("ReviewRunRequest must not require workspace")
	}
	workflowRefSchema := getSchema(t, spec, "WorkflowRefRequest")
	if schemaRequires(workflowRefSchema, "workspace") {
		t.Fatal("WorkflowRefRequest must not require workspace")
	}

	runSnapshot := getSchema(t, spec, "RunSnapshotPayload")
	if !schemaRequires(runSnapshot, "run") {
		t.Fatal("RunSnapshotPayload must require run")
	}
	properties := getMap(t, runSnapshot, "properties")
	if _, ok := properties["snapshot"]; ok {
		t.Fatal("RunSnapshotPayload must not wrap the payload under snapshot")
	}
	if _, ok := properties["jobs"]; !ok {
		t.Fatal("RunSnapshotPayload must expose jobs")
	}

	transportError := getSchema(t, spec, "TransportError")
	for _, field := range []string{"request_id", "code", "message"} {
		if !schemaRequires(transportError, field) {
			t.Fatalf("TransportError must require %s", field)
		}
	}

	for _, routeKey := range []string{
		"POST /api/tasks/{slug}/runs",
		"POST /api/tasks/{slug}/archive",
		"POST /api/reviews/{slug}/rounds/{round}/runs",
		"POST /api/sync",
		"POST /api/runs/{run_id}/cancel",
		"POST /api/workspaces/resolve",
	} {
		operation := getOperation(t, spec, routeKey)
		if !hasResponse(operation, "403") {
			t.Fatalf("%s is missing 403 browser security response", routeKey)
		}
	}
}

func loadBrowserOpenAPISpec(t *testing.T) map[string]any {
	t.Helper()

	path := filepath.Join("..", "..", "..", "openapi", "compozy-daemon.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}

	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v", path, err)
	}
	return doc
}

func openAPIContractRouteKeys(t *testing.T, spec map[string]any) []string {
	t.Helper()

	paths := getMap(t, spec, "paths")
	keys := make([]string, 0, len(paths))
	for path, rawPathItem := range paths {
		pathItem, ok := rawPathItem.(map[string]any)
		if !ok {
			t.Fatalf("paths[%s] = %T, want object", path, rawPathItem)
		}
		for method := range pathItem {
			if !isOpenAPIOperationMethod(method) {
				continue
			}
			keys = append(keys, strings.ToUpper(method)+" "+path)
		}
	}
	sort.Strings(keys)
	return keys
}

func registeredBrowserRouteKeys(routes gin.RoutesInfo) []string {
	keys := make([]string, 0, len(routes))
	seen := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		key := route.Method + " " + ginPathParamPattern.ReplaceAllString(route.Path, "{$1}")
		if _, skip := browserRouteExclusions[key]; skip {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func isOpenAPIOperationMethod(method string) bool {
	_, ok := openAPIOperationMethods[strings.ToLower(strings.TrimSpace(method))]
	return ok
}

func getOperation(t *testing.T, spec map[string]any, routeKey string) map[string]any {
	t.Helper()

	parts := stringsSplitN(routeKey, " ", 2)
	if len(parts) != 2 {
		t.Fatalf("invalid route key %q", routeKey)
	}
	method := strings.ToLower(parts[0])
	path := parts[1]

	paths := getMap(t, spec, "paths")
	pathItem := getMap(t, paths, path)
	return getMap(t, pathItem, method)
}

func stringsSplitN(s string, sep string, n int) []string {
	return strings.SplitN(s, sep, n)
}

func getSchema(t *testing.T, spec map[string]any, name string) map[string]any {
	t.Helper()

	components := getMap(t, spec, "components")
	schemas := getMap(t, components, "schemas")
	return getMap(t, schemas, name)
}

func getMap(t *testing.T, from map[string]any, key string) map[string]any {
	t.Helper()

	raw, ok := from[key]
	if !ok {
		t.Fatalf("missing key %q", key)
	}
	value, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("%s = %T, want object", key, raw)
	}
	return value
}

func hasParameterRef(operation map[string]any, wantRef string) bool {
	for _, parameter := range getParameters(operation) {
		if parameter["$ref"] == wantRef {
			return true
		}
	}
	return false
}

func getParameters(operation map[string]any) []map[string]any {
	raw, ok := operation["parameters"]
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}

	parameters := make([]map[string]any, 0, len(items))
	for _, item := range items {
		value, ok := item.(map[string]any)
		if ok {
			parameters = append(parameters, value)
		}
	}
	return parameters
}

func hasResponse(operation map[string]any, status string) bool {
	responses, ok := operation["responses"].(map[string]any)
	if !ok {
		return false
	}
	_, ok = responses[status]
	return ok
}

func requestBodySchemaRef(t *testing.T, operation map[string]any) string {
	t.Helper()

	requestBody := getMap(t, operation, "requestBody")
	content := getMap(t, requestBody, "content")
	jsonBody := getMap(t, content, "application/json")
	schema := getMap(t, jsonBody, "schema")
	ref, ok := schema["$ref"].(string)
	if !ok {
		t.Fatalf("request body schema ref = %T, want string", schema["$ref"])
	}
	return ref
}

func schemaRequires(schema map[string]any, field string) bool {
	raw, ok := schema["required"]
	if !ok {
		return false
	}
	items, ok := raw.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if item == field {
			return true
		}
	}
	return false
}
