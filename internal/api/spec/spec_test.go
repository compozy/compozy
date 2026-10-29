package spec

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/workspaceaccess"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	collisionfixture "github.com/compozy/compozy/internal/api/spec/testdata/componentnames/collision/session"
	sessionfixture "github.com/compozy/compozy/internal/api/spec/testdata/componentnames/session"
	taskfixture "github.com/compozy/compozy/internal/api/spec/testdata/componentnames/task"
	automationpkg "github.com/compozy/compozy/internal/automation"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	"github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/loop/dsl"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/tools"
	"github.com/getkin/kin-openapi/openapi3"
)

func TestValidateOperationRegistry(t *testing.T) {
	t.Parallel()

	t.Run("Should accept the complete production registry", func(t *testing.T) {
		t.Parallel()

		if err := validateOperationRegistry(Operations()); err != nil {
			t.Fatalf("validateOperationRegistry() error = %v", err)
		}
	})

	tests := []struct {
		name   string
		mutate func([]OperationSpec) []OperationSpec
		want   string
	}{
		{
			name: "Should reject an empty registry",
			mutate: func([]OperationSpec) []OperationSpec {
				return nil
			},
			want: "registry is empty",
		},
		{
			name: "Should reject unsupported methods",
			mutate: func(operations []OperationSpec) []OperationSpec {
				operations[0].Method = "TRACE"
				return operations
			},
			want: "unsupported method",
		},
		{
			name: "Should reject paths outside the API root",
			mutate: func(operations []OperationSpec) []OperationSpec {
				operations[0].Path = "/internal/status"
				return operations
			},
			want: "absolute /api path",
		},
		{
			name: "Should reject duplicate operation IDs",
			mutate: func(operations []OperationSpec) []OperationSpec {
				duplicate := operations[0]
				duplicate.Path = "/api/example/duplicate"
				return append(operations, duplicate)
			},
			want: "duplicate operation ID",
		},
		{
			name: "Should reject duplicate routes",
			mutate: func(operations []OperationSpec) []OperationSpec {
				duplicate := operations[0]
				duplicate.OperationID = "duplicateExample"
				return append(operations, duplicate)
			},
			want: "duplicate route",
		},
		{
			name: "Should reject missing transports",
			mutate: func(operations []OperationSpec) []OperationSpec {
				operations[0].Transports = nil
				return operations
			},
			want: "has no transports",
		},
		{
			name: "Should reject unknown transports",
			mutate: func(operations []OperationSpec) []OperationSpec {
				operations[0].Transports = []Transport{"unknown"}
				return operations
			},
			want: "unknown transport",
		},
		{
			name: "Should reject undeclared path parameters",
			mutate: func(operations []OperationSpec) []OperationSpec {
				operations[0].Path = "/api/example/{id}"
				return operations
			},
			want: "requires exactly one declaration",
		},
		{
			name: "Should reject optional path parameters",
			mutate: func(operations []OperationSpec) []OperationSpec {
				operations[0].Path = "/api/example/{id}"
				operations[0].Parameters = []ParameterSpec{{Name: "id", In: "path"}}
				return operations
			},
			want: "must be required",
		},
		{
			name: "Should reject operations without responses",
			mutate: func(operations []OperationSpec) []OperationSpec {
				operations[0].Responses = nil
				return operations
			},
			want: "has no responses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			operations := []OperationSpec{validRegistryOperation()}
			err := validateOperationRegistry(tt.mutate(operations))
			if err == nil {
				t.Fatalf("validateOperationRegistry() error = nil, want %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("validateOperationRegistry() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestDocumentDescribesAgentNameRequestConstraints(t *testing.T) {
	t.Parallel()

	doc, err := Document()
	if err != nil {
		t.Fatalf("Document() error = %v", err)
	}

	t.Run("Should describe strict authored and required agent identities", func(t *testing.T) {
		t.Parallel()

		createAgent := jsonRequestSchema(t, operationFor(t, doc, "/api/agents", http.MethodPost))
		assertRequired(t, createAgent, "agent")
		createAgentPayload := propertySchema(t, createAgent, "agent")
		assertRequired(t, createAgentPayload, "name")
		assertAgentNameRequestSchema(
			t,
			"create agent name",
			propertySchema(t, createAgentPayload, "name"),
			compozyconfig.AgentNamePattern,
		)

		duplicateAgent := jsonRequestSchema(t, operationFor(t, doc, "/api/agents/{name}/duplicate", http.MethodPost))
		assertRequired(t, duplicateAgent, "name")
		assertAgentNameRequestSchema(
			t,
			"duplicate agent name",
			propertySchema(t, duplicateAgent, "name"),
			compozyconfig.AgentNamePattern,
		)

		spawnAgent := jsonRequestSchema(t, operationFor(t, doc, "/api/agent/spawn", http.MethodPost))
		assertRequired(t, spawnAgent, "agent_name")
		assertAgentNameRequestSchema(
			t,
			"spawn agent_name",
			propertySchema(t, spawnAgent, "agent_name"),
			compozyconfig.AgentNamePattern,
		)
	})

	t.Run("Should describe empty-or-canonical reference fields without changing requiredness", func(t *testing.T) {
		t.Parallel()

		createSession := jsonRequestSchema(t, operationFor(t, doc, "/api/sessions", http.MethodPost))
		assertNotRequired(t, createSession, "agent_name")
		assertAgentNameRequestSchema(
			t,
			"create session agent_name",
			propertySchema(t, createSession, "agent_name"),
			compozyconfig.OptionalAgentNamePattern,
		)

		createWorkspace := jsonRequestSchema(t, operationFor(t, doc, "/api/workspaces", http.MethodPost))
		assertNotRequired(t, createWorkspace, "default_agent")
		assertAgentNameRequestSchema(
			t,
			"create workspace default_agent",
			propertySchema(t, createWorkspace, "default_agent"),
			compozyconfig.OptionalAgentNamePattern,
		)

		updateWorkspace := jsonRequestSchema(t, operationFor(t, doc, "/api/workspaces/{id}", http.MethodPatch))
		assertRequired(t, updateWorkspace, "default_agent")
		assertAgentNameRequestSchema(
			t,
			"update workspace default_agent",
			propertySchema(t, updateWorkspace, "default_agent"),
			compozyconfig.OptionalAgentNamePattern,
		)

		createJob := jsonRequestSchema(t, operationFor(t, doc, "/api/automation/jobs", http.MethodPost))
		assertRequired(t, createJob, "agent_name")
		assertAgentNameRequestSchema(
			t,
			"create job agent_name",
			propertySchema(t, createJob, "agent_name"),
			compozyconfig.OptionalAgentNamePattern,
		)

		createTrigger := jsonRequestSchema(t, operationFor(t, doc, "/api/automation/triggers", http.MethodPost))
		assertRequired(t, createTrigger, "agent_name")
		assertAgentNameRequestSchema(
			t,
			"create trigger agent_name",
			propertySchema(t, createTrigger, "agent_name"),
			compozyconfig.OptionalAgentNamePattern,
		)

		updatePersona := jsonRequestSchema(t, operationFor(t, doc, "/api/settings/persona", http.MethodPatch))
		defaults := propertySchema(t, updatePersona, "config")
		assertRequired(t, defaults, "agent")
		assertAgentNameRequestSchema(
			t,
			"settings defaults agent",
			propertySchema(t, defaults, "agent"),
			compozyconfig.OptionalAgentNamePattern,
		)

		updateRoles := jsonRequestSchema(t, operationFor(t, doc, "/api/settings/roles", http.MethodPatch))
		autoTitleRole := propertySchema(t, propertySchema(t, updateRoles, "config"), "auto_title")
		assertRequired(t, autoTitleRole, "agent")
		assertAgentNameRequestSchema(
			t,
			"settings role agent",
			propertySchema(t, autoTitleRole, "agent"),
			compozyconfig.OptionalAgentNamePattern,
		)
	})
}

func TestDocumentDescribesWorkspaceProfileHints(t *testing.T) {
	t.Parallel()
	t.Run("Should describe workspace profile hints", func(t *testing.T) {
		t.Parallel()

		doc, err := Document()
		if err != nil {
			t.Fatalf("Document() error = %v", err)
		}
		workspace := jsonResponseSchema(t, operationFor(t, doc, "/api/workspaces/{id}", http.MethodGet), http.StatusOK)
		hints := propertySchema(t, workspace, "profile_hints")
		if hints.Type == nil || !hints.Type.Is("array") || hints.Items == nil || hints.Items.Value == nil {
			t.Fatalf("profile_hints schema = %#v, want array items", hints)
		}
		hint := hints.Items.Value
		assertRequired(t, hint, "name", "path", "message", "action")
	})
}

func TestDocumentDescribesSessionAttachmentContracts(t *testing.T) {
	t.Parallel()

	doc, err := Document()
	if err != nil {
		t.Fatalf("Document() error = %v", err)
	}

	t.Run("Should describe multipart upload, binary attachment reads, and deletion", func(t *testing.T) {
		t.Parallel()

		upload := operationFor(
			t,
			doc,
			"/api/workspaces/{workspace_id}/sessions/{session_id}/attachments",
			http.MethodPost,
		)
		if upload.RequestBody == nil || upload.RequestBody.Value == nil {
			t.Fatal("attachment upload is missing a request body")
		}
		if !upload.RequestBody.Value.Required {
			t.Fatal("attachment upload request body is optional")
		}
		if len(upload.RequestBody.Value.Content) != 1 {
			t.Fatalf("attachment upload content types = %d, want one", len(upload.RequestBody.Value.Content))
		}
		form := upload.RequestBody.Value.Content.Get(RequestUploadForm)
		if form == nil || form.Schema == nil || form.Schema.Value == nil {
			t.Fatalf("missing %s upload schema", RequestUploadForm)
		}
		if form.Schema.Value.Type == nil || !form.Schema.Value.Type.Is("object") {
			t.Fatalf("upload schema type = %#v, want object", form.Schema.Value.Type)
		}
		assertSchemaHasAdditionalProperties(t, form.Schema.Value, false)
		assertRequired(t, form.Schema.Value, "file")
		file := propertySchema(t, form.Schema.Value, "file")
		if file.Type == nil || !file.Type.Is("string") || file.Format != "binary" {
			t.Fatalf("file schema = %#v, want string/binary", file)
		}

		bytesRead := operationFor(
			t,
			doc,
			"/api/workspaces/{workspace_id}/sessions/{session_id}/attachments/{attachment_id}/bytes",
			http.MethodGet,
		)
		bytesSchema := responseSchema(t, bytesRead, http.StatusOK, "*/*")
		if bytesSchema.Type == nil || !bytesSchema.Type.Is("string") || bytesSchema.Format != "binary" {
			t.Fatalf("bytes response schema = %#v, want string/binary", bytesSchema)
		}

		deleteOperation := operationFor(
			t,
			doc,
			"/api/workspaces/{workspace_id}/sessions/{session_id}/attachments/{attachment_id}",
			http.MethodDelete,
		)
		assertResponseStatus(t, deleteOperation, http.StatusNoContent)
	})
}

func validRegistryOperation() OperationSpec {
	return OperationSpec{
		AppAccess:   AppAccessDenied,
		Method:      httpMethodGet,
		Path:        "/api/example",
		OperationID: "getExample",
		Transports:  []Transport{TransportHTTP},
		Responses:   []ResponseSpec{{Status: http.StatusOK, Description: "OK"}},
	}
}

// TestDocumentTracksRequiredFieldsAndEnums checks public schemas and route selectors against their contracts.
func TestDocumentTracksRequiredFieldsAndEnums(t *testing.T) {
	t.Parallel()

	doc, err := Document()
	if err != nil {
		t.Fatalf("Document() error = %v", err)
	}

	tests := []struct {
		name  string
		check func(t *testing.T, doc *openapi3.T)
	}{

		{
			name: "Should describe the inactive-session native compaction refusal",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()
				compact := operationFor(
					t, doc, "/api/workspaces/{workspace_id}/sessions/{session_id}/compact", http.MethodPost,
				)
				refusal := jsonResponseSchema(t, compact, http.StatusBadRequest)
				description := compact.Responses.Status(http.StatusBadRequest).Value.Description
				if description == nil || !strings.Contains(*description, "session_not_promptable") {
					t.Fatalf("inactive-session response description = %v", description)
				}
				assertRequired(t, refusal, "error")
				code := propertySchema(t, refusal, "code")
				if code.Type == nil || !code.Type.Is("string") {
					t.Fatalf("refusal code schema = %#v, want string", code)
				}
			},
		},

		{
			name: "Should describe optional experimental compaction occupancy fields",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()
				usage := jsonResponseSchema(
					t,
					operationFor(t, doc, "/api/workspaces/{workspace_id}/sessions/{session_id}/usage", http.MethodGet),
					http.StatusOK,
				)
				context := propertySchema(t, propertySchema(t, usage, "usage"), "context")
				cleared := propertySchema(t, context, "cleared_by")
				turns := jsonResponseSchema(
					t,
					operationFor(
						t,
						doc,
						"/api/workspaces/{workspace_id}/sessions/{session_id}/usage/turns",
						http.MethodGet,
					),
					http.StatusOK,
				)
				marker := propertySchema(t, turns, "compactions").Items.Value
				after := propertySchema(t, marker, "context_after")
				for _, schema := range []*openapi3.Schema{cleared, after} {
					if schema.Extensions["x-stability"] != "experimental" ||
						!strings.Contains(schema.Description, "first terminal") {
						t.Fatalf("experimental chronology missing: %#v", schema)
					}
				}
				assertRequired(t, cleared, "compaction_id", "sequence")
				assertRequired(t, after, "used", "sequence")
				if slices.Contains(context.Required, "cleared_by") ||
					slices.Contains(marker.Required, "context_after") ||
					slices.Contains(after.Required, "size") {
					t.Fatal("unknown occupancy fields must remain optional")
				}
				for _, target := range []struct {
					schema *openapi3.Schema
					field  string
				}{{after, "used"}, {after, "size"}, {after, "sequence"}, {cleared, "sequence"}} {
					value := propertySchema(t, target.schema, target.field)
					if !value.Type.Is("integer") || value.Format != "int64" {
						t.Fatalf("%s must be int64: %#v", target.field, value)
					}
				}
			},
		},
		{
			name: "ShouldDescribeDaemonDrainAndAdmissionContracts",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				for _, target := range []struct {
					path string
				}{
					{path: "/api/drain"},
					{path: "/api/undrain"},
				} {
					operation := operationFor(t, doc, target.path, http.MethodPost)
					response := jsonResponseSchema(t, operation, http.StatusOK)
					assertRequired(t, response, "state")
					assertEnumValues(
						t,
						propertySchema(t, response, "state"),
						string(contract.DrainStateActive),
						string(contract.DrainStateDraining),
					)
					assertResponseStatus(t, operation, http.StatusServiceUnavailable)
				}

				admissionOperations := []struct {
					path   string
					method string
				}{
					{path: specSessionsPath, method: http.MethodPost},
					{path: "/api/workspaces/{workspace_id}/sessions/{session_id}/clear", method: http.MethodPost},
					{path: "/api/workspaces/{workspace_id}/sessions/{session_id}/prompt", method: http.MethodPost},
					{path: "/api/tasks/{id}/runs", method: http.MethodPost},
					{path: specAgentTaskClaimNextPath, method: http.MethodPost},
				}
				for _, target := range admissionOperations {
					operation := operationFor(t, doc, target.path, target.method)
					assertResponseStatus(t, operation, http.StatusServiceUnavailable)
				}
			},
		},
		{
			name: "ShouldDescribeDurablePromptIdentityContracts",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				prompt := operationFor(
					t,
					doc,
					"/api/workspaces/{workspace_id}/sessions/{session_id}/prompt",
					http.MethodPost,
				)
				promptRequest := jsonRequestSchema(t, prompt)
				assertRequired(t, promptRequest, "message_id", "idempotency_key")
				assertPropertyAbsent(t, promptRequest, "messageId")
				assertResponseStatus(t, prompt, http.StatusConflict)
				for _, status := range []int{http.StatusOK, http.StatusAccepted} {
					response := jsonResponseSchema(t, prompt, status)
					assertRequired(t, response, "prompt")
					payload := propertySchema(t, response, "prompt")
					assertRequired(t, payload, "status", "message_id", "idempotency_key", "replayed")
				}

				steer := operationFor(
					t,
					doc,
					"/api/workspaces/{workspace_id}/sessions/{session_id}/steer",
					http.MethodPost,
				)
				steerRequest := jsonRequestSchema(t, steer)
				assertRequired(t, steerRequest, "text", "message_id", "idempotency_key")
				assertPropertyAbsent(t, steerRequest, "messageId")
				assertResponseStatus(t, steer, http.StatusConflict)
			},
		},
		{
			name: "ShouldDescribeLogProfileReadModes",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				for _, path := range []string{"/api/logs", "/api/logs/stream"} {
					operation := operationFor(t, doc, path, http.MethodGet)
					assertParameter(t, operation, "profile", openapi3.ParameterInQuery, false)
					assertParameter(t, operation, "all_profiles", openapi3.ParameterInQuery, false)
				}
			},
		},

		{
			name: "ShouldDescribeAutomationProfileContracts",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				reads := []struct {
					path   string
					method string
				}{
					{path: "/api/automation/jobs", method: http.MethodGet},
					{path: "/api/automation/jobs/{id}", method: http.MethodGet},
					{path: "/api/automation/jobs/{id}/runs", method: http.MethodGet},
					{path: "/api/automation/triggers", method: http.MethodGet},
					{path: "/api/automation/triggers/{id}", method: http.MethodGet},
					{path: "/api/automation/triggers/{id}/runs", method: http.MethodGet},
					{path: "/api/automation/runs", method: http.MethodGet},
					{path: "/api/automation/runs/{id}", method: http.MethodGet},
				}
				for _, endpoint := range reads {
					operation := operationFor(t, doc, endpoint.path, endpoint.method)
					assertParameter(t, operation, "profile", openapi3.ParameterInQuery, false)
					assertParameter(t, operation, "all_profiles", openapi3.ParameterInQuery, false)
				}

				mutations := []struct {
					path   string
					method string
				}{
					{path: "/api/automation/jobs", method: http.MethodPost},
					{path: "/api/automation/jobs/{id}", method: http.MethodPatch},
					{path: "/api/automation/jobs/{id}", method: http.MethodDelete},
					{path: "/api/automation/jobs/{id}/trigger", method: http.MethodPost},
					{path: "/api/automation/triggers", method: http.MethodPost},
					{path: "/api/automation/triggers/{id}", method: http.MethodPatch},
					{path: "/api/automation/triggers/{id}", method: http.MethodDelete},
				}
				for _, endpoint := range mutations {
					operation := operationFor(t, doc, endpoint.path, endpoint.method)
					assertParameter(t, operation, "profile", openapi3.ParameterInQuery, false)
					assertParameterAbsent(t, operation, "all_profiles", openapi3.ParameterInQuery)
				}
			},
		},
		{
			name: "ShouldDescribeSessionProfileContracts",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				profileReads := []struct {
					path   string
					method string
				}{
					{path: "/api/workspaces/{workspace_id}/sessions/{session_id}", method: http.MethodGet},
				}
				for _, endpoint := range profileReads {
					operation := operationFor(t, doc, endpoint.path, endpoint.method)
					assertParameter(t, operation, "profile", openapi3.ParameterInQuery, false)
					assertParameter(t, operation, "all_profiles", openapi3.ParameterInQuery, false)
				}

				profileMutations := []struct {
					path   string
					method string
				}{
					{path: "/api/workspaces/{workspace_id}/sessions/{session_id}/stop", method: http.MethodPost},
					{path: "/api/workspaces/{workspace_id}/sessions/{session_id}/archive", method: http.MethodPost},
					{path: "/api/workspaces/{workspace_id}/sessions/{session_id}/rewind", method: http.MethodPost},
					{path: "/api/workspaces/{workspace_id}/sessions/{session_id}/unarchive", method: http.MethodPost},
					{path: "/api/workspaces/{workspace_id}/sessions/{session_id}", method: http.MethodPatch},
					{path: "/api/workspaces/{workspace_id}/subagents/{subagent_id}/cancel", method: http.MethodPost},
				}
				for _, endpoint := range profileMutations {
					operation := operationFor(t, doc, endpoint.path, endpoint.method)
					assertParameter(t, operation, "profile", openapi3.ParameterInQuery, false)
					assertParameterAbsent(t, operation, "all_profiles", openapi3.ParameterInQuery)
				}
			},
		},
		{
			name: "ShouldDescribeBoundedLoopCatalogContract",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				listLoops := operationFor(t, doc, "/api/workspaces/{workspace_id}/loops", "GET")
				for _, parameter := range []string{"q", "kind", "category", "status", "sort", "cursor", "limit"} {
					assertParameter(t, listLoops, parameter, openapi3.ParameterInQuery, false)
				}
				assertEnumValues(
					t,
					parameterSchema(t, listLoops, "kind", openapi3.ParameterInQuery),
					"read_only",
					"workspace",
				)
				assertEnumValues(
					t,
					parameterSchema(t, listLoops, "status", openapi3.ParameterInQuery),
					contract.LoopRunStatusValues()...,
				)
				assertEnumValues(
					t,
					parameterSchema(t, listLoops, "sort", openapi3.ParameterInQuery),
					"name",
				)

				response := jsonResponseSchema(t, listLoops, 200)
				assertRequired(t, response, "loops", "facets", "page")
				assertRequired(t, propertySchema(t, response, "page"), "has_more", "total", "limit")
				assertRequired(t, propertySchema(t, response, "facets"), "kinds", "categories", "statuses")
				loops := propertySchema(t, response, "loops")
				if loops.Items == nil || loops.Items.Value == nil {
					t.Fatal("expected Loop catalog entries to define an items schema")
				}
				assertRequired(
					t,
					loops.Items.Value,
					"name",
					"version",
					"source",
					"catalog",
					"contract",
					"aggregate_30d",
					"success_rate_30d",
				)
				lastRun := propertySchema(t, loops.Items.Value, "last_run")
				assertRequired(t, lastRun, "id", "status", "created_at")
				for _, richField := range []string{
					"workspace_id",
					"loop_name",
					"generation",
					"start_metadata",
					"inputs",
				} {
					assertPropertyAbsent(t, lastRun, richField)
				}
				for _, status := range []int{400, 404, 410, 503, 500} {
					assertResponseStatus(t, listLoops, status)
				}
			},
		},
		{
			name: "ShouldDescribeSessionListRequiredFieldsAndEnums",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				listSessions := operationFor(t, doc, "/api/sessions", "GET")
				listSessionsSchema := jsonResponseSchema(t, listSessions, 200)
				assertRequired(t, listSessionsSchema, "sessions", "page")
				assertRequired(t, propertySchema(t, listSessionsSchema, "page"), "has_more", "limit")
				assertNotRequired(t, propertySchema(t, listSessionsSchema, "page"), "total")
				assertParameter(t, listSessions, "skip_total", openapi3.ParameterInQuery, false)
				assertParameter(t, listSessions, "search_fields", openapi3.ParameterInQuery, false)
				facetsOperation := operationFor(t, doc, "/api/sessions/facets", "GET")
				assertParameter(t, facetsOperation, "parent", openapi3.ParameterInQuery, false)
				assertParameter(t, facetsOperation, "root", openapi3.ParameterInQuery, false)
				assertParameter(t, facetsOperation, "resumable", openapi3.ParameterInQuery, false)
				facetsResponse := jsonResponseSchema(t, facetsOperation, 200)
				assertRequired(t, facetsResponse, "facets", "by_workspace")
				assertRequired(
					t,
					propertySchema(t, facetsResponse, "facets"),
					"all",
					"needs_you",
					"working",
					"finished",
					"idle",
					"terminal_approvals",
				)

				sessionsSchema := propertySchema(t, listSessionsSchema, "sessions")
				if sessionsSchema.Items == nil || sessionsSchema.Items.Value == nil {
					t.Fatal("expected sessions to define an items schema")
				}

				sessionSchema := sessionsSchema.Items.Value
				assertRequired(
					t,
					sessionSchema,
					"id",
					"agent_name",
					"runtime",
					"state",
					"badge",
					"attachable",
					"created_at",
					"updated_at",
				)
				assertPropertyAbsent(t, sessionSchema, "provider")
				assertPropertyAbsent(t, sessionSchema, "model")
				assertPropertyAbsent(t, sessionSchema, "reasoning_effort")
				assertPropertyAbsent(t, sessionSchema, "speed")

				runtimeSchema := propertySchema(t, sessionSchema, "runtime")
				assertRequired(t, runtimeSchema, "status")
				assertNotRequired(
					t,
					runtimeSchema,
					"transition",
					"failure",
					"effective",
					"acp_session_id",
					"acp_caps",
				)
				effectiveRuntimeSchema := propertySchema(t, runtimeSchema, "effective")
				assertRequired(t, effectiveRuntimeSchema, "provider")
				assertNotRequired(
					t,
					effectiveRuntimeSchema,
					"model",
					"reasoning_effort",
					"speed",
					"speed_resolution",
				)
				assertNotRequired(
					t,
					sessionSchema,
					"workspace_id",
					"workspace_path",
					"stop_reason",
					"stop_detail",
					"attached_to",
					"attach_expires_at",
				)
				assertParameter(t, listSessions, "resumable", openapi3.ParameterInQuery, false)
				assertParameter(t, listSessions, "state", openapi3.ParameterInQuery, false)
				assertEnumValues(
					t,
					parameterSchema(t, listSessions, "type", openapi3.ParameterInQuery),
					"user",
					"system",
					"coordinator",
					"spawned",
				)
				assertParameter(t, listSessions, "agent", openapi3.ParameterInQuery, false)
				assertParameter(t, listSessions, "q", openapi3.ParameterInQuery, false)
				assertParameter(t, listSessions, "sort", openapi3.ParameterInQuery, false)
				assertParameter(t, listSessions, "cursor", openapi3.ParameterInQuery, false)
				assertParameter(t, listSessions, "limit", openapi3.ParameterInQuery, false)
				assertResponseStatus(t, listSessions, 400)
				assertResponseStatus(t, listSessions, 503)
				assertEnumValues(
					t,
					propertySchema(t, sessionSchema, "type"),
					"user",
					"system",
					"coordinator",
					"spawned",
				)
				assertEnumValues(
					t,
					propertySchema(t, sessionSchema, "state"),
					"starting",
					"active",
					"stopping",
					"stopped",
				)
				assertEnumValues(t, propertySchema(t, sessionSchema, "stop_reason"),
					"completed",
					"owner_released",
					"user_canceled",
					"max_iterations",
					"loop_detected",
					"timeout",
					"budget_exceeded",
					"error",
					"agent_crashed",
					"hook_stopped",
					"shutdown",
				)
			},
		},
		{
			name: "ShouldDescribeSessionFileSearchLimitAndMentionKinds",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				search := operationFor(t, doc, "/api/workspaces/{workspace_id}/sessions/{session_id}/files", "GET")
				// The handler reads the profile selectors; generated clients must be able to send them.
				assertParameter(t, search, "profile", openapi3.ParameterInQuery, false)
				assertParameter(t, search, "all_profiles", openapi3.ParameterInQuery, false)
				limit := parameterSchema(t, search, "limit", openapi3.ParameterInQuery)
				if limit.Min == nil || *limit.Min != 1 ||
					limit.Max == nil || *limit.Max != float64(contract.SessionFilesMaxLimit) ||
					limit.Default != contract.SessionFilesDefaultLimit {
					t.Fatalf(
						"limit schema min=%v max=%v default=%v, want 1/200/50",
						limit.Min,
						limit.Max,
						limit.Default,
					)
				}
				mention := componentSchema(t, doc, "SessionInputMentionPayload")
				if got := propertySchema(t, mention, "kind").Enum; !slices.Equal(
					got, []any{"file", "folder", "session"},
				) {
					t.Fatalf("mention kind enum = %v, want file|folder|session", got)
				}
			},
		},
		{
			name: "ShouldDescribeSessionAttachAndRecapContracts",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				attachSession := operationFor(
					t,
					doc,
					"/api/workspaces/{workspace_id}/sessions/{session_id}/attach",
					"POST",
				)
				attachRequest := jsonRequestSchema(t, attachSession)
				assertNotRequired(t, attachRequest, "attached_to", "ttl_seconds")
				ttlSeconds := propertySchema(t, attachRequest, "ttl_seconds")
				if ttlSeconds.Max == nil ||
					*ttlSeconds.Max != float64(contract.SessionAttachMaxTTLSeconds) {
					t.Fatalf(
						"attach ttl_seconds maximum = %v, want %d",
						ttlSeconds.Max,
						contract.SessionAttachMaxTTLSeconds,
					)
				}
				attachResponse := jsonResponseSchema(t, attachSession, 200)
				assertRequired(t, attachResponse, "session", "attach")
				assertRequired(
					t,
					propertySchema(t, attachResponse, "attach"),
					"session_id",
					"attached_to",
					"attach_expires_at",
					"attached_at",
				)
				attachError := jsonResponseSchema(t, attachSession, 400)
				assertRequired(t, attachError, "error")
				assertResponseStatus(t, attachSession, 409)
				assertResponseStatus(t, attachSession, 503)

				recapSession := operationFor(
					t,
					doc,
					"/api/workspaces/{workspace_id}/sessions/{session_id}/recap",
					"GET",
				)
				assertParameter(t, recapSession, "limit", openapi3.ParameterInQuery, false)
				recapResponse := jsonResponseSchema(t, recapSession, 200)
				assertRequired(t, recapResponse, "recap")
				recapSchema := propertySchema(t, recapResponse, "recap")
				assertRequired(
					t,
					recapSchema,
					"session",
					"recent_markers",
					"recent_messages",
					"pending_inputs",
					"pending_markers",
					"snapshot",
				)
				snapshotSchema := propertySchema(t, recapSchema, "snapshot")
				assertRequired(
					t,
					snapshotSchema,
					"generated_at",
					"event_cursor",
					"transcript_cursor",
					"queue_generation",
					"consistency",
				)
			},
		},
		{
			name: "ShouldDescribeEveryNamedSessionCatalogStreamPayload",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				operation := operationFor(t, doc, "/api/sessions/catalog-stream", "GET")
				stream := responseSchema(t, operation, 200, specContentTypeEventStream)
				if len(stream.OneOf) != 3 {
					t.Fatalf("session catalog SSE response oneOf = %#v, want three named payloads", stream.OneOf)
				}
				catalog := stream.OneOf[0].Value
				attention := stream.OneOf[1].Value
				notification := stream.OneOf[2].Value
				assertRequired(t, catalog, "kind", "workspace_id", "session_id")
				assertRequired(t, attention, "session_id", "workspace_id", "from", "to", "class", "at")
				assertRequired(
					t,
					notification,
					"notification_id",
					"session_id",
					"workspace_id",
					"title",
					"body",
					"at",
				)
			},
		},
		{
			name: "ShouldDescribeSessionTranscriptStreamAndDirectReadContracts",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				directSession := operationFor(t, doc, "/api/sessions/{session_id}", "GET")
				assertParameter(t, directSession, "session_id", openapi3.ParameterInPath, true)
				assertParameter(t, directSession, "include_health", openapi3.ParameterInQuery, false)
				directResponse := jsonResponseSchema(t, directSession, 200)
				assertRequired(t, directResponse, "session")

				transcriptOperation := operationFor(
					t,
					doc,
					"/api/workspaces/{workspace_id}/sessions/{session_id}/transcript",
					"GET",
				)
				assertParameter(t, transcriptOperation, "limit", openapi3.ParameterInQuery, false)
				assertParameter(t, transcriptOperation, "before_sequence", openapi3.ParameterInQuery, false)
				assertParameterAbsent(t, transcriptOperation, "after_sequence", openapi3.ParameterInQuery)
				transcriptResponse := jsonResponseSchema(t, transcriptOperation, 200)
				assertRequired(
					t,
					transcriptResponse,
					"entries",
					"epoch",
					"generation",
					"max_sequence",
					"has_older",
					"limit",
				)
				assertPropertyAbsent(t, transcriptResponse, "messages")
				entriesSchema := propertySchema(t, transcriptResponse, "entries")
				if entriesSchema.Items == nil || entriesSchema.Items.Value == nil {
					t.Fatal("expected transcript entries to define an items schema")
				}
				assertRequired(t, entriesSchema.Items.Value, "message", "start_sequence", "sequence")
				assertResponseStatus(t, transcriptOperation, 400)
				assertResponseStatus(t, transcriptOperation, 503)

				streamOperation := operationFor(
					t,
					doc,
					"/api/workspaces/{workspace_id}/sessions/{session_id}/stream",
					"GET",
				)
				assertParameter(t, streamOperation, "Last-Event-ID", openapi3.ParameterInHeader, false)
				assertParameter(t, streamOperation, "after_sequence", openapi3.ParameterInQuery, false)
				assertParameter(t, streamOperation, "epoch", openapi3.ParameterInQuery, false)
				assertParameter(t, streamOperation, "generation", openapi3.ParameterInQuery, false)
				assertParameter(t, streamOperation, "limit", openapi3.ParameterInQuery, false)
				assertEnumValues(
					t,
					parameterSchema(t, streamOperation, "frames", openapi3.ParameterInQuery),
					contract.SessionStreamFrameRaw,
					contract.SessionStreamFrameTranscript,
				)
				assertParameterAbsent(t, streamOperation, "replay", openapi3.ParameterInQuery)
				streamSchema := responseSchema(t, streamOperation, 200, specContentTypeEventStream)
				streamVariantSchema(t, streamSchema, "SessionEventPayload")
				snapshotSchema := streamVariantSchema(t, streamSchema, "TranscriptSnapshotPayload")
				assertRequired(
					t,
					snapshotSchema,
					"session_id",
					"epoch",
					"generation",
					"entries",
					"max_sequence",
					"has_older",
					"reset",
				)
				deltaSchema := streamVariantSchema(t, streamSchema, "TranscriptDeltaPayload")
				assertRequired(
					t,
					deltaSchema,
					"session_id",
					"epoch",
					"generation",
					"entries",
					"cursor",
					"max_sequence",
					"has_more",
				)
				streamVariantSchema(t, streamSchema, "SessionEventPayload")
				assertResponseStatus(t, streamOperation, 503)

				eventsOperation := operationFor(
					t,
					doc,
					"/api/workspaces/{workspace_id}/sessions/{session_id}/events",
					"GET",
				)
				assertParameter(t, eventsOperation, "limit", openapi3.ParameterInQuery, false)
				historyOperation := operationFor(
					t,
					doc,
					"/api/workspaces/{workspace_id}/sessions/{session_id}/history",
					"GET",
				)
				assertParameter(t, historyOperation, "limit", openapi3.ParameterInQuery, false)
			},
		},

		{
			name: "ShouldDescribeCreateSessionOptionalFields",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				createSession := operationFor(t, doc, "/api/sessions", "POST")
				createSessionSchema := jsonRequestSchema(t, createSession)
				assertNotRequired(
					t,
					createSessionSchema,
					"agent_name",
					"name",
					"workspace",
					"workspace_path",
				)
				assertPropertyAbsent(t, createSessionSchema, "provider")
				assertPropertyAbsent(t, createSessionSchema, "model")
				assertPropertyAbsent(t, createSessionSchema, "reasoning_effort")
				assertPropertyAbsent(t, createSessionSchema, "speed")
				assertPropertyAbsent(t, createSessionSchema, "runtime")
			},
		},
		{
			name: "ShouldDescribeConfirmedWorktreeForkContract",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				fork := operationFor(
					t,
					doc,
					"/api/workspaces/{workspace_id}/sessions/{session_id}/worktree-fork",
					"POST",
				)
				request := jsonRequestSchema(t, fork)
				assertRequired(t, request, "confirmed")
				assertNotRequired(t, request, "worktree", "new_worktree")
				assertSchemaHasAdditionalProperties(t, request, false)
				assertResponseStatus(t, fork, 201)
				assertResponseStatus(t, fork, 409)
			},
		},
		{
			name: "ShouldDescribeCreateAgentContract",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				createAgent := operationFor(t, doc, "/api/agents", "POST")
				assertTagsContain(t, createAgent, "agents")
				assertResponseStatus(t, createAgent, 400)
				assertResponseStatus(t, createAgent, 409)
				assertResponseStatus(t, createAgent, 503)

				requestSchema := jsonRequestSchema(t, createAgent)
				assertRequired(t, requestSchema, "scope", "agent")
				assertEnumValues(t, propertySchema(t, requestSchema, "scope"), "workspace", "global")

				agentSchema := propertySchema(t, requestSchema, "agent")
				assertRequired(t, agentSchema, "name", "prompt")
				assertNotRequired(
					t,
					agentSchema,
					"command",
					"provider",
					"model",
					"reasoning_effort",
					"tools",
					"toolsets",
					"deny_tools",
					"permissions",
					"category_path",
					"skills",
				)
				assertEnumValues(
					t,
					propertySchema(t, agentSchema, "permissions"),
					"deny-all",
					"approve-reads",
					"approve-all",
				)
				assertPropertyAbsent(t, agentSchema, "mcp_servers")
				assertPropertyAbsent(t, agentSchema, "hooks")

				responseSchema := jsonResponseSchema(t, createAgent, 201)
				assertRequired(t, responseSchema, "agent")
			},
		},
		{
			name: "ShouldDescribeAgentDefinitionMutationContracts",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				for _, operationSpec := range agentDefinitionMutationOperations() {
					operation := operationFor(t, doc, operationSpec.Path, operationSpec.Method)
					assertParameter(t, operation, specProfileKey, openapi3.ParameterInQuery, false)
					assertParameterAbsent(t, operation, "all_profiles", openapi3.ParameterInQuery)
				}
				assertParameter(
					t,
					operationFor(t, doc, "/api/agents", "POST"),
					specProfileKey,
					openapi3.ParameterInQuery,
					false,
				)

				update := operationFor(t, doc, "/api/agents/{name}", "PUT")
				assertParameter(t, update, "name", openapi3.ParameterInPath, true)
				assertResponseStatus(t, update, 409)
				updateSchema := jsonRequestSchema(t, update)
				assertRequired(t, updateSchema, "agent", "expected_digest")
				updateAgentSchema := propertySchema(t, updateSchema, "agent")
				assertPropertyAbsent(t, updateAgentSchema, "mcp_servers")

				deleteAgent := operationFor(t, doc, "/api/agents/{name}", "DELETE")
				assertResponseStatus(t, deleteAgent, 403)
				assertParameter(t, deleteAgent, "workspace", openapi3.ParameterInQuery, false)
				deleteSchema := jsonResponseSchema(t, deleteAgent, 200)
				assertRequired(t, deleteSchema, "name", "origin")
				assertNotRequired(t, deleteSchema, "unshadowed_origin")

				duplicate := operationFor(t, doc, "/api/agents/{name}/duplicate", "POST")
				assertResponseStatus(t, duplicate, 201)
				assertResponseStatus(t, duplicate, 409)
				duplicateSchema := jsonRequestSchema(t, duplicate)
				assertRequired(t, duplicateSchema, "name")
				assertNotRequired(t, duplicateSchema, "scope", "workspace", "overrides")
				assertEnumValues(t, propertySchema(t, duplicateSchema, "scope"), "workspace", "global")

				for _, path := range []string{"/api/agents", "/api/agents/catalog", "/api/agents/{name}"} {
					read := operationFor(t, doc, path, "GET")
					assertParameter(t, read, specProfileKey, openapi3.ParameterInQuery, false)
					assertParameterAbsent(t, read, "all_profiles", openapi3.ParameterInQuery)
				}

				getAgent := operationFor(t, doc, "/api/agents/{name}", "GET")
				getSchema := jsonResponseSchema(t, getAgent, 200)
				agentSchema := propertySchema(t, getSchema, "agent")
				assertRequired(
					t,
					agentSchema,
					"name",
					"provider",
					"origin",
					"definition_digest",
					"prompt",
				)
				assertEnumValues(t, propertySchema(t, agentSchema, "origin"), "global", "workspace")
			},
		},
		{
			name: "ShouldDescribeProviderModelCatalogAndOpenAIProjection",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				listModels := operationFor(t, doc, "/api/model-catalog/providers/{provider_id}/models", "GET")
				assertTagsContain(t, listModels, "providers")
				assertParameter(t, listModels, "provider_id", openapi3.ParameterInPath, true)
				assertParameter(t, listModels, "view", openapi3.ParameterInQuery, false)
				assertParameter(t, listModels, "source_id", openapi3.ParameterInQuery, false)
				assertParameter(t, listModels, "refresh", openapi3.ParameterInQuery, false)
				assertParameter(t, listModels, "include_stale", openapi3.ParameterInQuery, false)
				listSchema := jsonResponseSchema(t, listModels, 200)
				assertRequired(t, listSchema, "models")

				refresh := operationFor(t, doc, "/api/model-catalog/providers/{provider_id}/models/refresh", "POST")
				if refresh.RequestBody == nil || refresh.RequestBody.Value == nil ||
					refresh.RequestBody.Value.Required {
					t.Fatalf("refresh request body required = %#v, want optional body", refresh.RequestBody)
				}
				assertResponseStatus(t, refresh, 503)
				for _, path := range []string{
					"/api/model-catalog/models/refresh",
					"/api/model-catalog/providers/{provider_id}/models/refresh",
				} {
					for _, status := range []int{500, 503} {
						schema := jsonResponseSchema(t, operationFor(t, doc, path, "POST"), status)
						for _, body := range []map[string]any{
							{"error": "catalog unavailable"},
							{
								"error": "catalog refresh failed",
								"sources": []any{map[string]any{
									"source_id": "provider_live:cursor", "source_kind": "provider_live",
									"provider_id": "cursor", "priority": float64(110),
									"refresh_state": "failed", "row_count": float64(0), "stale": false,
									"failure_reason": "auth_required",
								}},
							},
						} {
							if err := schema.VisitJSON(body); err != nil {
								t.Fatalf("%s status %d rejects refresh error payload: %v", path, status, err)
							}
						}
					}
				}

				curate := operationFor(
					t,
					doc,
					"/api/model-catalog/providers/{provider_id}/models/curate",
					"POST",
				)
				assertParameter(t, curate, "provider_id", openapi3.ParameterInPath, true)
				assertRequired(t, jsonRequestSchema(t, curate), "model_id")
				effortSchema := propertySchema(t, jsonRequestSchema(t, curate), "default_effort")
				assertSchemaIncludesType(t, effortSchema, openapi3.TypeString)
				if err := effortSchema.VisitJSON("provider-next"); err != nil {
					t.Fatalf("provider effort rejected by wire schema: %v", err)
				}

				assertResponseStatus(t, curate, 422)
				assertResponseStatus(t, curate, 503)

				status := operationFor(t, doc, "/api/model-catalog/sources/status", "GET")
				statusSchema := jsonResponseSchema(t, status, 200)
				assertRequired(t, statusSchema, "sources")

				openAI := operationFor(t, doc, "/api/openai/v1/models", "GET")
				assertTagsContain(t, openAI, "openai")
				assertParameter(t, openAI, "provider_id", openapi3.ParameterInQuery, false)
				openAISchema := jsonResponseSchema(t, openAI, 200)
				assertRequired(t, openAISchema, "object", "data")
				assertResponseStatus(t, openAI, 403)
				assertResponseStatus(t, openAI, 503)
				openAIForbidden := jsonResponseSchema(t, openAI, 403)
				assertRequired(t, openAIForbidden, "error")
				assertRequired(t, propertySchema(t, openAIForbidden, "error"), "code", "message")
				openAIUnavailable := jsonResponseSchema(t, openAI, 503)
				assertRequired(t, openAIUnavailable, "error")
				assertRequired(t, propertySchema(t, openAIUnavailable, "error"), "code", "message")
			},
		},
		{
			name: "ShouldDescribeApproveSessionRequiredFields",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				approveSession := operationFor(
					t,
					doc,
					"/api/workspaces/{workspace_id}/sessions/{session_id}/approve",
					"POST",
				)
				approveSchema := jsonRequestSchema(t, approveSession)
				assertRequired(t, approveSchema, "request_id", "turn_id", "decision")
			},
		},
		{
			name: "ShouldDescribeClarificationAnswerWithoutCallerOwnedScope",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				answer := operationFor(
					t,
					doc,
					"/api/workspaces/{workspace_id}/sessions/{session_id}/clarifications/{request_id}/answer",
					"POST",
				)
				request := jsonRequestSchema(t, answer)
				assertNotRequired(t, request, "workspace_id", "agent_name", "fallback")
				response := jsonResponseSchema(t, answer, 200)
				assertRequired(t, response, "choice", "text", "fallback")
				assertResponseStatus(t, answer, 400)
				assertResponseStatus(t, answer, 404)
				assertResponseStatus(t, answer, 409)
			},
		},
		{
			name: "ShouldDescribeAutomationJobSchemasAndEnums",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				for _, operation := range []*openapi3.Operation{
					operationFor(t, doc, "/api/automation/jobs", "GET"),
					operationFor(t, doc, "/api/automation/triggers", "GET"),
				} {
					assertParameter(t, operation, "q", openapi3.ParameterInQuery, false)
					assertParameter(t, operation, "cursor", openapi3.ParameterInQuery, false)
					assertParameter(t, operation, "enabled", openapi3.ParameterInQuery, false)
					assertEnumValues(
						t,
						parameterSchema(t, operation, "source", openapi3.ParameterInQuery),
						"config",
						"package",
						"dynamic",
					)
					pageSchema := propertySchema(t, jsonResponseSchema(t, operation, 200), "page")
					assertRequired(t, pageSchema, "has_more", "total", "limit")
					assertNotRequired(t, pageSchema, "next_cursor")
				}

				createJob := operationFor(t, doc, "/api/automation/jobs", "POST")
				createJobSchema := jsonRequestSchema(t, createJob)
				assertRequired(t, createJobSchema, "scope", "name", "agent_name", "prompt", "schedule")
				assertNotRequired(t, createJobSchema, "workspace_id", "enabled", "retry", "fire_limit")
				assertEnumValues(
					t,
					propertySchema(t, createJobSchema, "scope"),
					"global",
					"workspace",
					"user",
					workspaceaccess.UserSelector,
				)

				scheduleSchema := propertySchema(t, createJobSchema, "schedule")
				assertRequired(t, scheduleSchema, "mode")
				assertEnumValues(t, propertySchema(t, scheduleSchema, "mode"), "at", "cron", "every")

				retrySchema := propertySchema(t, createJobSchema, "retry")
				assertRequired(t, retrySchema, "strategy", "max_retries", "base_delay")
				assertEnumValues(t, propertySchema(t, retrySchema, "strategy"), "backoff", "none")
			},
		},
		{
			name: "ShouldDescribeAutomationTriggerAndHealthSchemas",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				createTrigger := operationFor(t, doc, "/api/automation/triggers", "POST")
				createTriggerSchema := jsonRequestSchema(t, createTrigger)
				assertRequired(t, createTriggerSchema, "scope", "name", "agent_name", "prompt", "event")
				assertNotRequired(
					t,
					createTriggerSchema,
					"workspace_id",
					"filter",
					"enabled",
					"retry",
					"fire_limit",
					"webhook_id",
					"endpoint_slug",
					"webhook_secret_value",
				)
				assertEnumValues(
					t,
					propertySchema(t, createTriggerSchema, "scope"),
					"global",
					"workspace",
					"user",
					workspaceaccess.UserSelector,
				)

				healthOperation := operationFor(t, doc, "/api/status", "GET")
				healthSchema := jsonResponseSchema(t, healthOperation, 200)
				assertRequired(t, healthSchema, "daemon", "health", "automation", "config", "log_tail")

				automationSchema := propertySchema(t, healthSchema, "automation")
				assertRequired(t, automationSchema, "enabled", "jobs", "triggers", "scheduler_running")
				assertNotRequired(t, automationSchema, "next_fire")
			},
		},
		{
			name: "ShouldDescribeConsentFirstAutomationSuggestionOperations",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				list := operationFor(
					t,
					doc,
					"/api/workspaces/{workspace_id}/automation/suggestions",
					"GET",
				)
				if list.OperationID != "listAutomationSuggestions" {
					t.Fatalf("list operation id = %q, want listAutomationSuggestions", list.OperationID)
				}
				assertParameter(t, list, "workspace_id", openapi3.ParameterInPath, true)
				assertEnumValues(
					t,
					parameterSchema(t, list, "status", openapi3.ParameterInQuery),
					"accepted",
					"dismissed",
					"pending",
				)
				listSchema := jsonResponseSchema(t, list, 200)
				assertRequired(t, listSchema, "suggestions")

				accept := operationFor(
					t,
					doc,
					"/api/workspaces/{workspace_id}/automation/suggestions/{suggestion_id}/accept",
					"POST",
				)
				if accept.OperationID != "acceptAutomationSuggestion" {
					t.Fatalf("accept operation id = %q, want acceptAutomationSuggestion", accept.OperationID)
				}
				assertParameter(t, accept, "workspace_id", openapi3.ParameterInPath, true)
				assertParameter(t, accept, "suggestion_id", openapi3.ParameterInPath, true)
				assertRequired(t, jsonResponseSchema(t, accept, 200), "suggestion", "job")

				dismiss := operationFor(
					t,
					doc,
					"/api/workspaces/{workspace_id}/automation/suggestions/{suggestion_id}/dismiss",
					"POST",
				)
				if dismiss.OperationID != "dismissAutomationSuggestion" {
					t.Fatalf("dismiss operation id = %q, want dismissAutomationSuggestion", dismiss.OperationID)
				}
				assertParameter(t, dismiss, "workspace_id", openapi3.ParameterInPath, true)
				assertParameter(t, dismiss, "suggestion_id", openapi3.ParameterInPath, true)
				assertRequired(t, jsonResponseSchema(t, dismiss, 200), "suggestion")
			},
		},
		{
			name: "ShouldDescribeWebhookHeadersAndAutomationRunEnums",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				webhookOperation := operationFor(t, doc, "/api/webhooks/global/{endpoint}", "POST")
				assertParameter(t, webhookOperation, "endpoint", openapi3.ParameterInPath, true)
				assertParameter(t, webhookOperation, "X-Compozy-Webhook-Timestamp", openapi3.ParameterInHeader, true)
				assertParameter(t, webhookOperation, "X-Compozy-Webhook-Signature", openapi3.ParameterInHeader, true)

				runOperation := operationFor(t, doc, "/api/automation/runs/{id}", "GET")
				runSchema := jsonResponseSchema(t, runOperation, 200)
				runPayloadSchema := propertySchema(t, runSchema, "run")
				assertEnumValues(
					t,
					propertySchema(t, runPayloadSchema, "status"),
					"canceled",
					"completed",
					"delegated",
					"failed",
					"running",
					"scheduled",
				)
			},
		},
		{
			name: "ShouldDescribeSkillOperationsAgentAwareQueryParams",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				listSkills := operationFor(t, doc, "/api/skills", "GET")
				assertParameter(t, listSkills, "workspace", openapi3.ParameterInQuery, false)
				assertParameter(t, listSkills, "for_agent", openapi3.ParameterInQuery, false)
				listSkillsSchema := jsonResponseSchema(t, listSkills, 200)
				skillSchema := propertySchema(t, listSkillsSchema, "skills").Items.Value
				assertRequired(t, skillSchema, "activation", "origin")
				activationSchema := propertySchema(t, skillSchema, "activation")
				assertRequired(t, activationSchema, "active")
				activationReasonSchema := propertySchema(t, activationSchema, "reasons").Items.Value
				assertRequired(t, activationReasonSchema, "gate", "code", "message")
				assertEnumValues(
					t,
					propertySchema(t, activationReasonSchema, "code"),
					"capability_context_unavailable",
					"environment_context_unavailable",
					"environment_mismatch",
					"missing_capability",
					"missing_tool",
					"platform_mismatch",
					"tool_context_unavailable",
				)
				provenanceSchema := propertySchema(t, skillSchema, "provenance")
				assertRequired(t, provenanceSchema, "precedence_tier")
				shadowEntrySchema := propertySchema(t, provenanceSchema, "shadowed_by").Items.Value
				assertRequired(t, shadowEntrySchema, "path", "tier", "resolved_to_winner", "detected_at")
				skillDiagnosticsSchema := propertySchema(t, skillSchema, "diagnostics").Items.Value
				assertRequired(t, skillDiagnosticsSchema, "name", "state", "verification_status")
				assertEnumValues(
					t,
					propertySchema(t, skillDiagnosticsSchema, "state"),
					"inactive",
					"shadowed",
					"valid",
					"verification_failed",
				)
				assertEnumValues(
					t,
					propertySchema(t, skillDiagnosticsSchema, "verification_status"),
					"failed",
					"passed",
					"warning",
				)
				failureSchema := propertySchema(t, skillDiagnosticsSchema, "failure")
				assertRequired(t, failureSchema, "code", "message")
				assertNotRequired(t, failureSchema, "expected_hash", "actual_hash")

				getSkill := operationFor(t, doc, "/api/skills/{name}", "GET")
				assertParameter(t, getSkill, "workspace_id", openapi3.ParameterInQuery, false)
				assertParameter(t, getSkill, "for_agent", openapi3.ParameterInQuery, false)
				getSkillSchema := propertySchema(t, jsonResponseSchema(t, getSkill, 200), "skill")
				assertRequired(t, getSkillSchema, "origin")
				exposureSchema := propertySchema(t, getSkillSchema, "exposures").Items.Value
				assertRequired(t, exposureSchema, "target", "path", "status")
				assertEnumValues(t, propertySchema(t, exposureSchema, "status"),
					"broken", "foreign_conflict", "healthy", "missing")

				exposeSkill := operationFor(t, doc, "/api/skills/{name}/expose", "POST")
				assertParameter(t, exposeSkill, "profile", openapi3.ParameterInQuery, false)
				assertParameter(t, exposeSkill, "for_agent", openapi3.ParameterInQuery, false)
				exposeRequestSchema := jsonRequestSchema(t, exposeSkill)
				assertRequired(t, exposeRequestSchema, "targets")
				assertNotRequired(t, exposeRequestSchema, "workspace_id")
				exposeSuccessSchema := jsonResponseSchema(t, exposeSkill, 200)
				assertRequired(t, exposeSuccessSchema, "name", "results", "rolled_back")
				assertNotRequired(t, exposeSuccessSchema, "workspace_id")
				exposeFailureSchema := jsonResponseSchema(t, exposeSkill, 409)
				assertRequired(t, exposeFailureSchema, "error", "name", "results")
				assertNotRequired(t, exposeFailureSchema, "workspace_id", "rolled_back")
				failureErrorSchema := propertySchema(t, exposeFailureSchema, "error")
				assertRequired(t, failureErrorSchema, "code", "message")

				unexposeSkill := operationFor(t, doc, "/api/skills/{name}/unexpose", "POST")
				unexposeSuccessSchema := jsonResponseSchema(t, unexposeSkill, 200)
				assertRequired(t, unexposeSuccessSchema, "name", "results")
				if _, hasRollback := unexposeSuccessSchema.Properties["rolled_back"]; hasRollback {
					t.Fatal("unexpose success schema unexpectedly exposes rolled_back")
				}

				getSkillContent := operationFor(t, doc, "/api/skills/{name}/content", "GET")
				assertParameter(t, getSkillContent, "workspace", openapi3.ParameterInQuery, false)
				assertParameter(t, getSkillContent, "for_agent", openapi3.ParameterInQuery, false)

				getSkillShadows := operationFor(t, doc, "/api/skills/{name}/shadows", "GET")
				assertParameter(t, getSkillShadows, "workspace", openapi3.ParameterInQuery, false)
				assertParameter(t, getSkillShadows, "for_agent", openapi3.ParameterInQuery, false)
				shadowsSchema := jsonResponseSchema(t, getSkillShadows, 200)
				assertRequired(t, shadowsSchema, "name", "winner", "shadows")
				winnerSchema := propertySchema(t, shadowsSchema, "winner")
				assertRequired(t, winnerSchema, "path", "tier", "resolved_to_winner", "detected_at")

				enableSkill := operationFor(t, doc, "/api/skills/{name}/enable", "POST")
				assertParameter(t, enableSkill, "workspace", openapi3.ParameterInQuery, false)
				assertParameter(t, enableSkill, "for_agent", openapi3.ParameterInQuery, false)

				disableSkill := operationFor(t, doc, "/api/skills/{name}/disable", "POST")
				assertParameter(t, disableSkill, "workspace", openapi3.ParameterInQuery, false)
				assertParameter(t, disableSkill, "for_agent", openapi3.ParameterInQuery, false)
			},
		},

		{
			name: "ShouldDescribeToolRegistryContractsAndRoutes",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				listTools := operationFor(t, doc, "/api/tools", "GET")
				assertTagsContain(t, listTools, "tools")
				assertParameter(t, listTools, "workspace_id", openapi3.ParameterInQuery, false)
				profileScopedOperations := []struct {
					path   string
					method string
				}{
					{path: "/api/tools", method: "GET"},
					{path: "/api/tools/search", method: "POST"},
					{path: "/api/tools/{id}", method: "GET"},
					{path: "/api/tools/{id}/approvals", method: "POST"},
					{path: "/api/tools/{id}/invoke", method: "POST"},
					{path: "/api/toolsets", method: "GET"},
					{path: "/api/toolsets/{id}", method: "GET"},
				}
				for _, item := range profileScopedOperations {
					operation := operationFor(t, doc, item.path, item.method)
					assertParameter(t, operation, "profile", openapi3.ParameterInQuery, false)
					assertResponseStatus(t, operation, http.StatusBadRequest)
					assertResponseStatus(t, operation, http.StatusNotFound)
					assertResponseStatus(t, operation, http.StatusConflict)
				}
				if schema := jsonResponseSchema(
					t,
					operationFor(t, doc, "/api/tools/{id}/invoke", "POST"),
					400,
				); len(
					schema.OneOf,
				) != 2 {
					t.Fatalf("profile-scoped tool error variants = %d, want 2", len(schema.OneOf))
				}
				if schema := jsonResponseSchema(
					t,
					operationFor(t, doc, "/api/tools/{id}/invoke", "POST"),
					http.StatusConflict,
				); len(
					schema.OneOf,
				) != 2 {
					t.Fatalf("profile-scoped tool conflict variants = %d, want 2", len(schema.OneOf))
				}
				listSchema := jsonResponseSchema(t, listTools, 200)
				assertRequired(t, listSchema, "tools")
				toolsSchema := propertySchema(t, listSchema, "tools")
				if toolsSchema.Items == nil || toolsSchema.Items.Value == nil {
					t.Fatal("expected tools to define an items schema")
				}
				toolSchema := toolsSchema.Items.Value
				assertRequired(t, toolSchema, "descriptor", "availability", "decision")
				descriptorSchema := propertySchema(t, toolSchema, "descriptor")
				assertRequired(
					t,
					descriptorSchema,
					"tool_id",
					"backend",
					"description",
					"input_schema",
					"source",
					"visibility",
					"risk",
					"read_only",
					"destructive",
					"open_world",
					"requires_interaction",
					"concurrency_safe",
				)
				searchTools := operationFor(t, doc, "/api/tools/search", "POST")
				assertTagsContain(t, searchTools, "tools")
				searchRequest := jsonRequestSchema(t, searchTools)
				assertRequired(t, searchRequest, "query")
				assertNotRequired(t, searchRequest, "limit", "workspace_id", "session_id", "agent_name")
				searchSchema := jsonResponseSchema(t, searchTools, 200)
				assertRequired(t, searchSchema, "tools")
				assertEnumValues(
					t,
					propertySchema(t, propertySchema(t, descriptorSchema, "backend"), "kind"),
					"extension_host",
					"mcp",
					"native_go",
				)
				assertEnumValues(
					t,
					propertySchema(t, descriptorSchema, "visibility"),
					"internal",
					"model",
					"operator",
					"session",
				)
				assertEnumValues(
					t,
					propertySchema(t, descriptorSchema, "risk"),
					"destructive",
					"mutating",
					"open_world",
					"read",
				)

				invoke := operationFor(t, doc, "/api/tools/{id}/invoke", "POST")
				assertResponseStatus(t, invoke, 202)
				assertResponseStatus(t, invoke, 507)
				invokeRequest := jsonRequestSchema(t, invoke)
				assertRequired(t, invokeRequest, "input")
				assertNotRequired(t, invokeRequest, "approval_token", "session_id", "workspace_id")
				errorSchema := jsonResponseSchema(t, invoke, 202)
				errorPayload := propertySchema(t, errorSchema, "error")
				assertRequired(t, errorPayload, "code", "message")
				assertEnumValues(
					t,
					propertySchema(t, errorPayload, "code"),
					toolErrorCodeValues()...,
				)
				persistenceError := jsonResponseSchema(t, invoke, 507)
				persistencePayload := propertySchema(t, persistenceError, "error")
				assertRequired(t, persistencePayload, "code", "message")
				assertNotRequired(t, persistencePayload, "partial_result")

				approval := operationFor(t, doc, "/api/tools/{id}/approvals", "POST")
				approvalSchema := jsonResponseSchema(t, approval, 201)
				assertRequired(
					t,
					propertySchema(t, approvalSchema, "approval"),
					"approval_token",
					"expires_at",
					"tool_id",
					"input_digest",
				)

				sessionTools := operationFor(
					t,
					doc,
					"/api/workspaces/{workspace_id}/sessions/{session_id}/tools",
					"GET",
				)
				assertTagsContain(t, sessionTools, "sessions", "tools")
				assertParameter(t, sessionTools, "workspace_id", openapi3.ParameterInPath, true)
				assertParameter(t, sessionTools, "session_id", openapi3.ParameterInPath, true)
				assertResponseStatus(t, sessionTools, 200)
				sessionSearch := operationFor(
					t,
					doc,
					"/api/workspaces/{workspace_id}/sessions/{session_id}/tools/search",
					"POST",
				)
				assertTagsContain(t, sessionSearch, "sessions", "tools")
				assertParameter(t, sessionSearch, "workspace_id", openapi3.ParameterInPath, true)
				assertParameter(t, sessionSearch, "session_id", openapi3.ParameterInPath, true)
				sessionSearchRequest := jsonRequestSchema(t, sessionSearch)
				assertRequired(t, sessionSearchRequest, "query")
				assertNotRequired(t, sessionSearchRequest, "limit", "workspace_id", "session_id", "agent_name")
				sessionSearchSchema := jsonResponseSchema(t, sessionSearch, 200)
				assertRequired(t, sessionSearchSchema, "tools")

				artifactRead := operationFor(
					t,
					doc,
					"/api/workspaces/{workspace_id}/tool-artifacts/{artifact_id}",
					"GET",
				)
				assertTagsContain(t, artifactRead, "tools")
				assertParameter(t, artifactRead, "workspace_id", openapi3.ParameterInPath, true)
				assertParameter(t, artifactRead, "artifact_id", openapi3.ParameterInPath, true)
				assertParameter(t, artifactRead, "offset", openapi3.ParameterInQuery, false)
				assertParameter(t, artifactRead, "limit", openapi3.ParameterInQuery, false)
				artifactPage := jsonResponseSchema(t, artifactRead, 200)
				assertRequired(
					t,
					artifactPage,
					"artifact",
					"offset",
					"bytes",
					"total_bytes",
					"data_base64",
					"next_offset",
					"eof",
				)
				assertRequired(t, propertySchema(t, artifactPage, "artifact"), "uri")
				assertResponseStatus(t, artifactRead, 400)
				assertResponseStatus(t, artifactRead, 404)
				assertResponseStatus(t, artifactRead, 502)
				assertResponseStatus(t, artifactRead, 503)

				toolsets := operationFor(t, doc, "/api/toolsets", "GET")
				assertTagsContain(t, toolsets, "toolsets")
				toolsetsSchema := jsonResponseSchema(t, toolsets, 200)
				assertRequired(t, toolsetsSchema, "toolsets")
				toolsetsItems := propertySchema(t, toolsetsSchema, "toolsets")
				if toolsetsItems.Items == nil || toolsetsItems.Items.Value == nil {
					t.Fatal("expected toolsets to define an items schema")
				}
				toolsetSchema := toolsetsItems.Items.Value
				assertRequired(t, toolsetSchema, "id", "status")
				assertNotRequired(t, toolsetSchema, "reason_codes", "expanded_tools")
				toolset := operationFor(t, doc, "/api/toolsets/{id}", "GET")
				assertTagsContain(t, toolset, "toolsets")
				assertParameter(t, toolset, "id", openapi3.ParameterInPath, true)
				toolsetResponse := jsonResponseSchema(t, toolset, 200)
				assertRequired(t, propertySchema(t, toolsetResponse, "toolset"), "id", "status")
			},
		},

		{
			name: "ShouldRegisterExpandedTaskAndObserveOperations",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				operations := []struct {
					path   string
					method string
				}{
					{path: "/api/tasks", method: "GET"},
					{path: "/api/tasks", method: "POST"},
					{path: "/api/tasks/{id}", method: "GET"},
					{path: "/api/tasks/{id}", method: "PATCH"},
					{path: "/api/tasks/{id}/blocks", method: "POST"},
					{path: "/api/tasks/{id}/blocks", method: "GET"},
					{path: "/api/tasks/{id}/blocks/{block_id}/clear", method: "POST"},
					{path: "/api/tasks/{id}/recover", method: "POST"},
					{path: "/api/tasks/{id}/execution-profile", method: "GET"},
					{path: "/api/tasks/{id}/execution-profile", method: "PUT"},
					{path: "/api/tasks/{id}/execution-profile/worktree", method: "PATCH"},
					{path: "/api/tasks/{id}/execution-profile", method: "DELETE"},

					{path: "/api/tasks/{id}/reviews", method: "GET"},
					{path: "/api/tasks/{id}/publish", method: "POST"},
					{path: "/api/tasks/{id}/start", method: "POST"},
					{path: "/api/tasks/{id}/cancel", method: "POST"},
					{path: "/api/tasks/{id}/children", method: "POST"},
					{path: "/api/tasks/{id}/dependencies", method: "POST"},
					{path: "/api/tasks/{id}/dependencies/{depends_on_id}", method: "DELETE"},
					{path: "/api/tasks/{id}/runs", method: "GET"},
					{path: "/api/tasks/{id}/runs", method: "POST"},
					{path: "/api/task-runs/{id}", method: "GET"},
					{path: "/api/task-runs/{id}/start", method: "POST"},
					{path: "/api/task-runs/{id}/attach-session", method: "POST"},
					{path: "/api/task-runs/{id}/complete", method: "POST"},
					{path: "/api/task-runs/{id}/fail", method: "POST"},
					{path: "/api/task-runs/{id}/cancel", method: "POST"},
					{path: "/api/task-runs/{id}/reviews", method: "GET"},
					{path: "/api/task-runs/{id}/reviews", method: "POST"},
					{path: "/api/task-reviews/{id}", method: "GET"},
					{path: "/api/task-reviews/{id}/verdict", method: "POST"},
					{path: "/api/tasks/{id}/timeline", method: "GET"},
					{path: "/api/tasks/{id}/stream", method: "GET"},
					{path: "/api/tasks/{id}/tree", method: "GET"},
					{path: "/api/tasks/{id}/approve", method: "POST"},
					{path: "/api/tasks/{id}/reject", method: "POST"},
					{path: "/api/tasks/{id}/triage/read", method: "POST"},
					{path: "/api/tasks/{id}/triage/archive", method: "POST"},
					{path: "/api/tasks/{id}/triage/dismiss", method: "POST"},
					{path: "/api/observe/tasks/dashboard", method: "GET"},
					{path: "/api/observe/tasks/inbox", method: "GET"},
				}

				for _, operation := range operations {
					t.Run(operation.method+" "+operation.path, func(t *testing.T) {
						t.Parallel()
						operationFor(t, doc, operation.path, operation.method)
					})
				}

				setProfile := operationFor(t, doc, "/api/tasks/{id}/execution-profile", "PUT")
				setProfileSchema := jsonRequestSchema(t, setProfile)
				runtimeSchema := propertySchema(t, setProfileSchema, "runtime")
				assertRequired(t, runtimeSchema, "mode")
				assertEnumValues(
					t,
					propertySchema(t, runtimeSchema, "mode"),
					string(taskpkg.RuntimeModeDefault),
					string(taskpkg.RuntimeModeEvidence),
				)
				worktreePatch := operationFor(t, doc, "/api/tasks/{id}/execution-profile/worktree", "PATCH")
				worktreeSchema := jsonRequestSchema(t, worktreePatch)
				assertRequired(t, worktreeSchema, "mode")
				assertEnumValues(
					t,
					propertySchema(t, worktreeSchema, "mode"),
					string(taskpkg.WorktreeModeInherit),
					string(taskpkg.WorktreeModeNone),
					string(taskpkg.WorktreeModeRef),
					string(taskpkg.WorktreeModePerRun),
				)
			},
		},
		{
			name: "ShouldRegisterAgentAutonomyOperationsAndSchemas",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				operations := []struct {
					path   string
					method string
				}{
					{path: "/api/agent/me", method: "GET"},
					{path: "/api/agent/context", method: "GET"},

					{path: "/api/agent/tasks/claim-next", method: "POST"},
					{path: "/api/agent/tasks/{run_id}/start", method: "POST"},
					{path: "/api/agent/tasks/{run_id}/heartbeat", method: "POST"},
					{path: "/api/agent/tasks/{run_id}/complete", method: "POST"},
					{path: "/api/agent/tasks/{run_id}/fail", method: "POST"},
					{path: "/api/agent/tasks/{run_id}/release", method: "POST"},
					{path: "/api/agent/spawn", method: "POST"},
					{path: "/api/agent/coordinator/config", method: "GET"},
				}
				for _, operation := range operations {
					t.Run(operation.method+" "+operation.path, func(t *testing.T) {
						t.Parallel()
						spec := operationFor(t, doc, operation.path, operation.method)
						assertTagsContain(t, spec, "agent")
						assertParameter(
							t,
							spec,
							agentidentity.HeaderSessionID,
							openapi3.ParameterInHeader,
							true,
						)
						assertParameter(
							t,
							spec,
							agentidentity.HeaderAgent,
							openapi3.ParameterInHeader,
							true,
						)
						assertParameterAbsent(
							t,
							spec,
							"X-Compozy-Workspace-ID",
							openapi3.ParameterInHeader,
						)
					})
				}

				contextOperation := operationFor(t, doc, "/api/agent/context", "GET")
				contextSchema := jsonResponseSchema(t, contextOperation, 200)
				assertRequired(t, contextSchema, "context")
				contextPayload := propertySchema(t, contextSchema, "context")
				assertRequired(
					t,
					contextPayload,
					"self",
					"workspace",
					"session",
					"task",
					"capabilities",
					"limits",
					"provenance",
				)

				claimOperation := operationFor(t, doc, "/api/agent/tasks/claim-next", "POST")
				assertTagsContain(t, claimOperation, "tasks")
				claimRequest := jsonRequestSchema(t, claimOperation)
				assertNotRequired(
					t,
					claimRequest,
					"workspace_id",
					"required_capabilities",
					"priority_min",
					"lease_seconds",
					"wait",
				)
				claimResponse := jsonResponseSchema(t, claimOperation, 200)
				claimPayload := propertySchema(t, claimResponse, "claim")
				assertRequired(t, claimPayload, "task", "run", "lease")
				if _, exists := claimPayload.Properties["claim_token"]; exists {
					t.Fatalf("agent claim payload schema exposes raw claim_token")
				}
				leaseSchema := propertySchema(t, claimPayload, "lease")
				assertRequired(t, leaseSchema, "task_id", "run_id", "status")
				assertNotRequired(t, leaseSchema, "claim_token_hash")

				startOperation := operationFor(t, doc, "/api/agent/tasks/{run_id}/start", "POST")
				assertParameter(t, startOperation, "run_id", openapi3.ParameterInPath, true)
				startSchema := jsonRequestSchema(t, startOperation)
				assertNotRequired(t, startSchema, "idempotency_key")
				if _, exists := startSchema.Properties["claim_token"]; exists {
					t.Fatalf("agent start schema exposes raw claim_token")
				}

				heartbeatOperation := operationFor(t, doc, "/api/agent/tasks/{run_id}/heartbeat", "POST")
				assertParameter(t, heartbeatOperation, "run_id", openapi3.ParameterInPath, true)
				heartbeatSchema := jsonRequestSchema(t, heartbeatOperation)
				assertNotRequired(t, heartbeatSchema, "lease_seconds")
				if _, exists := heartbeatSchema.Properties["claim_token"]; exists {
					t.Fatalf("agent heartbeat schema exposes raw claim_token")
				}

				spawnOperation := operationFor(t, doc, "/api/agent/spawn", "POST")
				spawnSchema := jsonRequestSchema(t, spawnOperation)
				assertRequired(
					t,
					spawnSchema,
					"agent_name",
					"spawn_role",
					"ttl_seconds",
					"auto_stop_on_parent",
					"permissions",
				)
				assertEnumValues(t, propertySchema(t, spawnSchema, "speed"), "normal", "fast")
				spawnResponse := jsonResponseSchema(t, spawnOperation, 201)
				lineageSchema := propertySchema(t, propertySchema(t, spawnResponse, "spawn"), "lineage")
				assertRequired(
					t,
					lineageSchema,
					"spawn_depth",
					"auto_stop_on_parent",
					"spawn_budget",
					"permission_policy",
				)

				configOperation := operationFor(t, doc, "/api/agent/coordinator/config", "GET")
				configResponse := jsonResponseSchema(t, configOperation, 200)
				configSchema := propertySchema(t, configResponse, "coordinator")
				assertRequired(
					t,
					configSchema,
					"enabled",
					"agent_name",
					"default_ttl_seconds",
					"max_children",
					"max_active_sessions_per_workspace",
					"source",
				)
				assertEnumValues(t, propertySchema(t, configSchema, "source"), "workspace", "global", "default")
			},
		},
		{
			name: "Should describe Task schemas and enums",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				createTask := operationFor(t, doc, "/api/tasks", "POST")
				createTaskSchema := jsonRequestSchema(t, createTask)
				assertRequired(t, createTaskSchema, "scope", "title")
				assertNotRequired(
					t,
					createTaskSchema,
					"id",
					"identifier",
					"workspace",
					"description",
					"priority",
					"max_attempts",
					"draft",
					"approval_policy",
					"owner",
					"wake_creator",
					"metadata",
				)
				assertEnumValues(
					t,
					propertySchema(t, createTaskSchema, "scope"),
					"global",
					"workspace",
					"user",
					workspaceaccess.UserSelector,
				)
				assertEnumValues(t, propertySchema(t, createTaskSchema, "priority"), "low", "medium", "high", "urgent")
				assertEnumValues(t, propertySchema(t, createTaskSchema, "approval_policy"), "none", "manual")

				createTaskResponse := jsonResponseSchema(t, createTask, 201)
				assertRequired(t, createTaskResponse, "task")
				taskSchema := propertySchema(t, createTaskResponse, "task")
				assertEnumValues(t, propertySchema(t, taskSchema, "scope"), "global", "workspace")
				assertEnumValues(t, propertySchema(t, taskSchema, "priority"), "low", "medium", "high", "urgent")
				assertEnumValues(t, propertySchema(t, taskSchema, "status"),
					string(taskpkg.TaskStatusDraft),
					string(taskpkg.TaskStatusPending),
					string(taskpkg.TaskStatusBlocked),
					string(taskpkg.TaskStatusNeedsAttention),
					string(taskpkg.TaskStatusReady),
					string(taskpkg.TaskStatusInProgress),
					string(taskpkg.TaskStatusCompleted),
					string(taskpkg.TaskStatusFailed),
					string(taskpkg.TaskStatusCanceled),
				)
				assertEnumValues(t, propertySchema(t, taskSchema, "approval_policy"), "none", "manual")
				assertEnumValues(
					t,
					propertySchema(t, taskSchema, "approval_state"),
					"not_required",
					"pending",
					"approved",
					"rejected",
				)
				assertEnumValues(t, propertySchema(t, propertySchema(t, taskSchema, "owner"), "kind"),
					string(taskpkg.OwnerKindHuman),
					string(taskpkg.OwnerKindAgentSession),
					string(taskpkg.OwnerKindAutomation),
					string(taskpkg.OwnerKindExtension),
					string(taskpkg.OwnerKindPool),
				)
				assertEnumValues(t, propertySchema(t, propertySchema(t, taskSchema, "created_by"), "kind"),
					string(taskpkg.ActorKindHuman),
					string(taskpkg.ActorKindAgentSession),
					string(taskpkg.ActorKindAutomation),
					string(taskpkg.ActorKindExtension),
					string(taskpkg.ActorKindDaemon),
				)
				assertEnumValues(t, propertySchema(t, propertySchema(t, taskSchema, "origin"), "kind"),
					string(taskpkg.OriginKindCLI),
					string(taskpkg.OriginKindWeb),
					string(taskpkg.OriginKindUDS),
					string(taskpkg.OriginKindHTTP),
					string(taskpkg.OriginKindAutomation),
					string(taskpkg.OriginKindExtension),
					string(taskpkg.OriginKindAgentSession),
					string(taskpkg.OriginKindDaemon),
				)
				assertRequired(t, taskSchema, "wake_creator")
				propertySchema(t, taskSchema, "needs_attention")
				propertySchema(t, taskSchema, "needs_attention_reason")
				propertySchema(t, taskSchema, "needs_attention_at")
				propertySchema(t, taskSchema, "needs_attention_by")
				blockedReasonsSchema := propertySchema(t, taskSchema, "blocked_reasons")
				blockedReasonSchema := blockedReasonsSchema.Items.Value
				assertEnumValues(t, propertySchema(t, blockedReasonSchema, "source"),
					string(taskpkg.BlockedSourceDependency),
					string(taskpkg.BlockedSourceApproval),
					string(taskpkg.BlockedSourcePaused),
					string(taskpkg.BlockedSourceBlock),
				)
				assertEnumValues(t, propertySchema(t, blockedReasonSchema, "kind"),
					string(taskpkg.BlockKindNeedsInput),
					string(taskpkg.BlockKindCapability),
					string(taskpkg.BlockKindTransient),
				)

				listTasks := operationFor(t, doc, "/api/tasks", "GET")
				assertParameter(t, listTasks, "priority", openapi3.ParameterInQuery, false)
				assertParameter(t, listTasks, "include_drafts", openapi3.ParameterInQuery, false)
				assertParameter(t, listTasks, "include_loop", openapi3.ParameterInQuery, false)
				assertParameter(t, listTasks, "loop_run_id", openapi3.ParameterInQuery, false)
				assertParameter(t, listTasks, "approval_state", openapi3.ParameterInQuery, false)
				assertParameter(t, listTasks, "worktree", openapi3.ParameterInQuery, false)
				assertParameter(t, listTasks, "query", openapi3.ParameterInQuery, false)
				assertParameter(t, listTasks, "sort", openapi3.ParameterInQuery, false)
				assertParameter(t, listTasks, "cursor", openapi3.ParameterInQuery, false)
				assertEnumValues(
					t,
					parameterSchema(t, listTasks, "scope", openapi3.ParameterInQuery),
					"all", "global", "workspace", "user", workspaceaccess.UserSelector,
				)
				listTasksSchema := jsonResponseSchema(t, listTasks, 200)
				assertRequired(t, listTasksSchema, "tasks", "page", "facets")
				taskQueryErrorSchema := jsonResponseSchema(t, listTasks, 400)
				assertRequired(t, taskQueryErrorSchema, "error", "field")
				listItemsSchema := propertySchema(t, listTasksSchema, "tasks")
				if listItemsSchema.Items == nil || listItemsSchema.Items.Value == nil {
					t.Fatal("expected task catalog to define an items schema")
				}
				listItemSchema := listItemsSchema.Items.Value
				for _, field := range []string{
					"paused",
					"paused_by",
					"paused_at",
					"paused_reason",
					"effective_paused",
					"paused_by_task_id",
					"blocked_reasons",
					"dependencies",
				} {
					assertPropertyAbsent(t, listItemSchema, field)
				}
				listRunSchema := propertySchema(t, listItemSchema, "active_run")
				for _, field := range []string{
					"claim_token_hash",
					"designation_group_id",
					"designation",
				} {
					assertPropertyAbsent(t, listRunSchema, field)
				}

				blockTask := operationFor(t, doc, "/api/tasks/{id}/blocks", "POST")
				assertParameter(t, blockTask, "id", openapi3.ParameterInPath, true)
				blockTaskSchema := jsonRequestSchema(t, blockTask)
				assertRequired(t, blockTaskSchema, "kind", "reason")
				assertNotRequired(t, blockTaskSchema, "details", "expires_at", "run_id")
				assertEnumValues(t, propertySchema(t, blockTaskSchema, "kind"),
					string(taskpkg.BlockKindNeedsInput),
					string(taskpkg.BlockKindCapability),
					string(taskpkg.BlockKindTransient),
				)
				blockTaskResponse := jsonResponseSchema(t, blockTask, 201)
				assertRequired(t, blockTaskResponse, "block")
				blockSchema := propertySchema(t, blockTaskResponse, "block")
				assertRequired(t, blockSchema, "id", "task_id", "kind", "reason", "created_by", "created_at")

				listTaskBlocks := operationFor(t, doc, "/api/tasks/{id}/blocks", "GET")
				assertParameter(t, listTaskBlocks, "id", openapi3.ParameterInPath, true)
				assertParameter(t, listTaskBlocks, "include_cleared", openapi3.ParameterInQuery, false)
				assertResponseStatus(t, listTaskBlocks, 400)
				listTaskBlocksResponse := jsonResponseSchema(t, listTaskBlocks, 200)
				assertRequired(t, listTaskBlocksResponse, "blocks")

				clearTaskBlock := operationFor(t, doc, "/api/tasks/{id}/blocks/{block_id}/clear", "POST")
				assertParameter(t, clearTaskBlock, "id", openapi3.ParameterInPath, true)
				assertParameter(t, clearTaskBlock, "block_id", openapi3.ParameterInPath, true)
				clearTaskBlockSchema := jsonRequestSchema(t, clearTaskBlock)
				assertNotRequired(t, clearTaskBlockSchema, "note")
				clearTaskBlockResponse := jsonResponseSchema(t, clearTaskBlock, 200)
				assertRequired(t, clearTaskBlockResponse, "block")

				recoverTask := operationFor(t, doc, "/api/tasks/{id}/recover", "POST")
				assertParameter(t, recoverTask, "id", openapi3.ParameterInPath, true)
				recoverTaskSchema := jsonRequestSchema(t, recoverTask)
				assertNotRequired(t, recoverTaskSchema, "note")
				recoverTaskResponse := jsonResponseSchema(t, recoverTask, 200)
				assertRequired(t, recoverTaskResponse, "task")

				getTask := operationFor(t, doc, "/api/tasks/{id}", "GET")
				getTaskSchema := jsonResponseSchema(t, getTask, 200)
				assertRequired(t, getTaskSchema, "task")
				detailSchema := propertySchema(t, getTaskSchema, "task")
				assertRequired(t, detailSchema, "summary", "task")
				assertNotRequired(
					t,
					detailSchema,
					"children",
					"dependencies",
					"dependency_references",
					"runs",
					"events",
				)

				summarySchema := propertySchema(t, detailSchema, "summary")
				assertRequired(
					t,
					summarySchema,
					"id",
					"scope",
					"title",
					"status",
					"created_by",
					"origin",
					"created_at",
					"updated_at",
					"wake_creator",
				)
				assertNotRequired(
					t,
					summarySchema,
					"priority",
					"max_attempts",
					"approval_policy",
					"approval_state",
					"draft",
					"dependencies",
					"active_run",
					"last_activity_at",
				)

				for _, path := range []string{
					"/api/task-runs/{id}", "/api/runs/{id}/inspect", "/api/task-runs/{id}/result",
					"/api/tasks/{id}/runs", "/api/tasks/{id}/timeline", "/api/tasks/{id}/stream",
					"/api/tasks/{id}/tree", "/api/task-runs/{id}/reviews",
				} {
					read := operationFor(t, doc, path, http.MethodGet)
					assertParameter(t, read, "profile", openapi3.ParameterInQuery, false)
					assertParameter(t, read, "all_profiles", openapi3.ParameterInQuery, false)
				}
				cancelRun := operationFor(t, doc, "/api/task-runs/{id}/cancel", http.MethodPost)
				assertParameter(t, cancelRun, "profile", openapi3.ParameterInQuery, false)
				assertParameterAbsent(t, cancelRun, "all_profiles", openapi3.ParameterInQuery)
				scheduler := operationFor(t, doc, "/api/scheduler", http.MethodGet)
				assertParameterAbsent(t, scheduler, "profile", openapi3.ParameterInQuery)
				assertParameterAbsent(t, scheduler, "all_profiles", openapi3.ParameterInQuery)

				listTaskRuns := operationFor(t, doc, "/api/tasks/{id}/runs", "GET")
				assertParameter(t, listTaskRuns, "status", openapi3.ParameterInQuery, false)
				assertParameter(t, listTaskRuns, "session_id", openapi3.ParameterInQuery, false)

				completeTaskRun := operationFor(t, doc, "/api/task-runs/{id}/complete", "POST")
				completeTaskRunSchema := jsonRequestSchema(t, completeTaskRun)
				propertySchema(t, completeTaskRunSchema, "created_task_ids")

				completeAgentTaskRun := operationFor(t, doc, "/api/agent/tasks/{run_id}/complete", "POST")
				completeAgentTaskRunSchema := jsonRequestSchema(t, completeAgentTaskRun)
				propertySchema(t, completeAgentTaskRunSchema, "created_task_ids")

				getRun := operationFor(t, doc, "/api/task-runs/{id}", "GET")
				getRunSchema := jsonResponseSchema(t, getRun, 200)
				assertRequired(t, getRunSchema, "run")
				runDetailSchema := propertySchema(t, getRunSchema, "run")
				assertRequired(t, runDetailSchema, "run", "summary")
				assertNotRequired(t, runDetailSchema, "task", "session")

				runSchema := propertySchema(t, runDetailSchema, "run")
				assertEnumValues(t, propertySchema(t, runSchema, "status"),
					taskpkg.TaskRunStatusQueued.String(),
					taskpkg.TaskRunStatusClaimed.String(),
					taskpkg.TaskRunStatusStarting.String(),
					taskpkg.TaskRunStatusRunning.String(),
					taskpkg.TaskRunStatusCompleted.String(),
					taskpkg.TaskRunStatusFailed.String(),
					taskpkg.TaskRunStatusCanceled.String(),
					taskpkg.TaskRunStatusNeedsAttention.String(),
				)
				assertEnumValues(t, propertySchema(t, propertySchema(t, runSchema, "origin"), "kind"),
					string(taskpkg.OriginKindCLI),
					string(taskpkg.OriginKindWeb),
					string(taskpkg.OriginKindUDS),
					string(taskpkg.OriginKindHTTP),
					string(taskpkg.OriginKindAutomation),
					string(taskpkg.OriginKindExtension),
					string(taskpkg.OriginKindAgentSession),
					string(taskpkg.OriginKindDaemon),
				)

				addDependency := operationFor(t, doc, "/api/tasks/{id}/dependencies", "POST")
				addDependencySchema := jsonRequestSchema(t, addDependency)
				assertRequired(t, addDependencySchema, "depends_on_task_id")
				assertNotRequired(t, addDependencySchema, "kind")
				assertEnumValues(
					t,
					propertySchema(t, addDependencySchema, "kind"),
					string(taskpkg.DependencyKindBlocks),
				)

				attachRun := operationFor(t, doc, "/api/task-runs/{id}/attach-session", "POST")
				attachRunSchema := jsonRequestSchema(t, attachRun)
				assertRequired(t, attachRunSchema, "session_id")

				publishTask := operationFor(t, doc, "/api/tasks/{id}/publish", "POST")
				assertParameter(t, publishTask, "id", openapi3.ParameterInPath, true)
				publishTaskSchema := jsonRequestSchema(t, publishTask)
				assertNotRequired(t, publishTaskSchema, "idempotency_key", "metadata")
				publishTaskResponse := jsonResponseSchema(t, publishTask, 200)
				assertRequired(t, publishTaskResponse, "task", "run")
				assertResponseStatus(t, publishTask, 409)
				assertResponseStatus(t, publishTask, 422)

				startTask := operationFor(t, doc, "/api/tasks/{id}/start", "POST")
				assertParameter(t, startTask, "id", openapi3.ParameterInPath, true)
				startTaskSchema := jsonRequestSchema(t, startTask)
				assertNotRequired(t, startTaskSchema, "idempotency_key", "metadata")
				startTaskResponse := jsonResponseSchema(t, startTask, 201)
				assertRequired(t, startTaskResponse, "task", "run")
				assertResponseStatus(t, startTask, 409)
				assertResponseStatus(t, startTask, 422)

				pauseTask := operationFor(t, doc, "/api/tasks/{id}/pause", "POST")
				assertParameter(t, pauseTask, "id", openapi3.ParameterInPath, true)
				pauseTaskSchema := jsonRequestSchema(t, pauseTask)
				assertRequired(t, pauseTaskSchema, "reason")
				assertNotRequired(t, pauseTaskSchema, "metadata")
				pauseTaskResponse := jsonResponseSchema(t, pauseTask, 200)
				assertRequired(t, pauseTaskResponse, "task")
				assertResponseStatus(t, pauseTask, 403)
				assertResponseStatus(t, pauseTask, 409)
				assertResponseStatus(t, pauseTask, 429)

				resumeTask := operationFor(t, doc, "/api/tasks/{id}/resume", "POST")
				assertParameter(t, resumeTask, "id", openapi3.ParameterInPath, true)
				if resumeTask.RequestBody == nil || resumeTask.RequestBody.Value == nil {
					t.Fatal("resume task missing optional request body schema")
				}
				if resumeTask.RequestBody.Value.Required {
					t.Fatal("resume task request body is required, want optional")
				}
				resumeTaskSchema := jsonRequestSchema(t, resumeTask)
				assertNotRequired(t, resumeTaskSchema, "metadata")
				resumeTaskResponse := jsonResponseSchema(t, resumeTask, 200)
				assertRequired(t, resumeTaskResponse, "task")
				assertResponseStatus(t, resumeTask, 403)
				assertResponseStatus(t, resumeTask, 409)
				assertResponseStatus(t, resumeTask, 429)
			},
		},
		{
			name: "ShouldDescribeSchedulerControlSchemas",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				status := operationFor(t, doc, "/api/scheduler", "GET")
				assertTagsContain(t, status, "tasks")
				statusResponse := jsonResponseSchema(t, status, 200)
				assertRequired(t, statusResponse, "scheduler")
				schedulerSchema := propertySchema(t, statusResponse, "scheduler")
				assertRequired(
					t,
					schedulerSchema,
					"paused",
					"active_claim_count",
					"queued_run_count",
					"paused_task_count",
					"as_of",
				)
				assertNotRequired(t, schedulerSchema, "paused_by", "paused_at", "paused_reason")

				pauseScheduler := operationFor(t, doc, "/api/scheduler/pause", "POST")
				if pauseScheduler.RequestBody == nil || pauseScheduler.RequestBody.Value == nil {
					t.Fatal("pause scheduler missing optional request body schema")
				}
				if pauseScheduler.RequestBody.Value.Required {
					t.Fatal("pause scheduler request body is required, want optional")
				}
				pauseSchema := jsonRequestSchema(t, pauseScheduler)
				assertNotRequired(t, pauseSchema, "reason")
				assertRequired(t, jsonResponseSchema(t, pauseScheduler, 200), "scheduler")
				assertResponseStatus(t, pauseScheduler, 403)
				assertResponseStatus(t, pauseScheduler, 422)

				resumeScheduler := operationFor(t, doc, "/api/scheduler/resume", "POST")
				if resumeScheduler.RequestBody == nil || resumeScheduler.RequestBody.Value == nil {
					t.Fatal("resume scheduler missing optional request body schema")
				}
				if resumeScheduler.RequestBody.Value.Required {
					t.Fatal("resume scheduler request body is required, want optional")
				}
				resumeSchema := jsonRequestSchema(t, resumeScheduler)
				assertNotRequired(t, resumeSchema, "reason")
				assertRequired(t, jsonResponseSchema(t, resumeScheduler, 200), "scheduler")
				assertResponseStatus(t, resumeScheduler, 403)
				assertResponseStatus(t, resumeScheduler, 422)

				drainScheduler := operationFor(t, doc, "/api/scheduler/drain", "POST")
				if drainScheduler.RequestBody == nil || drainScheduler.RequestBody.Value == nil {
					t.Fatal("drain scheduler missing optional request body schema")
				}
				if drainScheduler.RequestBody.Value.Required {
					t.Fatal("drain scheduler request body is required, want optional")
				}
				drainSchema := jsonRequestSchema(t, drainScheduler)
				assertNotRequired(t, drainSchema, "reason", "timeout_seconds")
				timeoutSeconds := propertySchema(t, drainSchema, "timeout_seconds")
				if timeoutSeconds.Format != specFormatInt64 {
					t.Fatalf("scheduler timeout_seconds format = %q, want %q", timeoutSeconds.Format, specFormatInt64)
				}
				if timeoutSeconds.Min == nil || *timeoutSeconds.Min != 0 {
					t.Fatalf("scheduler timeout_seconds minimum = %v, want 0", timeoutSeconds.Min)
				}
				if timeoutSeconds.Max == nil ||
					*timeoutSeconds.Max != float64(contract.SchedulerDrainTimeoutMaxSeconds) {
					t.Fatalf(
						"scheduler timeout_seconds maximum = %v, want %d",
						timeoutSeconds.Max,
						contract.SchedulerDrainTimeoutMaxSeconds,
					)
				}
				drainResponse := jsonResponseSchema(t, drainScheduler, 200)
				assertRequired(
					t,
					drainResponse,
					"scheduler",
					"completed",
					"remaining_claims",
					"started_at",
					"completed_at",
				)
				assertResponseStatus(t, drainScheduler, 403)
				assertResponseStatus(t, drainScheduler, 400)
				assertResponseStatus(t, drainScheduler, 422)

				backlog := operationFor(t, doc, "/api/scheduler/backlog", "GET")
				assertParameter(t, backlog, "limit", openapi3.ParameterInQuery, false)
				assertParameter(t, backlog, "scope", openapi3.ParameterInQuery, false)
				assertParameter(t, backlog, "workspace", openapi3.ParameterInQuery, false)
				assertParameter(t, backlog, "include_paused", openapi3.ParameterInQuery, false)
				backlogResponse := jsonResponseSchema(t, backlog, 200)
				assertRequired(t, backlogResponse, "backlog")
				backlogSchema := propertySchema(t, backlogResponse, "backlog")
				assertRequired(t, backlogSchema, "runs", "total")
				runsSchema := propertySchema(t, backlogSchema, "runs")
				if runsSchema.Items == nil || runsSchema.Items.Value == nil {
					t.Fatal("expected backlog runs to define an items schema")
				}
				backlogRunSchema := runsSchema.Items.Value
				assertRequired(t, backlogRunSchema, "task", "run")
				backlogTaskSchema := propertySchema(t, backlogRunSchema, "task")
				propertySchema(t, backlogTaskSchema, "paused")
				propertySchema(t, backlogTaskSchema, "effective_paused")
				propertySchema(t, backlogTaskSchema, "paused_by_task_id")
			},
		},
		{
			name: "ShouldDescribeTaskLiveAndObserveSchemas",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				timeline := operationFor(t, doc, "/api/tasks/{id}/timeline", "GET")
				assertParameter(t, timeline, "id", openapi3.ParameterInPath, true)
				assertParameter(t, timeline, "after_sequence", openapi3.ParameterInQuery, false)
				assertParameter(t, timeline, "limit", openapi3.ParameterInQuery, false)
				timelineSchema := jsonResponseSchema(t, timeline, 200)
				assertRequired(t, timelineSchema, "timeline")

				timelineItems := propertySchema(t, timelineSchema, "timeline")
				if timelineItems.Items == nil || timelineItems.Items.Value == nil {
					t.Fatal("expected timeline to define an items schema")
				}
				timelineItemSchema := timelineItems.Items.Value
				assertRequired(
					t,
					timelineItemSchema,
					"sequence",
					"event_id",
					"task",
					"event_type",
					"actor",
					"origin",
					"timestamp",
				)
				assertNotRequired(t, timelineItemSchema, "run", "payload")

				stream := operationFor(t, doc, "/api/tasks/{id}/stream", "GET")
				assertParameter(t, stream, "id", openapi3.ParameterInPath, true)
				assertParameter(t, stream, "after_sequence", openapi3.ParameterInQuery, false)
				streamSchema := responseSchema(t, stream, 200, "text/event-stream")
				assertRequired(t, streamSchema, "sequence", "type", "timeline")
				assertResponseStatus(t, stream, 422)

				tree := operationFor(t, doc, "/api/tasks/{id}/tree", "GET")
				treeSchema := jsonResponseSchema(t, tree, 200)
				assertRequired(t, treeSchema, "tree")
				treePayload := propertySchema(t, treeSchema, "tree")
				assertRequired(t, treePayload, "root")
				assertNotRequired(t, treePayload, "descendants")

				approve := operationFor(t, doc, "/api/tasks/{id}/approve", "POST")
				if approve.RequestBody == nil || approve.RequestBody.Value == nil {
					t.Fatal("approve task missing optional request body schema")
				}
				if approve.RequestBody.Value.Required {
					t.Fatal("approve task request body is required, want optional")
				}
				approveSchema := jsonRequestSchema(t, approve)
				assertNotRequired(t, approveSchema, "idempotency_key", "metadata")
				approveResponse := jsonResponseSchema(t, approve, 201)
				assertRequired(t, approveResponse, "task", "run")
				assertResponseStatus(t, approve, 409)
				assertResponseStatus(t, approve, 422)

				dashboard := operationFor(t, doc, "/api/observe/tasks/dashboard", "GET")
				assertTagsContain(t, dashboard, "observe", "tasks")
				assertParameter(t, dashboard, "scope", openapi3.ParameterInQuery, false)
				assertParameter(t, dashboard, "workspace", openapi3.ParameterInQuery, false)
				assertParameter(t, dashboard, "worktree", openapi3.ParameterInQuery, false)
				assertParameter(t, dashboard, "owner_kind", openapi3.ParameterInQuery, false)
				assertParameter(t, dashboard, "owner_ref", openapi3.ParameterInQuery, false)
				assertParameter(t, dashboard, "origin_kind", openapi3.ParameterInQuery, false)
				dashboardSchema := jsonResponseSchema(t, dashboard, 200)
				assertRequired(t, dashboardSchema, "dashboard")
				assertObjectPropertyKeys(
					t,
					propertySchema(t, dashboardSchema, "dashboard"),
					"totals",
					"cards",
					"queue",
					"health",
					"active_runs",
					"freshness",
				)

				inbox := operationFor(t, doc, "/api/observe/tasks/inbox", "GET")
				assertTagsContain(t, inbox, "observe", "tasks")
				assertParameter(t, inbox, "worktree", openapi3.ParameterInQuery, false)
				assertParameter(t, inbox, "lane", openapi3.ParameterInQuery, false)
				assertParameter(t, inbox, "unread", openapi3.ParameterInQuery, false)
				assertParameter(t, inbox, "query", openapi3.ParameterInQuery, false)
				assertParameter(t, inbox, "status", openapi3.ParameterInQuery, false)
				assertParameter(t, inbox, "priority", openapi3.ParameterInQuery, false)
				assertParameter(t, inbox, "cursor", openapi3.ParameterInQuery, false)
				inboxSchema := jsonResponseSchema(t, inbox, 200)
				assertRequired(t, inboxSchema, "inbox")

				inboxPayload := propertySchema(t, inboxSchema, "inbox")
				assertRequired(t, inboxPayload, "page", "facets", "unread_total", "archived_total", "groups")
				assertRequired(t, propertySchema(t, inboxPayload, "page"), "total", "limit", "has_more")
				groupsSchema := propertySchema(t, inboxPayload, "groups")
				if groupsSchema.Items == nil || groupsSchema.Items.Value == nil {
					t.Fatal("expected groups to define an items schema")
				}
				groupSchema := groupsSchema.Items.Value
				assertRequired(t, groupSchema, "lane", "count", "unread_count")
				assertNotRequired(t, groupSchema, "items")
				groupItemsSchema := propertySchema(t, groupSchema, "items")
				if groupItemsSchema.Items == nil || groupItemsSchema.Items.Value == nil {
					t.Fatal("expected task inbox group items to define an items schema")
				}
				inboxItemSchema := groupItemsSchema.Items.Value
				inboxTaskSchema := propertySchema(t, inboxItemSchema, "task")
				for _, field := range []string{"paused", "effective_paused", "paused_by_task_id"} {
					assertPropertyAbsent(t, inboxTaskSchema, field)
				}
				inboxRunSchema := propertySchema(t, inboxItemSchema, "run")
				for _, field := range []string{
					"claim_token_hash",
					"designation_group_id",
					"designation",
				} {
					assertPropertyAbsent(t, inboxRunSchema, field)
				}
				assertEnumValues(
					t,
					propertySchema(t, groupSchema, "lane"),
					"my_work",
					"approvals",
					"failed_runs",
					"blocked",
					"archived",
				)

				triage := operationFor(t, doc, "/api/tasks/{id}/triage/archive", "POST")
				triageSchema := jsonResponseSchema(t, triage, 200)
				assertRequired(t, triageSchema, "triage")
				triagePayload := propertySchema(t, triageSchema, "triage")
				assertRequired(t, triagePayload, "task_id", "actor", "read", "archived", "dismissed", "updated_at")
				assertNotRequired(t, triagePayload, "last_seen_activity_at")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.check(t, doc)
		})
	}
}

func TestWriteFileAndEnumHelpers(t *testing.T) {
	t.Parallel()

	t.Run("Should render the document in memory with the same bytes as WriteFile", func(t *testing.T) {
		t.Parallel()

		rendered, err := Render()
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if !json.Valid(rendered) {
			t.Fatalf("Render() output is not valid JSON: %s", string(rendered))
		}
		if !strings.HasSuffix(string(rendered), "\n") {
			t.Fatalf("Render() output must end with newline: %q", string(rendered))
		}

		path := filepath.Join(t.TempDir(), "openapi", "compozy.json")
		if err := WriteFile(path); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		written, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("os.ReadFile() error = %v", err)
		}
		if !bytes.Equal(written, rendered) {
			t.Fatalf("WriteFile() output must match Render()")
		}
	})
}

func TestSchemaCustomizerCoversAdditionalEnums(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		typ  any
	}{
		{name: "TaskScope", typ: taskpkg.ScopeGlobal},
		{name: "TaskStatus", typ: taskpkg.TaskStatusReady},
		{name: "TaskPriority", typ: taskpkg.PriorityHigh},
		{name: "TaskApprovalPolicy", typ: taskpkg.ApprovalPolicyManual},
		{name: "TaskApprovalState", typ: taskpkg.ApprovalStateApproved},
		{name: "TaskRunStatus", typ: taskpkg.TaskRunStatusQueued},
		{name: "TaskActorKind", typ: taskpkg.ActorKindHuman},
		{name: "TaskOwnerKind", typ: taskpkg.OwnerKindPool},
		{name: "TaskOriginKind", typ: taskpkg.OriginKindHTTP},
		{name: "TaskDependencyKind", typ: taskpkg.DependencyKindBlocks},
		{name: "TaskBlockKind", typ: taskpkg.BlockKindNeedsInput},
		{name: "TaskBlockedSource", typ: taskpkg.BlockedSourceBlock},
		{name: "TaskWorktreeMode", typ: taskpkg.WorktreeModePerRun},
		{name: "ResolvedWorktreeMode", typ: contract.ResolvedWorktreeModePerRun},
		{name: "TaskRuntimeMode", typ: taskpkg.RuntimeModeDefault},
		{name: "LoopEnvironmentMode", typ: contract.LoopEnvironmentModePerRun},
		{name: "AutomationSchedulerCatchUpPolicy", typ: automationpkg.SchedulerCatchUpPolicySkipMissed},
		{name: "TaskInboxLane", typ: contract.TaskInboxLaneApprovals},
		{name: "Should cover LoopProvenanceRole", typ: contract.LoopProvenanceRoleCoordinator},
		{name: "IssueSeverity", typ: contract.IssueSeverityError},
		{name: "HookSkillSource", typ: hooks.HookSkillSourceBundled},
		{name: "HookExecutorKind", typ: hooks.HookExecutorNative},
		{name: "ToolSource", typ: tools.ToolSourceBuiltin},
		{name: "HostAPIMethod", typ: extensionprotocol.HostAPIMethod("session.read")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			schema := openapi3.NewStringSchema()
			if err := schemaCustomizer("", reflect.TypeOf(tt.typ), "", schema); err != nil {
				t.Fatalf("schemaCustomizer() error = %v", err)
			}
			if len(schema.Enum) == 0 {
				t.Fatalf("schemaCustomizer() enum = %v, want non-empty", schema.Enum)
			}
		})
	}
}

func TestStopWhenSpecSchemaShouldMatchTheDualWireFormat(t *testing.T) {
	t.Parallel()

	schema := openapi3.NewSchema()
	if err := schemaCustomizer("", reflect.TypeFor[dsl.StopWhenSpec](), "", schema); err != nil {
		t.Fatalf("schemaCustomizer() error = %v", err)
	}
	if got, want := len(schema.OneOf), 2; got != want {
		t.Fatalf("stop_when oneOf variants = %d, want %d", got, want)
	}
	if scalar := schema.OneOf[0].Value; scalar == nil || scalar.Type == nil || !scalar.Type.Is("string") {
		t.Fatalf("stop_when scalar schema = %#v, want string", scalar)
	}
	object := schema.OneOf[1].Value
	if object == nil || object.Type == nil || !object.Type.Is("object") ||
		object.AdditionalProperties.Has == nil || *object.AdditionalProperties.Has ||
		!slices.Equal(object.Required, []string{"expr"}) {
		t.Fatalf("stop_when object schema = %#v, want closed object requiring expr", object)
	}
	policy := object.Properties["on_eval_error"].Value
	if policy == nil || !reflect.DeepEqual(policy.Enum, []any{"fail", "exit"}) {
		t.Fatalf("stop_when on_eval_error enum = %#v, want fail and exit", policy)
	}
}

func TestExtensionAuthoringComponentSchemas(t *testing.T) {
	t.Parallel()

	t.Run("Should expose closed validation contracts", func(t *testing.T) {
		t.Parallel()

		document, err := Document()
		if err != nil {
			t.Fatalf("Document() error = %v", err)
		}
		severity := document.Components.Schemas["IssueSeverity"]
		if severity == nil || severity.Value == nil {
			t.Fatal("IssueSeverity component is missing")
		}
		if got, want := severity.Value.Enum, []any{"error", "warning", "warn"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("IssueSeverity enum = %v, want %v", got, want)
		}
		payload := document.Components.Schemas["ExtensionValidatePayload"]
		if payload == nil || payload.Value == nil {
			t.Fatal("ExtensionValidatePayload component is missing")
		}
		for _, field := range []string{"status", "format", "issues"} {
			if !slices.Contains(payload.Value.Required, field) {
				t.Fatalf("ExtensionValidatePayload required = %v, want %q", payload.Value.Required, field)
			}
		}
	})
}

func TestEnumHelpersReturnStableValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  []string
		want []string
	}{
		{
			name: "hook skill source values",
			got:  hookSkillSourceValues(),
			want: []string{"bundled", "marketplace", "user", "additional", "workspace"},
		},
		{
			name: "hook executor kind values",
			got:  hookExecutorKindValues(),
			want: []string{"native", "subprocess", "wasm"},
		},
		{
			name: "tool source values",
			got:  toolSourceValues(),
			want: []string{"builtin", "mcp", "extension", "dynamic"},
		},
		{
			name: "task scope values",
			got:  taskScopeValues(),
			want: []string{"global", "workspace"},
		},
		{
			name: "task status values",
			got:  taskStatusValues(),
			want: []string{
				string(taskpkg.TaskStatusDraft),
				string(taskpkg.TaskStatusPending),
				string(taskpkg.TaskStatusBlocked),
				string(taskpkg.TaskStatusNeedsAttention),
				string(taskpkg.TaskStatusReady),
				string(taskpkg.TaskStatusInProgress),
				string(taskpkg.TaskStatusCompleted),
				string(taskpkg.TaskStatusFailed),
				string(taskpkg.TaskStatusCanceled),
			},
		},
		{
			name: "task priority values",
			got:  taskPriorityValues(),
			want: []string{"low", "medium", "high", "urgent"},
		},
		{
			name: "task approval policy values",
			got:  taskApprovalPolicyValues(),
			want: []string{"none", "manual"},
		},
		{
			name: "task approval state values",
			got:  taskApprovalStateValues(),
			want: []string{"not_required", "pending", "approved", "rejected"},
		},
		{
			name: "task block kind values",
			got:  taskBlockKindValues(),
			want: []string{"needs_input", "capability", "transient"},
		},
		{
			name: "task blocked source values",
			got:  taskBlockedSourceValues(),
			want: []string{"dependency", "approval", "paused", "block"},
		},
		{
			name: "task run status values",
			got:  taskRunStatusValues(),
			want: []string{
				taskpkg.TaskRunStatusQueued.String(),
				taskpkg.TaskRunStatusClaimed.String(),
				taskpkg.TaskRunStatusStarting.String(),
				taskpkg.TaskRunStatusRunning.String(),
				taskpkg.TaskRunStatusCompleted.String(),
				taskpkg.TaskRunStatusFailed.String(),
				taskpkg.TaskRunStatusCanceled.String(),
				taskpkg.TaskRunStatusNeedsAttention.String(),
			},
		},
		{
			name: "task actor kind values",
			got:  taskActorKindValues(),
			want: []string{"human", "agent_session", "automation", "extension", "daemon"},
		},
		{
			name: "task owner kind values",
			got:  taskOwnerKindValues(),
			want: []string{"human", "agent_session", "automation", "extension", "pool"},
		},
		{
			name: "task origin kind values",
			got:  taskOriginKindValues(),
			want: []string{
				"cli",
				"web",
				"uds",
				"http",
				"automation",
				"extension",
				"agent_session",
				"daemon",
			},
		},
		{
			name: "task dependency kind values",
			got:  taskDependencyKindValues(),
			want: []string{"blocks"},
		},
		{
			name: "task inbox lane values",
			got:  taskInboxLaneValues(),
			want: []string{"my_work", "approvals", "failed_runs", "blocked", "archived"},
		},
		{
			name: "automation scheduler catch-up policy values",
			got:  automationSchedulerCatchUpPolicyValues(),
			want: []string{"skip_missed", "coalesce", "replay", "run_once_on_catchup"},
		},
		{
			name: "loop run status values",
			got:  loopRunStatusValues(),
			want: contract.LoopRunStatusValues(),
		},
		{
			name: "loop run event kind values",
			got:  loopRunEventKindValues(),
			want: contract.LoopRunEventKindValues(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if !slices.Equal(tt.got, tt.want) {
				t.Fatalf("values = %v, want %v", tt.got, tt.want)
			}
		})
	}

	t.Run("Should include config path failures in tool reason codes", func(t *testing.T) {
		t.Parallel()

		if got := toolReasonCodeValues(); !slices.Contains(got, string(tools.ReasonConfigPathNotFound)) {
			t.Fatalf("toolReasonCodeValues() = %v, want %q", got, tools.ReasonConfigPathNotFound)
		}
	})

	t.Run("Should host api method values", func(t *testing.T) {
		t.Parallel()

		got := hostAPIMethodValues()
		if !slices.IsSorted(got) {
			t.Fatalf("values are not sorted: %v", got)
		}
		for _, want := range []string{"tasks/runs/complete", "sessions/health/get", "agents/soul/get"} {
			if !contains(got, want) {
				t.Fatalf("expected %q in host api method values %v", want, got)
			}
		}
	})
}

func operationFor(t *testing.T, doc *openapi3.T, path string, method string) *openapi3.Operation {
	t.Helper()

	pathItem := doc.Paths.Value(path)
	if pathItem == nil {
		t.Fatalf("missing path %q", path)
	}
	operation := pathItem.GetOperation(method)
	if operation == nil {
		t.Fatalf("missing operation %s %s", method, path)
	}
	return operation
}

func jsonResponseSchema(t *testing.T, operation *openapi3.Operation, status int) *openapi3.Schema {
	t.Helper()

	return responseSchema(t, operation, status, "application/json")
}

func responseSchema(t *testing.T, operation *openapi3.Operation, status int, contentType string) *openapi3.Schema {
	t.Helper()

	responseRef := operation.Responses.Status(status)
	if responseRef == nil || responseRef.Value == nil {
		t.Fatalf("missing %d response", status)
	}
	mediaType := responseRef.Value.Content.Get(contentType)
	if mediaType == nil || mediaType.Schema == nil || mediaType.Schema.Value == nil {
		t.Fatalf("missing %s schema for %d response", contentType, status)
	}
	return mediaType.Schema.Value
}

func jsonRequestSchema(t *testing.T, operation *openapi3.Operation) *openapi3.Schema {
	t.Helper()

	if operation.RequestBody == nil || operation.RequestBody.Value == nil {
		t.Fatal("missing request body")
	}
	mediaType := operation.RequestBody.Value.Content.Get("application/json")
	if mediaType == nil || mediaType.Schema == nil || mediaType.Schema.Value == nil {
		t.Fatal("missing application/json request schema")
	}
	return mediaType.Schema.Value
}

func propertySchema(t *testing.T, schema *openapi3.Schema, name string) *openapi3.Schema {
	t.Helper()
	schema = resolvedTestSchema(schema)

	propertyRef := schema.Properties[name]
	if propertyRef == nil || propertyRef.Value == nil {
		t.Fatalf("missing property %q", name)
	}
	return resolvedTestSchema(propertyRef.Value)
}

func assertAgentNameRequestSchema(t *testing.T, owner string, schema *openapi3.Schema, pattern string) {
	t.Helper()

	if schema.Type == nil || !schema.Type.Is("string") {
		t.Fatalf("%s type = %#v, want string", owner, schema.Type)
	}
	if schema.Pattern != pattern {
		t.Fatalf("%s pattern = %q, want %q", owner, schema.Pattern, pattern)
	}
	if schema.MaxLength == nil || *schema.MaxLength != uint64(compozyconfig.AgentNameMaxLength) {
		t.Fatalf("%s maxLength = %v, want %d", owner, schema.MaxLength, compozyconfig.AgentNameMaxLength)
	}
}

func assertPropertyAbsent(t *testing.T, schema *openapi3.Schema, name string) {
	t.Helper()

	if propertyRef := schema.Properties[name]; propertyRef != nil {
		t.Fatalf("expected property %q to be absent", name)
	}
}

func assertParameter(t *testing.T, operation *openapi3.Operation, name string, in string, required bool) {
	t.Helper()

	parameterSchema(t, operation, name, in)
	for _, ref := range operation.Parameters {
		if ref == nil || ref.Value == nil {
			continue
		}
		if ref.Value.Name == name && ref.Value.In == in {
			if ref.Value.Required != required {
				t.Fatalf("parameter %s in %s required = %v, want %v", name, in, ref.Value.Required, required)
			}
			return
		}
	}
	t.Fatalf("missing parameter %q in %s", name, in)
}

func assertParameterAbsent(t *testing.T, operation *openapi3.Operation, name string, in string) {
	t.Helper()

	for _, ref := range operation.Parameters {
		if ref == nil || ref.Value == nil {
			continue
		}
		if ref.Value.Name == name && ref.Value.In == in {
			t.Fatalf("expected parameter %q in %s to be absent", name, in)
		}
	}
}

func parameterSchema(t *testing.T, operation *openapi3.Operation, name string, in string) *openapi3.Schema {
	t.Helper()

	for _, ref := range operation.Parameters {
		if ref == nil || ref.Value == nil {
			continue
		}
		if ref.Value.Name == name && ref.Value.In == in {
			if ref.Value.Schema == nil || ref.Value.Schema.Value == nil {
				t.Fatalf("missing schema for parameter %q in %s", name, in)
			}
			return ref.Value.Schema.Value
		}
	}
	t.Fatalf("missing parameter %q in %s", name, in)
	return nil
}

func assertResponseStatus(t *testing.T, operation *openapi3.Operation, status int) {
	t.Helper()

	if ref := operation.Responses.Status(status); ref == nil || ref.Value == nil {
		t.Fatalf("missing %d response", status)
	}
}

func assertTagsContain(t *testing.T, operation *openapi3.Operation, tags ...string) {
	t.Helper()

	for _, tag := range tags {
		if !contains(operation.Tags, tag) {
			t.Fatalf("expected tags %v to contain %q", operation.Tags, tag)
		}
	}
}

func assertRequired(t *testing.T, schema *openapi3.Schema, names ...string) {
	t.Helper()
	for _, name := range names {
		if !contains(schema.Required, name) {
			t.Fatalf("expected %q to be required, got %v", name, schema.Required)
		}
	}
}

func assertNotRequired(t *testing.T, schema *openapi3.Schema, names ...string) {
	t.Helper()
	for _, name := range names {
		if contains(schema.Required, name) {
			t.Fatalf("expected %q to be optional, got %v", name, schema.Required)
		}
	}
}

func assertEnumValues(t *testing.T, schema *openapi3.Schema, values ...string) {
	t.Helper()

	got := make([]string, 0, len(schema.Enum))
	for idx, value := range schema.Enum {
		text, ok := value.(string)
		if !ok {
			t.Fatalf("expected enum[%d] to be string, got %T", idx, value)
		}
		got = append(got, text)
	}

	want := append([]string(nil), values...)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("expected enum values %v, got %v", want, got)
	}
}

func assertSchemaIncludesType(t *testing.T, schema *openapi3.Schema, want string) {
	t.Helper()

	if !schemaIncludesType(schema, want) {
		t.Fatalf("expected schema types to include %q, got %#v", want, schema.Type)
	}
}

func assertSchemaHasAdditionalProperties(t *testing.T, schema *openapi3.Schema, want bool) {
	t.Helper()

	if want {
		if schema.AdditionalProperties.Schema == nil &&
			(schema.AdditionalProperties.Has == nil || !*schema.AdditionalProperties.Has) {
			t.Fatalf("expected additionalProperties to be allowed, got %#v", schema.AdditionalProperties)
		}
		return
	}
	if schema.AdditionalProperties.Has == nil {
		t.Fatalf("expected additionalProperties=%v, got nil", want)
	}
	if got := *schema.AdditionalProperties.Has; got != want {
		t.Fatalf("expected additionalProperties=%v, got %v", want, got)
	}
}

func assertAdditionalPropertiesSchemaIncludesType(t *testing.T, schema *openapi3.Schema, want string) {
	t.Helper()

	additional := schema.AdditionalProperties.Schema
	if additional == nil || additional.Value == nil {
		t.Fatalf("expected typed additionalProperties schema, got %#v", schema.AdditionalProperties)
	}
	assertSchemaIncludesType(t, additional.Value, want)
}

func assertObjectPropertyKeys(t *testing.T, schema *openapi3.Schema, names ...string) {
	t.Helper()

	for _, name := range names {
		if _, ok := schema.Properties[name]; !ok {
			t.Fatalf("expected property %q in schema, got %v", name, schema.Properties)
		}
	}
}

func contains(values []string, target string) bool {
	return slices.Contains(values, target)
}

func TestDocumentSchemaComponents(t *testing.T) {
	t.Parallel()
	doc, err := Document()
	if err != nil {
		t.Fatalf("Document() error = %v", err)
	}
	t.Run("Should publish named components and retain the existing component names", func(t *testing.T) {
		t.Parallel()
		for _, name := range []string{"SessionPayload", "WorkspacePayload", "SendPromptResultPayload", "SessionCompactRequest", "OptionalStringList"} {
			if doc.Components.Schemas[name] == nil {
				t.Errorf("missing component %s", name)
			}
		}
		baseline := readSchemaBaseline(t)
		for name := range baseline["components"].(map[string]any)["schemas"].(map[string]any) {
			if doc.Components.Schemas[name] == nil {
				t.Errorf("removed existing component %s", name)
			}
		}
		for _, target := range []struct{ path, method, status string }{
			{"/api/sessions", http.MethodPost, "201"},
			{"/api/workspaces/{workspace_id}/sessions/{session_id}", http.MethodGet, "200"},
		} {
			body := operationFor(t, doc, target.path, target.method).Responses.Value(target.status).Value.Content["application/json"].Schema
			if body.Ref == "" {
				t.Errorf("%s response must reference its named response component", target.path)
			}
			if body.Value.Properties["session"].Ref != "#/components/schemas/SessionPayload" {
				t.Errorf("%s session payload was inlined", target.path)
			}
		}
	})
	t.Run("Should pass the OpenAPI and generator compatibility checks", func(t *testing.T) {
		t.Parallel()
		if err := doc.Validate(t.Context()); err != nil {
			t.Fatalf("validate: %v", err)
		}
		if err := lintDocumentSchemas(doc); err != nil {
			t.Fatalf("lint: %v", err)
		}
	})
	t.Run("Should describe both prompt outcomes and experimental app token operations", func(t *testing.T) {
		t.Parallel()
		prompt := operationFor(t, doc, "/api/workspaces/{workspace_id}/sessions/{session_id}/prompt", http.MethodPost)
		content := prompt.Responses.Status(200).Value.Content
		if content["application/json"] == nil || content[specContentTypeEventStream] == nil {
			t.Fatal("prompt lacks JSON or event-stream media")
		}
		header := prompt.Responses.Status(200).Value.Headers["x-vercel-ai-ui-message-stream"]
		if header == nil || header.Value == nil {
			t.Fatal("prompt lacks stream version header")
		}
		assertEnumValues(t, header.Value.Schema.Value, "v1")
		if prompt.Extensions["x-compozy-stream"] != "ai-sdk-ui-message-stream-v1" {
			t.Fatal("prompt lacks stream protocol marker")
		}
		for _, operation := range appTokenOperations() {
			rendered := operationFor(t, doc, operation.Path, operation.Method)
			if rendered.Extensions["x-stability"] != schemaStabilityExperimental {
				t.Errorf("%s lacks experimental marker", operation.OperationID)
			}
			if !reflect.DeepEqual(
				rendered.Extensions["x-compozy-transports"],
				[]Transport{TransportHTTP, TransportUDS},
			) {
				t.Errorf("%s transport mismatch", operation.OperationID)
			}
		}
	})
}

func TestDocumentSchemaLint(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, suffix string
		schema       *openapi3.Schema
		named        bool
	}{
		{
			name:   "Should reject an OpenAPI 3.1 type array",
			suffix: "/type",
			schema: &openapi3.Schema{Type: &openapi3.Types{"string", "array"}},
		},
		{
			name:   "Should reject integer bounds outside int64",
			suffix: "/maximum",
			schema: openapi3.NewIntegerSchema().WithMax(18446744073709551615),
		},
		{
			name:   "Should reject an inline named component",
			suffix: "named struct SessionPayload",
			schema: openapi3.NewObjectSchema(),
			named:  true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			names := make(map[*openapi3.Schema]string)
			if tc.named {
				tc.schema.Origin = &openapi3.Origin{Key: &openapi3.Location{Name: "SessionPayload"}}
			}
			err := lintSchemaRef(&openapi3.SchemaRef{Value: tc.schema}, "#/fixture/schema", names, false)
			if err == nil || !strings.Contains(err.Error(), "#/fixture/schema") ||
				!strings.Contains(err.Error(), tc.suffix) {
				t.Fatalf("lint error = %v", err)
			}
		})
	}
}

func TestDocumentPreservesSchemaSemantics(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve every existing request response and component schema", func(t *testing.T) {
		t.Parallel()
		baseline := readSchemaBaseline(t)
		correctLegacySessionStreamSchemas(t, baseline)
		applyApprovedAdditiveSchemaChanges(t, baseline)
		correctProviderSettingsCommandAuthorship(t, baseline)
		addProviderUsageSettingsSection(t, baseline)
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
		oldComponents := baseline["components"].(map[string]any)["schemas"].(map[string]any)
		newComponents := current["components"].(map[string]any)["schemas"].(map[string]any)
		compareDocumentSchemas(t, baseline["paths"], current["paths"], "#/paths", oldComponents, newComponents)
		for name, oldSchema := range oldComponents {
			compareSchemaContract(
				t,
				oldSchema,
				newComponents[name],
				"#/components/schemas/"+name,
				oldComponents,
				newComponents,
				make(map[string]bool),
			)
		}
	})
}

func readSchemaBaseline(t *testing.T) map[string]any {
	t.Helper()
	compressed, err := os.ReadFile("testdata/openapi-pre-components.json.gz")
	if err != nil {
		t.Fatalf("read schema baseline: %v", err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("open schema baseline: %v", err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read compressed schema baseline: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close schema baseline: %v", err)
	}
	var baseline map[string]any
	if err := json.Unmarshal(data, &baseline); err != nil {
		t.Fatalf("decode schema baseline: %v", err)
	}
	return baseline
}

func compareDocumentSchemas(
	t *testing.T,
	oldValue, newValue any,
	pointer string,
	oldComponents, newComponents map[string]any,
) {
	t.Helper()
	switch old := oldValue.(type) {
	case map[string]any:
		next, ok := newValue.(map[string]any)
		if !ok {
			t.Fatalf("%s: missing document node", pointer)
		}
		for key, value := range old {
			if key == "schema" {
				compareSchemaContract(
					t,
					value,
					next[key],
					pointer+"/schema",
					oldComponents,
					newComponents,
					make(map[string]bool),
				)
			} else if key != "example" && key != "examples" {
				compareDocumentSchemas(
					t,
					value,
					next[key],
					pointer+"/"+schemaPointerEscape(key),
					oldComponents,
					newComponents,
				)
			}
		}
	case []any:
		next, ok := newValue.([]any)
		if !ok || len(next) != len(old) {
			t.Fatalf("%s: document sequence differs", pointer)
		}
		for i, value := range old {
			compareDocumentSchemas(t, value, next[i], pointer, oldComponents, newComponents)
		}
	}
}

func compareSchemaContract(
	t *testing.T,
	oldValue, newValue any,
	pointer string,
	oldComponents, newComponents map[string]any,
	active map[string]bool,
) {
	t.Helper()
	old, oldRef := normalizeSchemaNode(oldValue, oldComponents)
	next, newRef := normalizeSchemaNode(newValue, newComponents)
	if oldRef != "" && newRef != "" {
		key := oldRef + "|" + newRef
		if active[key] {
			return
		}
		active[key] = true
		defer delete(active, key)
	}
	switch expected := old.(type) {
	case map[string]any:
		actual, ok := next.(map[string]any)
		if !ok {
			t.Fatalf("%s: expected schema object, got %T", pointer, next)
		}
		if len(actual) != len(expected) {
			t.Fatalf("%s: schema keys differ: old=%v new=%v", pointer, schemaMapKeys(expected), schemaMapKeys(actual))
		}
		for key, value := range expected {
			compareSchemaContract(
				t,
				value,
				actual[key],
				pointer+"/"+schemaPointerEscape(key),
				oldComponents,
				newComponents,
				active,
			)
		}
	case []any:
		actual, ok := next.([]any)
		if !ok || len(actual) != len(expected) {
			t.Fatalf("%s: schema sequence differs", pointer)
		}
		for i, value := range expected {
			compareSchemaContract(t, value, actual[i], pointer, oldComponents, newComponents, active)
		}
	default:
		if !reflect.DeepEqual(old, next) {
			t.Fatalf("%s: schema value changed from %v to %v", pointer, old, next)
		}
	}
}

func normalizeSchemaNode(value any, components map[string]any) (any, string) {
	object, ok := value.(map[string]any)
	if !ok {
		return value, ""
	}
	reference, _ := object["$ref"].(string)
	if reference != "" {
		object = components[strings.TrimPrefix(reference, "#/components/schemas/")].(map[string]any)
	}
	normalized := make(map[string]any, len(object))
	maps.Copy(normalized, object)
	if alternatives, ok := normalized["oneOf"].([]any); ok && len(alternatives) == 2 {
		if null, ok := alternatives[1].(map[string]any); ok && reflect.DeepEqual(null["enum"], []any{nil}) {
			target, ref := normalizeSchemaNode(alternatives[0], components)
			normalized = target.(map[string]any)
			normalized["nullable"] = true
			if reference == "" {
				reference = ref
			}
		}
	}
	if types, ok := normalized["type"].([]any); ok && reflect.DeepEqual(types, []any{"array", "string"}) {
		delete(normalized, "type")
		delete(normalized, "items")
		normalized["oneOf"] = []any{
			map[string]any{"type": "string"},
			map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		}
	}
	if maximum, ok := normalized["maximum"].(float64); ok && maximum == float64(^uint64(0)) {
		delete(normalized, "maximum")
		normalized["format"] = "uint64"
	}
	return normalized, reference
}

func schemaMapKeys(value map[string]any) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func TestSchemaComponentNameCollisions(t *testing.T) {
	t.Parallel()
	t.Run("Should prefix colliding type names with their package names", func(t *testing.T) {
		t.Parallel()
		fixture := struct {
			Session sessionfixture.State `json:"session"`
			Task    taskfixture.State    `json:"task"`
		}{}
		names, err := componentNamesForTypes(reflect.TypeOf(fixture))
		if err != nil {
			t.Fatalf("component names: %v", err)
		}
		schemas := openapi3.Schemas{}
		root, err := schemaRefForValueWithNames(fixture, schemas, names)
		if err != nil {
			t.Fatalf("schema: %v", err)
		}
		for property, name := range map[string]string{"session": "SessionState", "task": "TaskState"} {
			if schemas[name] == nil || root.Value.Properties[property].Ref != "#/components/schemas/"+name {
				t.Errorf("%s did not reference %s", property, name)
			}
		}
	})
	t.Run("Should reject a remaining collision after package prefixing", func(t *testing.T) {
		t.Parallel()
		_, err := componentNamesForTypes(
			reflect.TypeFor[sessionfixture.State](),
			reflect.TypeFor[collisionfixture.State](),
		)
		if err == nil || !strings.Contains(err.Error(), "SessionState") {
			t.Fatalf("collision error = %v", err)
		}
	})
}

func streamVariantSchema(t *testing.T, schema *openapi3.Schema, name string) *openapi3.Schema {
	t.Helper()
	for _, variant := range schema.AnyOf {
		if variant.Ref == "#/components/schemas/"+name {
			return variant.Value
		}
	}
	t.Fatalf("missing raw stream frame component %s", name)
	return nil
}

func applyApprovedAdditiveSchemaChanges(t *testing.T, baseline map[string]any) {
	t.Helper()
	// #1005: lossless workspace archive metadata and typed busy blockers are additive.
	if addWorkspaceArchiveSchemas(baseline) == 0 {
		t.Fatal("no historical workspace/error schemas found")
	}
	for _, code := range []tools.ErrorCode{tools.ErrorCodeWorkspaceBusy, tools.ErrorCodeWorkspaceArchived} {
		if addToolErrorCode(baseline, string(code)) == 0 {
			t.Fatal("no historical tool error codes found")
		}
	}
	// #888: optional daemon host zone; existing config fields and required fields stay unchanged.
	automationProperties := baseline
	for _, key := range []string{
		"paths", "/api/settings/automation", "get", "responses", "200", "content", "application/json",
		"schema", "properties", "runtime", "properties",
	} {
		next, ok := automationProperties[key].(map[string]any)
		if !ok {
			t.Fatalf("approved automation schema addition: missing object %q", key)
		}
		automationProperties = next
	}
	if _, exists := automationProperties["host_timezone"]; exists {
		t.Fatal("host_timezone already exists in the historical baseline")
	}
	automationProperties["host_timezone"] = map[string]any{"type": "string"}
	// #864: optional workspace-config error details; required fields stay unchanged.
	if addWorkspaceConfigErrorSchema(baseline) == 0 {
		t.Fatal("approved workspace config error additions: no historical error schemas found")
	}
	if addDeclaredEnvironmentSchemas(baseline) == 0 {
		t.Fatal("approved environment additions: no historical declaration owners found")
	}
	// Spawn orchestration: optional worktree, inheritance, toolset, and first-prompt fields on the
	// agent spawn request and payload, plus the spawn_budget_exhausted tool reason code.
	addSpawnOrchestrationSchemas(t, baseline)
	// #908: tool error code for a failed stop-first archive; existing codes stay unchanged.
	if addToolErrorCode(baseline, string(tools.ErrorCodeSessionArchiveStopFailed)) == 0 {
		t.Fatal("approved tool error code addition: no historical tool error code enums found")
	}
	// #832: last_known has the existing optional RuntimeSelectionPayload contract.
	// #914: requested has the existing optional PromptRuntimeSelectionPayload contract.
	if addSessionRuntimeReadSchemas(baseline) == 0 {
		t.Fatal("approved session runtime addition: no historical runtime schemas found")
	}
	// #804: optional additive field; no required/type/removal change.
	// The pre-components fixture inlines SettingsUpdateRuntimePayload in the GET response.
	properties := baseline
	for _, key := range []string{
		"paths", "/api/settings/update", "get", "responses", "200", "content", "application/json",
		"schema", "properties", "runtime", "properties",
	} {
		next, ok := properties[key].(map[string]any)
		if !ok {
			t.Fatalf("approved update schema addition: missing object %q", key)
		}
		properties = next
	}
	if _, exists := properties["checked_at"]; exists {
		t.Fatal("approved update schema addition: checked_at already exists in the historical baseline")
	}
	properties["checked_at"] = map[string]any{"type": "string", "format": "date-time", "nullable": true}

	// #788: optional additive field; no required/type/removal change.
	paths := baseline["paths"].(map[string]any)
	prefix := "/api/workspaces/{workspace_id}/sessions/{session_id}"
	// #920: optional own-turn work aggregate; existing usage fields remain unchanged.
	usageProperties := paths
	for _, key := range []string{
		prefix + "/usage", "get", "responses", "200", "content", "application/json",
		"schema", "properties", "usage", "properties",
	} {
		next, ok := usageProperties[key].(map[string]any)
		if !ok {
			t.Fatalf("approved work-time addition: missing object %q", key)
		}
		usageProperties = next
	}
	if _, exists := usageProperties["work_time"]; exists {
		t.Fatal("approved work-time addition: work_time already exists in the historical baseline")
	}
	usageProperties["work_time"] = map[string]any{"oneOf": []any{
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"active_duration_ms":     map[string]any{"type": "integer", "format": "int64"},
				"turn_count":             map[string]any{"type": "integer", "format": "int64"},
				"last_completed_turn_id": map[string]any{"type": "string"},
			},
			"required": []any{"active_duration_ms", "turn_count"},
		},
		map[string]any{"enum": []any{nil}, "nullable": true},
	}}
	prompt := paths[prefix+"/prompt"].(map[string]any)["post"].(map[string]any)
	steer := paths[prefix+"/steer"].(map[string]any)["post"].(map[string]any)
	promptBody := prompt["requestBody"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)
	steerBody := steer["requestBody"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)
	promptProperties := promptBody["schema"].(map[string]any)["properties"].(map[string]any)
	steerProperties := steerBody["schema"].(map[string]any)["properties"].(map[string]any)
	steerProperties["attachments"] = promptProperties["attachments"]

	// #814: optional additive session-runtime defaults; no required/type/removal change.
	persona := paths["/api/settings/persona"].(map[string]any)
	personaConfigs := []map[string]any{
		persona["get"].(map[string]any)["responses"].(map[string]any)["200"].(map[string]any),
		persona["patch"].(map[string]any)["requestBody"].(map[string]any),
		persona["patch"].(map[string]any)["responses"].(map[string]any)["200"].(map[string]any),
	}
	for _, body := range personaConfigs {
		schema := body["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
		config, ok := schema["properties"].(map[string]any)["config"].(map[string]any)
		if !ok {
			continue
		}
		configProperties := config["properties"].(map[string]any)
		for _, field := range []string{"model", "reasoning_effort"} {
			if _, exists := configProperties[field]; exists {
				t.Fatalf("approved persona schema addition: %s already exists in the historical baseline", field)
			}
			configProperties[field] = map[string]any{"type": "string"}
		}
	}

	// Session monitors: optional additive `monitor_summary` on session payloads; no
	// required/type/removal change.
	addSessionMonitorSummary(t, baseline)
	addSessionsKanbanSchemas(t, baseline)

	// Composer mentions: optional additive `mentions` on queued-input payloads; no
	// required/type/removal change.
	addQueuedInputMentions(t, baseline)

	// #984: optional additive shell font preferences; no required/type/removal change.
	addShellAppearanceSchema(t, baseline)

	// #993: optional additive delegated-child liveness on subagent rows; no
	// required/type/removal change.
	if addSubagentChildActivity(baseline) == 0 {
		t.Fatal("approved subagent child activity addition: no historical subagent row schemas found")
	}
	// #993: optional urgency order on one parent's subagent list; the default
	// order and existing parameters keep their meaning.
	subagentList := paths["/api/workspaces/{workspace_id}/sessions/{session_id}/subagents"].(map[string]any)["get"].(map[string]any)
	subagentList["parameters"] = append(subagentList["parameters"].([]any), map[string]any{
		"in": "query", "name": "order", "schema": map[string]any{"type": "string", "enum": []any{"newest", "urgency"}},
	})

	// #813: optional opt-in query parameters on worktree cleanup, inserted before
	// the trailing profile selector; existing parameters keep their meaning.
	boolQuery := func(name string) map[string]any {
		return map[string]any{"in": "query", "name": name, "schema": map[string]any{"type": "boolean"}}
	}
	worktreePath := "/api/workspaces/{workspace_id}/worktrees/{worktree_id}"
	for _, change := range []struct {
		path, method string
		added        []any
	}{
		{worktreePath, "delete", []any{boolQuery("delete_branch")}},
		{worktreePath + "/dismiss", "post", []any{boolQuery("delete_branch"), boolQuery("force")}},
		// #908: optional stop-first opt-in on session archive.
		{"/api/workspaces/{workspace_id}/sessions/{session_id}/archive", "post", []any{boolQuery("stop")}},
	} {
		operation := paths[change.path].(map[string]any)[change.method].(map[string]any)
		parameters := operation["parameters"].([]any)
		last := len(parameters) - 1
		if parameters[last].(map[string]any)["name"] != "profile" {
			t.Fatalf(
				"approved worktree cleanup addition: %s %s no longer ends with profile",
				change.method, change.path,
			)
		}
		operation["parameters"] = append(
			append(append([]any{}, parameters[:last]...), change.added...),
			parameters[last],
		)
	}

	// #953: optional human-readable reason on worktree removal refusals; the
	// code and risk inventory keep their meaning.
	refusalProperties := paths[worktreePath].(map[string]any)
	for _, key := range []string{
		"delete", "responses", "409", "content", "application/json", "schema", "properties",
	} {
		refusalProperties = refusalProperties[key].(map[string]any)
	}
	if _, exists := refusalProperties["error"]; exists {
		t.Fatal("approved removal refusal addition: error already exists in the historical baseline")
	}
	refusalProperties["error"] = map[string]any{"type": "string"}
	riskSchema := refusalProperties["risk"].(map[string]any)
	riskSchema["properties"].(map[string]any)["leftover_files"] = map[string]any{"type": "integer"}
	if required, ok := riskSchema["required"].([]any); ok {
		required = append(required, "leftover_files")
		slices.SortFunc(required, func(left, right any) int {
			return strings.Compare(left.(string), right.(string))
		})
		riskSchema["required"] = required
	}

	// All projects (AP BR-12): optional breadth parameters inserted right after the
	// existing workspace selector; existing parameters keep their meaning.
	workspaceIDs := map[string]any{
		"in": "query", "name": "workspace_ids",
		"schema": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
	}
	for _, change := range []struct {
		path, method string
		added        []any
	}{
		{"/api/notifications/attention", "get", []any{boolQuery("all_workspaces")}},
		{"/api/notifications/attention/acknowledge", "post", []any{boolQuery("all_workspaces")}},
		{"/api/observe/tasks/inbox", "get", []any{boolQuery("all_workspaces"), workspaceIDs}},
	} {
		operation := paths[change.path].(map[string]any)[change.method].(map[string]any)
		parameters := operation["parameters"].([]any)
		at := slices.IndexFunc(parameters, func(parameter any) bool {
			return parameter.(map[string]any)["name"] == "workspace"
		})
		if at < 0 {
			t.Fatalf("approved All projects addition: %s %s has no workspace parameter", change.method, change.path)
		}
		operation["parameters"] = slices.Concat(parameters[:at+1], change.added, parameters[at+1:])
	}
	// All projects: the owning workspace of each Home attention item (empty for User rows).
	attentionItem := paths["/api/observe/overview"].(map[string]any)
	for _, key := range []string{
		"get", "responses", "200", "content", "application/json", "schema", "properties", "overview",
		"properties", "attention", "properties", "items", "items",
	} {
		attentionItem = attentionItem[key].(map[string]any)
	}
	attentionItem["properties"].(map[string]any)["workspace_id"] = map[string]any{"type": "string"}
	attentionItem["required"] = append(attentionItem["required"].([]any), "workspace_id")
}

// Match historical generic error fields, including payloads that embed them.
func addWorkspaceConfigErrorSchema(value any) int {
	count := 0
	switch node := value.(type) {
	case []any:
		for _, child := range node {
			count += addWorkspaceConfigErrorSchema(child)
		}
	case map[string]any:
		for child := range maps.Values(node) {
			count += addWorkspaceConfigErrorSchema(child)
		}
		properties, ok := node["properties"].(map[string]any)
		if !ok || properties["code"] == nil || properties["current_turn_id"] == nil ||
			properties["details"] == nil || properties["diagnostic"] == nil || properties["error"] == nil {
			return count
		}
		properties["config_path"] = map[string]any{"type": "string"}
		properties["unknown_keys"] = map[string]any{
			"type": "array", "items": map[string]any{"type": "string"},
		}
		count++
	}
	return count
}

// The historical baseline inlines SessionRuntimePayload at every response site.
// Preserve its complete existing shape and add only the explicitly approved field.
func addDeclaredEnvironmentSchemas(value any) int {
	count := 0
	switch node := value.(type) {
	case []any:
		for _, child := range node {
			count += addDeclaredEnvironmentSchemas(child)
		}
	case map[string]any:
		for child := range maps.Values(node) {
			count += addDeclaredEnvironmentSchemas(child)
		}
		properties, ok := node["properties"].(map[string]any)
		if !ok {
			return count
		}
		runtime := properties["sources"] != nil && properties["provider"] != nil && properties["acp_options"] != nil
		declarations := properties["command"] != nil && properties["provider"] != nil &&
			properties["acp_options"] != nil
		if runtime || declarations || properties["credential_slots"] != nil {
			properties["session_env"] = map[string]any{
				"type": "object",
				"additionalProperties": map[string]any{
					"type":     "object",
					"required": []any{"ref", "required"},
					"properties": map[string]any{
						"ref":      map[string]any{"type": "string"},
						"required": map[string]any{"type": "boolean"},
					},
				},
			}
			count++
		}
		providerDetail := properties["auth_status"] != nil && properties["runtime_strategy"] != nil &&
			properties["display_name"] != nil
		if runtime || providerDetail || properties["command_available"] != nil && properties["credentials"] != nil {
			properties["session_env_status"] = map[string]any{
				"type": "array", "items": map[string]any{
					"type": "object", "required": []any{"name", "present", "required", "secret_ref", "target_env"},
					"properties": map[string]any{
						"name": map[string]any{"type": "string"}, "target_env": map[string]any{"type": "string"},
						"secret_ref": map[string]any{"type": "string"}, "kind": map[string]any{"type": "string"},
						"source": map[string]any{"type": "string"}, "required": map[string]any{"type": "boolean"},
						"present": map[string]any{"type": "boolean"},
					},
				},
			}
			count++
		}
	}
	return count
}

func addSpawnOrchestrationSchemas(t *testing.T, baseline map[string]any) {
	t.Helper()
	components := baseline["components"].(map[string]any)["schemas"].(map[string]any)
	stringSchema := func() map[string]any { return map[string]any{"type": "string"} }
	added := map[string]map[string]any{
		"SpawnNewWorktreeRequest": {
			"type": "object",
			"properties": map[string]any{
				"name": stringSchema(), "base_ref": stringSchema(), "branch": stringSchema(),
			},
		},
		"SpawnWorktreePayload": {
			"type": "object",
			"properties": map[string]any{
				"id": stringSchema(), "name": stringSchema(), "branch": stringSchema(), "path": stringSchema(),
				"base_sha": stringSchema(), "created": map[string]any{"type": "boolean"},
			},
			"required": []any{"branch", "created", "id", "name", "path"},
		},
		// The request's permission lists become optional so inherit_permissions can omit them.
		"SpawnPermissionRequestPayload": {
			"type": "object",
			"properties": map[string]any{
				"tools":           map[string]any{"type": "array", "items": stringSchema()},
				"skills":          map[string]any{"type": "array", "items": stringSchema()},
				"mcp_servers":     map[string]any{"type": "array", "items": stringSchema()},
				"workspace_paths": map[string]any{"type": "array", "items": stringSchema()},
			},
		},
		"SpawnPromptPayload": {
			"type": "object",
			"properties": map[string]any{
				"message_id": stringSchema(), "status": stringSchema(),
				"reply_watch_id": stringSchema(), "reply_watch_state": stringSchema(), "error": stringSchema(),
			},
			"required": []any{"message_id", "status"},
		},
	}
	for name, schema := range added {
		if _, exists := components[name]; exists {
			t.Fatalf("approved spawn orchestration addition: %s already exists in the historical baseline", name)
		}
		components[name] = schema
	}
	nullableRef := func(name string) map[string]any {
		return map[string]any{"oneOf": []any{
			map[string]any{"$ref": "#/components/schemas/" + name},
			map[string]any{"enum": []any{nil}, "nullable": true},
		}}
	}
	requests, payloads, enums := 0, 0, 0
	var walk func(any)
	walk = func(value any) {
		switch node := value.(type) {
		case []any:
			for _, child := range node {
				walk(child)
			}
		case map[string]any:
			for key, child := range node {
				if values, ok := child.([]any); ok && slices.Contains(values, any(string(tools.ReasonConflictedID))) {
					node[key] = insertReasonCode(values, string(tools.ReasonSpawnBudgetExhausted))
					enums++
					continue
				}
				walk(child)
			}
			properties, ok := node["properties"].(map[string]any)
			if !ok {
				return
			}
			if properties["ttl_seconds"] != nil && properties["spawn_role"] != nil && properties["agent_name"] != nil {
				properties["inherit_permissions"] = map[string]any{"type": "boolean"}
				properties["toolsets"] = map[string]any{"type": "array", "items": stringSchema()}
				properties["worktree"] = stringSchema()
				properties["new_worktree"] = nullableRef("SpawnNewWorktreeRequest")
				properties["prompt"] = stringSchema()
				properties["permissions"] = map[string]any{"$ref": "#/components/schemas/SpawnPermissionRequestPayload"}
				requests++
			}
			if properties["lineage"] != nil && properties["permissions"] != nil && properties["session"] != nil &&
				len(properties) == 3 {
				properties["worktree"] = nullableRef("SpawnWorktreePayload")
				properties["prompt"] = nullableRef("SpawnPromptPayload")
				payloads++
			}
		}
	}
	walk(baseline)
	if requests == 0 || payloads == 0 || enums == 0 {
		t.Fatalf("approved spawn orchestration additions: requests=%d payloads=%d enums=%d", requests, payloads, enums)
	}
}

// insertReasonCode adds code where the generator places it: sorted enums keep their order.
func insertReasonCode(values []any, code string) []any {
	codes := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			return append(slices.Clone(values), code)
		}
		codes = append(codes, text)
	}
	if !slices.IsSorted(codes) {
		return append(slices.Clone(values), code)
	}
	index, _ := slices.BinarySearch(codes, code)
	return slices.Insert(slices.Clone(values), index, any(code))
}

// addToolErrorCode inserts code into every tool error code enum, identified by
// the compaction_unsupported member only tool error codes carry.
func addToolErrorCode(value any, code string) int {
	count := 0
	switch node := value.(type) {
	case []any:
		for _, child := range node {
			count += addToolErrorCode(child, code)
		}
	case map[string]any:
		for key, child := range node {
			if values, ok := child.([]any); ok &&
				slices.Contains(values, any(string(tools.ErrorCodeCompactionUnsupported))) {
				node[key] = insertReasonCode(values, code)
				count++
				continue
			}
			count += addToolErrorCode(child, code)
		}
	}
	return count
}

func addSessionRuntimeReadSchemas(value any) int {
	count := 0
	switch node := value.(type) {
	case []any:
		for _, child := range node {
			count += addSessionRuntimeReadSchemas(child)
		}
	case map[string]any:
		for child := range maps.Values(node) {
			count += addSessionRuntimeReadSchemas(child)
		}
		properties, ok := node["properties"].(map[string]any)
		if !ok || properties["acp_session_id"] == nil || properties["selection_revision"] == nil {
			return count
		}
		if effective, exists := properties["effective"]; exists {
			properties["last_known"] = effective
			count++
		}
		if selected, exists := properties["selected"]; exists {
			properties["requested"] = selected
		}
	}
	return count
}

// Sessions Kanban DX: add only the declared response fields and query filters.
// Keep historical properties and required fields so the comparator still detects drift.
func addSessionsKanbanSchemas(t *testing.T, baseline map[string]any) {
	t.Helper()
	var additions map[string]any
	if err := json.Unmarshal([]byte(`{
		"SessionPullRequestChecksPayload": {
			"type":"object", "required":["failed","failed_names","passed","pending","state"],
			"properties": {
				"state":{"type":"string","enum":["passing","failing","pending","none","unknown"]},
				"passed":{"type":"integer"},"failed":{"type":"integer"},"pending":{"type":"integer"},
				"failed_names":{"type":"array","items":{"type":"string"}}
			}
		},
		"SessionPullRequestPayload": {
			"type":"object",
			"required":["checks","draft","head_sha","mergeable","number","provider","ready_to_merge","review_decision","state","url"],
			"properties": {
				"provider":{"type":"string"},"number":{"type":"integer"},"url":{"type":"string"},
				"state":{"type":"string","enum":["open","closed","merged"]},
				"draft":{"type":"boolean","nullable":true},"head_sha":{"type":"string"},
				"checks":{"$ref":"#/components/schemas/SessionPullRequestChecksPayload"},
				"review_decision":{"type":"string","enum":["approved","changes_requested","review_required","none","unknown"]},
				"mergeable":{"type":"string","enum":["mergeable","conflicting","unknown"]},
				"ready_to_merge":{"type":"boolean"}
			}
		},
		"SessionPullRequestLookupPayload": {
			"type":"object", "required":["cause","observed_at","stale","state"],
			"properties": {
				"state":{"type":"string","enum":["ok","pending","error","incomplete"]},
				"cause":{"type":"string"},"observed_at":{"type":"string","format":"date-time","nullable":true},
				"stale":{"type":"boolean"}
			}
		},
		"SessionBoardCountsPayload": {
			"type":"object", "required":["done","in_review","needs_you","working"],
			"properties": {
				"done":{"type":"integer"},"in_review":{"type":"integer"},
				"needs_you":{"type":"integer"},"working":{"type":"integer"}
			}
		},
		"SessionChildrenPayload": {
			"type":"object", "required":["done","in_review","needs_you","total","working"],
			"properties": {
				"done":{"type":"integer"},"in_review":{"type":"integer"},"needs_you":{"type":"integer"},
				"total":{"type":"integer"},"working":{"type":"integer"}
			}
		}
	}`), &additions); err != nil {
		t.Fatalf("decode approved Sessions Kanban schemas: %v", err)
	}
	components := baseline["components"].(map[string]any)["schemas"].(map[string]any)
	for name, schema := range additions {
		if components[name] != nil {
			t.Fatalf("approved Sessions Kanban schema %s already exists", name)
		}
		components[name] = schema
	}
	ref := func(name string) map[string]any {
		return map[string]any{"$ref": "#/components/schemas/" + name}
	}
	nullable := func(name string) map[string]any {
		return map[string]any{"oneOf": []any{ref(name), map[string]any{"enum": []any{nil}, "nullable": true}}}
	}
	enum := func(values ...string) map[string]any {
		items := make([]any, len(values))
		for i, value := range values {
			items[i] = value
		}
		return map[string]any{"type": "string", "enum": items}
	}
	badgeValues := enum(
		"running", "idle", "unhealthy", "hung", "waiting-for-auth", "waiting-for-input",
		"done", "stopped", "failed", "unknown", "needs-attention", "waiting",
	)["enum"]
	counts := make(map[string]int)
	var walk func(any)
	walk = func(value any) {
		switch node := value.(type) {
		case []any:
			for _, child := range node {
				walk(child)
			}
		case map[string]any:
			for child := range maps.Values(node) {
				walk(child)
			}
			properties, ok := node["properties"].(map[string]any)
			if !ok {
				return
			}
			if properties["badge"] != nil && properties["attachable"] != nil {
				// The response now documents all existing badges plus the new waiting value.
				properties["badge"].(map[string]any)["enum"] = badgeValues
			}

			if properties["session_id"] != nil && properties["class"] != nil && properties["from"] != nil &&
				properties["to"] != nil {
				properties["from"].(map[string]any)["enum"] = badgeValues
				properties["to"].(map[string]any)["enum"] = badgeValues
			}

			switch {
			case properties["subagent_summary"] != nil && properties["badge"] != nil:
				properties["branch"] = map[string]any{"type": "string"}
				properties["parent_title"] = map[string]any{"type": "string"}
				properties["waiting_on"] = enum("sessions", "monitor")
				properties["board_column"] = enum("needs_you", "working", "in_review", "done")
				properties["children"] = nullable("SessionChildrenPayload")
				properties["pull_request"] = nullable("SessionPullRequestPayload")
				properties["pull_request_lookup"] = nullable("SessionPullRequestLookupPayload")

				node["required"] = append(
					node["required"].([]any),
					"board_column",
					"pull_request",
					"pull_request_lookup",
				)
				slices.SortFunc(
					node["required"].([]any),
					func(a, b any) int { return strings.Compare(a.(string), b.(string)) },
				)
				counts["session"]++
			case properties["terminal_approvals"] != nil && properties["finished"] != nil:
				properties["board"] = ref("SessionBoardCountsPayload")
				node["required"] = append(node["required"].([]any), "board")
				slices.SortFunc(
					node["required"].([]any),
					func(a, b any) int { return strings.Compare(a.(string), b.(string)) },
				)
				counts["facets"]++
			case properties["kind"] != nil && properties["ref"] != nil &&
				(properties["since"] != nil || properties["state"] != nil):
				kind := properties["kind"].(map[string]any)
				values, ok := kind["enum"].([]any)
				if ok && slices.Contains(values, any("awaiting_reply")) {
					kind["enum"] = append(values, "active_monitor")
					counts["work signal"]++
				}
			}
		}
	}
	walk(baseline)
	for _, name := range []string{"session", "facets", "work signal"} {
		if counts[name] == 0 {
			t.Fatalf("approved Sessions Kanban addition: no historical %s schemas", name)
		}
	}
	paths := baseline["paths"].(map[string]any)
	for _, path := range []string{"/api/sessions", "/api/sessions/facets", "/api/workspaces/{workspace_id}/sessions"} {
		operation := paths[path].(map[string]any)["get"].(map[string]any)
		parameters := operation["parameters"].([]any)
		var updated []any
		for _, parameter := range parameters {
			updated = append(updated, parameter)
			switch parameter.(map[string]any)["name"] {
			case "subagents":
				updated = append(
					updated,
					map[string]any{"in": "query", "name": "children", "schema": enum("include", "exclude")},
				)
			case "attention":
				if path != "/api/sessions/facets" {
					updated = append(
						updated,
						map[string]any{"in": "query", "name": "board_column", "schema": map[string]any{
							"type": "array", "items": map[string]any{"type": "string"},
						}},
					)
				}
			}
		}
		operation["parameters"] = updated
	}
	addSessionsKanbanRoutes(t, paths, nullable)
}

func addSessionsKanbanRoutes(t *testing.T, paths map[string]any, nullable func(string) map[string]any) {
	t.Helper()
	prefix := "/api/workspaces/{workspace_id}"
	plan := paths[prefix+"/worktrees/{worktree_id}/exit"].(map[string]any)["get"].(map[string]any)
	paths[prefix+"/worktrees/{worktree_id}/exit/prepare"] = map[string]any{"post": map[string]any{
		"parameters": plan["parameters"].([]any)[:2], "responses": plan["responses"],
	}}
	sessionRead := paths[prefix+"/sessions/{session_id}"].(map[string]any)["get"].(map[string]any)
	errorResponse := sessionRead["responses"].(map[string]any)["404"]
	errorSchema := errorResponse.(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	refreshError := maps.Clone(errorSchema)
	properties := maps.Clone(errorSchema["properties"].(map[string]any))
	properties["message"] = map[string]any{"type": "string"}
	properties["cause"] = map[string]any{"type": "string"}
	refreshError["properties"] = properties
	refreshError["required"] = append(slices.Clone(errorSchema["required"].([]any)), "message")
	slices.SortFunc(
		refreshError["required"].([]any),
		func(a, b any) int { return strings.Compare(a.(string), b.(string)) },
	)
	response := func(schema any) map[string]any {
		return map[string]any{"content": map[string]any{"application/json": map[string]any{"schema": schema}}}
	}
	paths["/api/sessions/{session_id}/pull-request/refresh"] = map[string]any{"post": map[string]any{
		"parameters": []any{
			map[string]any{
				"in":       "path",
				"name":     "session_id",
				"required": true,
				"schema":   map[string]any{"type": "string"},
			},
			map[string]any{"in": "query", "name": "profile", "schema": map[string]any{"type": "string"}},
		},
		"responses": map[string]any{
			"200": response(map[string]any{
				"type": "object", "required": []any{"branch", "pull_request", "pull_request_lookup", "session_id"},
				"properties": map[string]any{
					"session_id": map[string]any{"type": "string"}, "branch": map[string]any{"type": "string"},
					"pull_request":        nullable("SessionPullRequestPayload"),
					"pull_request_lookup": nullable("SessionPullRequestLookupPayload"),
				},
			}),
			"404": errorResponse, "409": response(refreshError), "502": response(refreshError),
		},
	}}
}

// addSessionMonitorSummary adds the approved nullable `monitor_summary` to every
// historical session payload schema (identified by subagent_summary + badge).
func addSessionMonitorSummary(t *testing.T, baseline map[string]any) {
	t.Helper()
	components := baseline["components"].(map[string]any)["schemas"].(map[string]any)
	if _, exists := components["SessionMonitorSummaryPayload"]; exists {
		t.Fatal("approved session monitor addition: summary schema already exists in the historical baseline")
	}
	components["SessionMonitorSummaryPayload"] = map[string]any{
		"type":       "object",
		"properties": map[string]any{"active": map[string]any{"type": "integer"}},
		"required":   []any{"active"},
	}
	var walk func(any) int
	walk = func(value any) int {
		count := 0
		switch node := value.(type) {
		case []any:
			for _, child := range node {
				count += walk(child)
			}
		case map[string]any:
			for child := range maps.Values(node) {
				count += walk(child)
			}
			properties, ok := node["properties"].(map[string]any)
			if !ok || properties["subagent_summary"] == nil || properties["badge"] == nil {
				return count
			}
			properties["monitor_summary"] = map[string]any{"oneOf": []any{
				map[string]any{"$ref": "#/components/schemas/SessionMonitorSummaryPayload"},
				map[string]any{"enum": []any{nil}, "nullable": true},
			}}
			count++
		}
		return count
	}
	if walk(baseline) == 0 {
		t.Fatal("approved session monitor addition: no historical session payload schemas found")
	}
}

// addQueuedInputMentions adds the approved `mentions` array to every historical
// queued-input payload schema (identified by skill_invocations + queue_generation).
func addQueuedInputMentions(t *testing.T, baseline map[string]any) {
	t.Helper()
	mentionKind := map[string]any{"type": "string", "enum": []any{"file", "folder", "session"}}
	item := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"kind":   mentionKind,
			"token":  map[string]any{"type": "string"},
			"target": map[string]any{"type": "string"},
			"label":  map[string]any{"type": "string"},
			"start":  map[string]any{"type": "integer"},
			"end":    map[string]any{"type": "integer"},
		},
		"required": []any{"end", "kind", "label", "start", "target", "token"},
	}
	added := 0
	var walk func(node any)
	walk = func(node any) {
		switch value := node.(type) {
		case map[string]any:
			if properties, ok := value["properties"].(map[string]any); ok {
				_, skills := properties["skill_invocations"]
				_, generation := properties["queue_generation"]
				if _, exists := properties["mentions"]; skills && generation && !exists {
					properties["mentions"] = map[string]any{"type": "array", "items": item}
					added++
				}
			}
			for _, child := range value {
				walk(child)
			}
		case []any:
			for _, child := range value {
				walk(child)
			}
		}
	}
	walk(baseline)
	if added == 0 {
		t.Fatal("approved mentions addition: no queued-input payload schema found in the historical baseline")
	}
}

// correctProviderSettingsCommandAuthorship records the read-payload addition the baseline predates.
func correctProviderSettingsCommandAuthorship(t *testing.T, baseline map[string]any) {
	t.Helper()
	paths := baseline["paths"].(map[string]any)
	addInherited := func(settings any) {
		properties := settings.(map[string]any)["properties"].(map[string]any)
		if _, ok := properties["command_inherited"]; !ok {
			properties["command_inherited"] = map[string]any{"type": "boolean"}
		}
	}
	listItems := paths["/api/settings/providers"].(map[string]any)["get"].(map[string]any)["responses"].(map[string]any)["200"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)["properties"].(map[string]any)["providers"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
	addInherited(listItems["settings"])
	addInherited(listItems["fallback"].(map[string]any)["properties"].(map[string]any)["settings"])
	provider := paths["/api/settings/providers/{name}"].(map[string]any)["get"].(map[string]any)["responses"].(map[string]any)["200"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)["properties"].(map[string]any)["provider"].(map[string]any)["properties"].(map[string]any)
	addInherited(provider["settings"])
	addInherited(provider["fallback"].(map[string]any)["properties"].(map[string]any)["settings"])
}

func correctLegacySessionStreamSchemas(t *testing.T, baseline map[string]any) {
	t.Helper()
	// The old pseudo-envelope described fields that the SSE writer never puts on the wire.
	for _, path := range []string{"/api/sessions/{session_id}/stream", "/api/workspaces/{workspace_id}/sessions/{session_id}/stream"} {
		operation := baseline["paths"].(map[string]any)[path].(map[string]any)["get"].(map[string]any)
		responses := operation["responses"].(map[string]any)
		media := responses["200"].(map[string]any)["content"].(map[string]any)[specContentTypeEventStream].(map[string]any)
		properties := media["schema"].(map[string]any)["properties"].(map[string]any)
		var variants []any
		for _, field := range []string{"subagents_snapshot", "subagent_updated", "session_usage_changed", "consumer_degraded", "raw", "transcript_snapshot", "transcript_delta", "goal_snapshot_changed", "session_commands_changed"} {
			schema := properties[field].(map[string]any)
			delete(schema, "nullable")
			variants = append(variants, schema)
		}
		failure := responses["500"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"]
		media["schema"] = map[string]any{"anyOf": append(variants, failure)}
	}
}

func TestSessionStreamRawFrameContract(t *testing.T) {
	t.Parallel()
	t.Run("Should validate the raw transcript frame emitted by the session stream writer", func(t *testing.T) {
		t.Parallel()
		document, err := Document()
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{"/api/sessions/{session_id}/stream", "/api/workspaces/{workspace_id}/sessions/{session_id}/stream"} {
			schema := responseSchema(
				t,
				operationFor(t, document, path, http.MethodGet),
				200,
				specContentTypeEventStream,
			)
			if len(schema.AnyOf) != 10 {
				t.Fatalf("%s frame alternatives = %d, want 10", path, len(schema.AnyOf))
			}
			snapshot := streamVariantSchema(t, schema, "TranscriptSnapshotPayload")
			frame := map[string]any{
				"session_id":   "session-1",
				"epoch":        float64(1),
				"generation":   float64(1),
				"entries":      []any{},
				"max_sequence": float64(0),
				"has_older":    false,
				"reset":        false,
			}
			if err := snapshot.VisitJSON(frame); err != nil {
				t.Fatalf("raw transcript snapshot invalid: %v", err)
			}
			if err := schema.VisitJSON(frame); err != nil {
				t.Fatalf("raw session stream frame invalid: %v", err)
			}
			if err := snapshot.VisitJSON(map[string]any{"transcript_snapshot": frame}); err == nil {
				t.Fatal("pseudo-envelope must not validate as a raw transcript snapshot")
			}
		}
	})
}

func resolvedTestSchema(schema *openapi3.Schema) *openapi3.Schema {
	if len(schema.OneOf) == 2 {
		nullable := schema.OneOf[1].Value
		if nullable != nil && len(nullable.Enum) == 1 && nullable.Enum[0] == nil && schema.OneOf[0].Value != nil {
			resolved := *schema.OneOf[0].Value
			resolved.Nullable = true
			return &resolved
		}
	}
	return schema
}

func schemaIncludesType(schema *openapi3.Schema, want string) bool {
	if schema.Type.Includes(want) {
		return true
	}
	for _, alternatives := range []openapi3.SchemaRefs{schema.OneOf, schema.AnyOf} {
		for _, alternative := range alternatives {
			if alternative.Value != nil && schemaIncludesType(alternative.Value, want) {
				return true
			}
		}
	}
	return false
}

// Provider usage adds one section and apply target without removing existing values.
func addProviderUsageSettingsSection(t *testing.T, baseline map[string]any) {
	t.Helper()
	changes := 0
	var walk func(any)
	walk = func(value any) {
		switch node := value.(type) {
		case map[string]any:
			if values, ok := node["enum"].([]any); ok && len(values) >= 12 && values[0] == "general" &&
				values[8] == "shell" &&
				values[9] == "observability" {
				next := append([]any{}, values[:9]...)
				next = append(next, "provider-usage")
				node["enum"] = append(next, values[9:]...)
				changes++
			}
			for _, child := range node {
				walk(child)
			}
		case []any:
			for _, child := range node {
				walk(child)
			}
		}
	}
	walk(baseline)
	if changes == 0 {
		t.Fatal("provider usage addition: historical settings enum not found")
	}
}

// #984: add the optional `appearance` object to every historical shell config schema.
func addSubagentChildActivity(node any) int {
	switch value := node.(type) {
	case map[string]any:
		count := 0
		if properties, ok := value["properties"].(map[string]any); ok &&
			properties["subagent_id"] != nil && properties["work_state"] != nil {
			properties["child_activity"] = map[string]any{
				"type": "string",
				"enum": []any{"working", "idle", "stopped"},
			}
			count++
		}
		for _, child := range value {
			count += addSubagentChildActivity(child)
		}
		return count
	case []any:
		count := 0
		for _, child := range value {
			count += addSubagentChildActivity(child)
		}
		return count
	default:
		return 0
	}
}

func addShellAppearanceSchema(t *testing.T, baseline map[string]any) {
	t.Helper()
	var appearance map[string]any
	if err := json.Unmarshal([]byte(`{"oneOf":[
		{"type":"object","additionalProperties":false,"properties":{
			"ui_font":{"type":"string","nullable":true,"maxLength":128},
			"mono_font":{"type":"string","nullable":true,"maxLength":128}
		}},
		{"enum":[null],"nullable":true}
	]}`), &appearance); err != nil {
		t.Fatalf("approved shell appearance addition: %v", err)
	}
	shell := baseline["paths"].(map[string]any)["/api/settings/shell"].(map[string]any)
	bodies := []map[string]any{
		shell["get"].(map[string]any)["responses"].(map[string]any)["200"].(map[string]any),
		shell["patch"].(map[string]any)["requestBody"].(map[string]any),
		shell["patch"].(map[string]any)["responses"].(map[string]any)["200"].(map[string]any),
	}
	for _, body := range bodies {
		schema := body["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
		config := schema["properties"].(map[string]any)["config"].(map[string]any)
		configProperties := config["properties"].(map[string]any)
		if _, exists := configProperties["appearance"]; exists {
			t.Fatal("approved shell appearance addition: appearance already exists in the historical baseline")
		}
		configProperties["appearance"] = appearance
	}
}

func addWorkspaceArchiveSchemas(value any) int {
	count := 0
	switch node := value.(type) {
	case []any:
		for _, child := range node {
			count += addWorkspaceArchiveSchemas(child)
		}
	case map[string]any:
		for child := range maps.Values(node) {
			count += addWorkspaceArchiveSchemas(child)
		}
		properties, ok := node["properties"].(map[string]any)
		if !ok {
			return count
		}
		if properties["id"] != nil && properties["root_dir"] != nil && properties["created_at"] != nil {
			properties["archived_at"] = map[string]any{"type": "string", "format": "date-time", "nullable": true}
			count++
		}
		if properties["error"] != nil && properties["current_turn_id"] != nil && properties["diagnostic"] != nil {
			properties["blockers"] = map[string]any{"type": "array", "items": map[string]any{
				"type": "object", "required": []any{"id", "kind", "name"}, "properties": map[string]any{
					"id": map[string]any{
						"type": "string",
					},
					"kind": map[string]any{"type": "string"},
					"name": map[string]any{"type": "string"},
				},
			}}
			count++
		}
	}
	return count
}
