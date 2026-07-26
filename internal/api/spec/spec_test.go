package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/tools"
	"github.com/getkin/kin-openapi/openapi3"
)

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
				assertRequired(t, propertySchema(t, listSessionsSchema, "page"), "has_more", "total", "limit")

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
					"provider",
					"state",
					"badge",
					"attachable",
					"created_at",
					"updated_at",
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
					"dream",
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
				assertResponseStatus(t, attachSession, 409)

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
				propertySchema(t, streamSchema, "raw")
				snapshotSchema := propertySchema(t, streamSchema, "transcript_snapshot")
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
				deltaSchema := propertySchema(t, streamSchema, "transcript_delta")
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
				propertySchema(t, streamSchema, "session_stopped")
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
			name: "ShouldDescribeNetworkSubscriptionFilters",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				listSubscriptions := operationFor(
					t,
					doc,
					"/api/workspaces/{workspace_id}/network/channels/{channel}/subscriptions",
					"GET",
				)
				assertParameter(t, listSubscriptions, "session_id", openapi3.ParameterInQuery, false)
				assertParameter(t, listSubscriptions, "thread_id", openapi3.ParameterInQuery, false)
				assertParameter(t, listSubscriptions, "limit", openapi3.ParameterInQuery, false)
			},
		},
		{
			name: "Should describe the network send conversation contract",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				send := operationFor(t, doc, "/api/workspaces/{workspace_id}/network/send", "POST")
				request := jsonRequestSchema(t, send)
				assertSchemaHasAdditionalProperties(t, request, false)
				assertEnumValues(
					t,
					propertySchema(t, request, "kind"),
					"greet",
					"whois",
					"say",
					"capability",
					"receipt",
					"trace",
				)
				assertEnumValues(t, propertySchema(t, request, "surface"), "thread", "direct")

				threadID := propertySchema(t, request, "thread_id")
				const threadIDPattern = `^thread_[a-z0-9][a-z0-9_-]{2,95}$`
				if threadID.Pattern != threadIDPattern {
					t.Fatalf("network send thread_id pattern = %q, want %q", threadID.Pattern, threadIDPattern)
				}
				if !strings.Contains(threadID.Description, "first valid send creates the public thread") {
					t.Fatalf(
						"network send thread_id description = %q, want implicit-create contract",
						threadID.Description,
					)
				}
				directID := propertySchema(t, request, "direct_id")
				const directIDPattern = `^direct_[a-f0-9]{32}$`
				if directID.Pattern != directIDPattern {
					t.Fatalf("network send direct_id pattern = %q, want %q", directID.Pattern, directIDPattern)
				}
				body := propertySchema(t, request, "body")
				assertSchemaIncludesType(t, body, openapi3.TypeObject)
				if !strings.Contains(body.Description, "say requires a non-empty text field") {
					t.Fatalf("network send body description = %q, want say body contract", body.Description)
				}
				workID := propertySchema(t, request, "work_id")
				if !strings.Contains(workID.Description, "Required for capability, receipt, and trace") {
					t.Fatalf("network send work_id description = %q, want required-kind contract", workID.Description)
				}
				target := propertySchema(t, request, "to")
				if !strings.Contains(target.Description, "Required for capability and for say carrying work_id") {
					t.Fatalf("network send to description = %q, want lifecycle target contract", target.Description)
				}
				if !strings.Contains(request.Description, "greet and whois omit conversation and work fields") {
					t.Fatalf(
						"network send request description = %q, want discovery omission contract",
						request.Description,
					)
				}

				upsertSubscription := operationFor(
					t,
					doc,
					"/api/workspaces/{workspace_id}/network/channels/{channel}/subscriptions",
					"PUT",
				)
				assertSchemaHasAdditionalProperties(t, jsonRequestSchema(t, upsertSubscription), false)
				promoteThread := operationFor(
					t,
					doc,
					"/api/workspaces/{workspace_id}/network/channels/{channel}/threads/{thread_id}/promote-task",
					"POST",
				)
				assertSchemaHasAdditionalProperties(t, jsonRequestSchema(t, promoteThread), false)
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
					"provider",
					"name",
					"workspace",
					"workspace_path",
				)
				assertEnumValues(
					t,
					propertySchema(t, createSessionSchema, "reasoning_effort"),
					"none",
					"minimal",
					"low",
					"medium",
					"high",
					"xhigh",
					"max",
				)
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

				update := operationFor(t, doc, "/api/agents/{name}", "PUT")
				assertParameter(t, update, "name", openapi3.ParameterInPath, true)
				assertResponseStatus(t, update, 409)
				updateSchema := jsonRequestSchema(t, update)
				assertRequired(t, updateSchema, "agent", "expected_digest")
				updateAgentSchema := propertySchema(t, updateSchema, "agent")
				assertPropertyAbsent(t, updateAgentSchema, "mcp_servers")

				deleteAgent := operationFor(t, doc, "/api/agents/{name}", "DELETE")
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

				curate := operationFor(
					t,
					doc,
					"/api/model-catalog/providers/{provider_id}/models/curate",
					"POST",
				)
				assertParameter(t, curate, "provider_id", openapi3.ParameterInPath, true)
				assertRequired(t, jsonRequestSchema(t, curate), "model_id")
				assertEnumValues(
					t,
					propertySchema(t, jsonRequestSchema(t, curate), "default_effort"),
					"none",
					"minimal",
					"low",
					"medium",
					"high",
					"xhigh",
					"max",
				)
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
			name: "ShouldDescribeMemoryV2PublicContractAndHardCuts",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				writeMemory := operationFor(t, doc, "/api/memory", "POST")
				writeMemorySchema := jsonRequestSchema(t, writeMemory)
				assertRequired(t, writeMemorySchema, "scope", "type", "name", "content")
				assertNotRequired(
					t,
					writeMemorySchema,
					"workspace_id",
					"agent_name",
					"agent_tier",
					"origin",
					"idempotency_key",
					"dry_run",
				)
				assertEnumValues(t, propertySchema(t, writeMemorySchema, "scope"), "global", "workspace", "agent")
				assertEnumValues(t, propertySchema(t, writeMemorySchema, "agent_tier"), "workspace", "global")
				assertEnumValues(
					t,
					propertySchema(t, writeMemorySchema, "type"),
					"user",
					"feedback",
					"project",
					"reference",
				)
				assertEnumValues(t, propertySchema(t, writeMemorySchema, "origin"),
					"cli",
					"http",
					"uds",
					"tool",
					"extractor",
					"dreaming",
					"file",
					"provider",
				)

				editMemory := operationFor(t, doc, "/api/memory/{filename}", "PATCH")
				editMemorySchema := jsonRequestSchema(t, editMemory)
				assertRequired(t, editMemorySchema, "content")
				assertNotRequired(t, editMemorySchema, "workspace_id", "agent_name", "agent_tier")

				readMemory := operationFor(t, doc, "/api/memory/{filename}", "GET")
				readMemorySchema := jsonResponseSchema(t, readMemory, 200)
				assertRequired(t, readMemorySchema, "memory")
				memorySchema := propertySchema(t, readMemorySchema, "memory")
				assertRequired(t, memorySchema, "summary", "content")
				summarySchema := propertySchema(t, memorySchema, "summary")
				assertEnumValues(t, propertySchema(t, summarySchema, "scope"), "global", "workspace", "agent")
				assertEnumValues(t, propertySchema(t, summarySchema, "agent_tier"), "workspace", "global")

				searchMemory := operationFor(t, doc, "/api/memory/search", "POST")
				searchSchema := jsonRequestSchema(t, searchMemory)
				assertRequired(t, searchSchema, "query_text")
				assertNotRequired(t, searchSchema, "include_system", "include_already_surfaced", "agent_tier")

				decision := operationFor(t, doc, "/api/memory/decisions/{decision_id}", "GET")
				decisionSchema := propertySchema(t, jsonResponseSchema(t, decision, 200), "decision")
				assertRequired(
					t,
					decisionSchema,
					"id",
					"candidate_hash",
					"op",
					"scope",
					"frontmatter",
					"confidence",
					"source",
					"decided_at",
				)
				assertEnumValues(
					t,
					propertySchema(t, decisionSchema, "op"),
					"noop",
					"add",
					"update",
					"delete",
					"reject",
				)
				assertPropertyAbsent(t, decisionSchema, "post_content")
				assertPropertyAbsent(t, decisionSchema, "prior_content")

				errorSchema := jsonResponseSchema(t, writeMemory, 422)
				assertRequired(t, errorSchema, "code", "message")
				assertNotRequired(t, errorSchema, "details")
				assertPropertyAbsent(t, errorSchema, "error")

				assertOperationAbsent(t, doc, "/api/memory/{filename}", "PUT")
				assertOperationAbsent(t, doc, "/api/memory/search", "GET")
				assertOperationAbsent(t, doc, "/api/memory/consolidate", "POST")
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
				assertEnumValues(t, propertySchema(t, createJobSchema, "scope"), "global", "workspace")

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
				assertEnumValues(t, propertySchema(t, createTriggerSchema, "scope"), "global", "workspace")

				healthOperation := operationFor(t, doc, "/api/status", "GET")
				healthSchema := jsonResponseSchema(t, healthOperation, 200)
				assertRequired(t, healthSchema, "daemon", "health", "memory", "automation", "config", "log_tail")

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
				assertParameter(t, webhookOperation, "X-AGH-Webhook-Timestamp", openapi3.ParameterInHeader, true)
				assertParameter(t, webhookOperation, "X-AGH-Webhook-Signature", openapi3.ParameterInHeader, true)

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
				assertRequired(t, skillSchema, "activation")
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
				assertParameter(t, getSkill, "workspace", openapi3.ParameterInQuery, false)
				assertParameter(t, getSkill, "for_agent", openapi3.ParameterInQuery, false)

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
			name: "ShouldDescribeBridgeCreateRequiredFieldsAndEnums",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				createBridge := operationFor(t, doc, "/api/bridges", "POST")
				createBridgeSchema := jsonRequestSchema(t, createBridge)
				assertRequired(
					t,
					createBridgeSchema,
					"scope",
					"platform",
					"extension_name",
					"display_name",
					"enabled",
					"routing_policy",
				)
				assertNotRequired(
					t,
					createBridgeSchema,
					"workspace_id",
					"dm_policy",
					"provider_config",
					"delivery_defaults",
				)
				assertEnumValues(t, propertySchema(t, createBridgeSchema, "scope"), "global", "workspace")
				assertEnumValues(t, propertySchema(t, createBridgeSchema, "dm_policy"), "open", "allowlist", "pairing")

				providerConfigSchema := propertySchema(t, createBridgeSchema, "provider_config")
				assertSchemaIncludesType(t, providerConfigSchema, openapi3.TypeObject)
				assertSchemaHasAdditionalProperties(t, providerConfigSchema, true)

				deliveryDefaultsSchema := propertySchema(t, createBridgeSchema, "delivery_defaults")
				assertSchemaIncludesType(t, deliveryDefaultsSchema, openapi3.TypeObject)
				assertSchemaHasAdditionalProperties(t, deliveryDefaultsSchema, true)
				assertAdditionalPropertiesSchemaIncludesType(t, deliveryDefaultsSchema, openapi3.TypeString)
				if got := deliveryDefaultsSchema.Extensions[openapiTSWidenAdditionalPropertiesExtension]; got != true {
					t.Fatalf(
						"delivery defaults %s = %#v, want true",
						openapiTSWidenAdditionalPropertiesExtension,
						got,
					)
				}
				assertEnumValues(t, propertySchema(t, deliveryDefaultsSchema, "mode"), "direct-send", "reply")

				progressSchema := propertySchema(t, deliveryDefaultsSchema, "progress")
				assertRequired(t, progressSchema, "tool_progress", "grouping")
				assertNotRequired(t, progressSchema, "typing", "reactions")
				assertSchemaHasAdditionalProperties(t, progressSchema, false)
				assertEnumValues(t, propertySchema(t, progressSchema, "tool_progress"), "off", "new", "all", "verbose")
				assertEnumValues(t, propertySchema(t, progressSchema, "grouping"), "accumulate", "separate")
				assertSchemaIncludesType(t, propertySchema(t, progressSchema, "typing"), openapi3.TypeBoolean)
				assertSchemaIncludesType(t, propertySchema(t, progressSchema, "reactions"), openapi3.TypeBoolean)

				updateBridge := operationFor(t, doc, "/api/bridges/{id}", "PATCH")
				updateDefaultsSchema := propertySchema(t, jsonRequestSchema(t, updateBridge), "delivery_defaults")
				updateProgressSchema := propertySchema(t, updateDefaultsSchema, "progress")
				assertEnumValues(
					t,
					propertySchema(t, updateProgressSchema, "tool_progress"),
					"off",
					"new",
					"all",
					"verbose",
				)
			},
		},
		{
			name: "ShouldDescribeBridgeTestDeliveryTypedTargetShape",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				testDelivery := operationFor(t, doc, "/api/bridges/{id}/test-delivery", "POST")
				testDeliverySchema := jsonRequestSchema(t, testDelivery)
				assertRequired(t, testDeliverySchema, "target")
				assertNotRequired(t, testDeliverySchema, "message")

				targetSchema := propertySchema(t, testDeliverySchema, "target")
				assertNotRequired(t, targetSchema, "bridge_instance_id", "peer_id", "thread_id", "group_id", "mode")
				assertEnumValues(t, propertySchema(t, targetSchema, "mode"), "direct-send", "reply")

				responseSchema := jsonResponseSchema(t, testDelivery, 200)
				assertRequired(t, responseSchema, "status", "delivery_target")
			},
		},
		{
			name: "ShouldDescribeBridgeProvidersAndHealthTelemetry",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				listBridges := operationFor(t, doc, "/api/bridges", "GET")
				assertParameter(t, listBridges, "scope", openapi3.ParameterInQuery, false)
				assertParameter(t, listBridges, "workspace_id", openapi3.ParameterInQuery, false)
				assertParameter(t, listBridges, "workspace", openapi3.ParameterInQuery, false)
				assertParameter(t, listBridges, "q", openapi3.ParameterInQuery, false)
				assertParameter(t, listBridges, "platform", openapi3.ParameterInQuery, false)
				assertParameter(t, listBridges, "status", openapi3.ParameterInQuery, false)
				assertParameter(t, listBridges, "sort", openapi3.ParameterInQuery, false)
				assertParameter(t, listBridges, "cursor", openapi3.ParameterInQuery, false)
				assertParameter(t, listBridges, "limit", openapi3.ParameterInQuery, false)
				listSchema := jsonResponseSchema(t, listBridges, 200)
				assertRequired(t, listSchema, "bridges", "bridge_health", "facets", "page")
				streamHealth := operationFor(t, doc, "/api/bridges/health/stream", "GET")
				assertParameter(t, streamHealth, "bridge_ids", openapi3.ParameterInQuery, true)

				providers := operationFor(t, doc, "/api/bridges/providers", "GET")
				providersSchema := jsonResponseSchema(t, providers, 200)
				assertRequired(t, providersSchema, "providers")

				providerItems := propertySchema(t, providersSchema, "providers")
				if providerItems.Items == nil || providerItems.Items.Value == nil {
					t.Fatal("expected providers to define an items schema")
				}
				providerSchema := providerItems.Items.Value
				assertRequired(
					t,
					providerSchema,
					"platform",
					"extension_name",
					"display_name",
					"enabled",
					"state",
					"health",
				)
				assertNotRequired(t, providerSchema, "description", "health_message", "secret_slots", "config_schema")

				getBridge := operationFor(t, doc, "/api/bridges/{id}", "GET")
				getBridgeSchema := jsonResponseSchema(t, getBridge, 200)
				bridgeSchema := propertySchema(t, getBridgeSchema, "bridge")
				assertEnumValues(t, propertySchema(t, bridgeSchema, "dm_policy"), "open", "allowlist", "pairing")
				assertEnumValues(t, propertySchema(t, bridgeSchema, "source"), "dynamic", "package")
				assertSchemaIncludesType(t, propertySchema(t, bridgeSchema, "provider_config"), openapi3.TypeObject)
				assertSchemaHasAdditionalProperties(t, propertySchema(t, bridgeSchema, "provider_config"), true)
				assertSchemaIncludesType(t, propertySchema(t, bridgeSchema, "delivery_defaults"), openapi3.TypeObject)
				bridgeDefaultsSchema := propertySchema(t, bridgeSchema, "delivery_defaults")
				assertSchemaHasAdditionalProperties(t, bridgeDefaultsSchema, true)
				assertAdditionalPropertiesSchemaIncludesType(t, bridgeDefaultsSchema, openapi3.TypeString)

				healthSchema := propertySchema(t, getBridgeSchema, "health")
				assertNotRequired(t, healthSchema, "last_success_at", "last_error", "last_error_at", "degradation")
				assertEnumValues(t, propertySchema(t, propertySchema(t, healthSchema, "degradation"), "reason"),
					"auth_failed",
					"rate_limited",
					"webhook_invalid",
					"provider_timeout",
					"tenant_config_invalid",
				)
				diagnosticsSchema := propertySchema(t, healthSchema, "diagnostics")
				if diagnosticsSchema.Items == nil || diagnosticsSchema.Items.Value == nil {
					t.Fatal("expected bridge diagnostics to define an items schema")
				}
				diagnosticSchema := diagnosticsSchema.Items.Value
				assertRequired(t, diagnosticSchema, "kind", "severity", "source", "message")
				assertEnumValues(t, propertySchema(t, diagnosticSchema, "kind"),
					"unknown_destination",
					"missing_token",
					"permission_denied",
					"unsupported_capability",
					"transient_delivery_failure",
				)
				assertEnumValues(t, propertySchema(t, diagnosticSchema, "severity"), "info", "warning", "error")
			},
		},
		{
			name: "Should describe the Slack manifest route and typed artifact",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				operation := operationFor(t, doc, "/api/bridges/providers/slack/manifest", "GET")
				assertParameter(t, operation, "instance", openapi3.ParameterInQuery, true)
				responseSchema := jsonResponseSchema(t, operation, 200)
				assertRequired(t, responseSchema, "manifest")
				manifestSchema := propertySchema(t, responseSchema, "manifest")
				assertRequired(
					t,
					manifestSchema,
					"_metadata",
					"display_information",
					"features",
					"oauth_config",
					"settings",
				)
				settingsSchema := propertySchema(t, manifestSchema, "settings")
				assertRequired(
					t,
					settingsSchema,
					"event_subscriptions",
					"interactivity",
					"org_deploy_enabled",
					"socket_mode_enabled",
					"token_rotation_enabled",
				)
				assertSchemaIncludesType(
					t,
					propertySchema(t, settingsSchema, "socket_mode_enabled"),
					openapi3.TypeBoolean,
				)
			},
		},
		{
			name: "ShouldDescribeTypedBridgeControlRoutesAndResponses",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				verify := operationFor(t, doc, "/api/bridges/{id}/verify", "POST")
				verifySchema := jsonResponseSchema(t, verify, http.StatusOK)
				assertRequired(t, verifySchema, "bridge_instance_id", "checks")
				checksSchema := propertySchema(t, verifySchema, "checks")
				if checksSchema.Items == nil || checksSchema.Items.Value == nil {
					t.Fatal("expected verify checks to define an items schema")
				}
				checkSchema := checksSchema.Items.Value
				assertRequired(t, checkSchema, "check", "status", "remediation")
				assertEnumValues(
					t,
					propertySchema(t, checkSchema, "status"),
					"pass",
					"warn",
					"fail",
					"skipped",
				)
				assertRequired(t, jsonResponseSchema(t, verify, http.StatusServiceUnavailable), "error")

				sendTest := operationFor(t, doc, "/api/bridges/{id}/send-test", "POST")
				sendTestRequest := jsonRequestSchema(t, sendTest)
				assertRequired(t, sendTestRequest, "message", "target")
				targetSchema := propertySchema(t, sendTestRequest, "target")
				assertNotRequired(t, targetSchema, "peer_id", "thread_id", "group_id", "mode")
				assertEnumValues(t, propertySchema(t, targetSchema, "mode"), "direct-send", "reply")
				sendTestResponse := jsonResponseSchema(t, sendTest, http.StatusOK)
				assertRequired(
					t,
					sendTestResponse,
					"status",
					"bridge_instance_id",
					"delivery_id",
					"delivery_target",
				)
				assertNotRequired(t, sendTestResponse, "remote_message_id", "error")
				assertEnumValues(
					t,
					propertySchema(t, sendTestResponse, "status"),
					"delivered",
					"committed_result_unavailable",
				)
				assertRequired(t, propertySchema(t, sendTestResponse, "error"), "message")
				assertRequired(t, jsonResponseSchema(t, sendTest, http.StatusBadRequest), "error")

				registerWebhook := operationFor(t, doc, "/api/bridges/{id}/webhook/register", "POST")
				registerSchema := jsonResponseSchema(t, registerWebhook, http.StatusOK)
				assertRequired(t, registerSchema, "bridge_instance_id", "status", "remediation")
				assertEnumValues(
					t,
					propertySchema(t, registerSchema, "status"),
					"pass",
					"warn",
					"fail",
					"skipped",
				)
				assertRequired(
					t,
					jsonResponseSchema(t, registerWebhook, http.StatusServiceUnavailable),
					"error",
				)
			},
		},
		{
			name: "ShouldDescribeToolRegistryContractsAndRoutes",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				listTools := operationFor(t, doc, "/api/tools", "GET")
				assertTagsContain(t, listTools, "tools")
				assertParameter(t, listTools, "workspace_id", openapi3.ParameterInQuery, false)
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
					"bridge",
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
					"model_not_found",
					"reasoning_effort_unsupported",
					"tool_approval_required",
					"tool_backend_failed",
					"tool_canceled",
					"tool_conflict",
					"tool_denied",
					"tool_invalid_input",
					"tool_not_found",
					"tool_result_persistence_failed",
					"tool_result_too_large",
					"tool_timed_out",
					"tool_unavailable",
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
			name: "ShouldDescribeBridgeSecretBindingContracts",
			check: func(t *testing.T, doc *openapi3.T) {
				t.Helper()

				listBindings := operationFor(t, doc, "/api/bridges/{id}/secret-bindings", "GET")
				assertParameter(t, listBindings, "id", openapi3.ParameterInPath, true)
				listBindingsSchema := jsonResponseSchema(t, listBindings, 200)
				assertRequired(t, listBindingsSchema, "bindings")

				bindingsSchema := propertySchema(t, listBindingsSchema, "bindings")
				if bindingsSchema.Items == nil || bindingsSchema.Items.Value == nil {
					t.Fatal("expected bindings to define an items schema")
				}
				bindingSchema := bindingsSchema.Items.Value
				assertRequired(
					t,
					bindingSchema,
					"bridge_instance_id",
					"binding_name",
					"secret_ref",
					"kind",
					"created_at",
					"updated_at",
				)

				putBinding := operationFor(t, doc, "/api/bridges/{id}/secret-bindings/{binding_name}", "PUT")
				assertParameter(t, putBinding, "id", openapi3.ParameterInPath, true)
				assertParameter(t, putBinding, "binding_name", openapi3.ParameterInPath, true)
				putBindingSchema := jsonRequestSchema(t, putBinding)
				assertRequired(t, putBindingSchema, "secret_ref", "kind")

				putBindingResponseSchema := jsonResponseSchema(t, putBinding, 200)
				assertRequired(t, putBindingResponseSchema, "binding")

				deleteBinding := operationFor(t, doc, "/api/bridges/{id}/secret-bindings/{binding_name}", "DELETE")
				assertParameter(t, deleteBinding, "id", openapi3.ParameterInPath, true)
				assertParameter(t, deleteBinding, "binding_name", openapi3.ParameterInPath, true)
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
					{path: "/api/tasks/{id}/execution-profile", method: "DELETE"},
					{path: "/api/tasks/{id}/notifications/bridges", method: "POST"},
					{path: "/api/tasks/{id}/notifications/bridges", method: "GET"},
					{path: "/api/tasks/{id}/notifications/bridges/{subscription_id}", method: "GET"},
					{path: "/api/tasks/{id}/notifications/bridges/{subscription_id}", method: "DELETE"},
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
					{path: "/api/agent/channels", method: "GET"},
					{path: "/api/agent/channels/{channel}/recv", method: "GET"},
					{path: "/api/agent/channels/{channel}/send", method: "POST"},
					{path: "/api/agent/channels/reply", method: "POST"},
					{path: "/api/agent/tasks/claim-next", method: "POST"},
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
					"coordination_channel",
					"inbox_summary",
					"peer_roster",
					"capabilities",
					"limits",
					"provenance",
				)

				channelContext := propertySchema(t, contextPayload, "coordination_channel")
				assertRequired(t, channelContext, "available")
				assertNotRequired(t, channelContext, "channel")
				channelSchema := propertySchema(t, channelContext, "channel")
				assertRequired(t, channelSchema, "id", "display_name", "allowed_message_kinds")
				kindsSchema := propertySchema(t, channelSchema, "allowed_message_kinds")
				if kindsSchema.Items == nil || kindsSchema.Items.Value == nil {
					t.Fatal("allowed_message_kinds should define item schema")
				}
				assertEnumValues(
					t,
					kindsSchema.Items.Value,
					"status",
					"request",
					"reply",
					"blocker",
					"handoff",
					"result",
					"review_request",
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
				assertNotRequired(t, leaseSchema, "claim_token_hash", "coordination_channel")

				heartbeatOperation := operationFor(t, doc, "/api/agent/tasks/{run_id}/heartbeat", "POST")
				assertParameter(t, heartbeatOperation, "run_id", openapi3.ParameterInPath, true)
				heartbeatSchema := jsonRequestSchema(t, heartbeatOperation)
				assertNotRequired(t, heartbeatSchema, "lease_seconds")
				if _, exists := heartbeatSchema.Properties["claim_token"]; exists {
					t.Fatalf("agent heartbeat schema exposes raw claim_token")
				}

				sendOperation := operationFor(t, doc, "/api/agent/channels/{channel}/send", "POST")
				assertParameter(t, sendOperation, "channel", openapi3.ParameterInPath, true)
				sendSchema := jsonRequestSchema(t, sendOperation)
				assertRequired(t, sendSchema, "body", "metadata")
				metadataSchema := propertySchema(t, sendSchema, "metadata")
				assertRequired(
					t,
					metadataSchema,
					"task_id",
					"run_id",
					"channel_id",
					"message_kind",
					"correlation_id",
				)
				assertEnumValues(
					t,
					propertySchema(t, metadataSchema, "message_kind"),
					"status",
					"request",
					"reply",
					"blocker",
					"handoff",
					"result",
					"review_request",
				)
				if _, exists := metadataSchema.Properties["claim_token"]; exists {
					t.Fatalf("coordination metadata schema exposes raw claim_token")
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
					"network_participation",
					"description",
					"priority",
					"max_attempts",
					"draft",
					"approval_policy",
					"owner",
					"wake_creator",
					"metadata",
				)
				assertEnumValues(t, propertySchema(t, createTaskSchema, "scope"), "global", "workspace")
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
					string(taskpkg.OwnerKindNetworkPeer),
					string(taskpkg.OwnerKindPool),
				)
				assertEnumValues(t, propertySchema(t, propertySchema(t, taskSchema, "created_by"), "kind"),
					string(taskpkg.ActorKindHuman),
					string(taskpkg.ActorKindAgentSession),
					string(taskpkg.ActorKindAutomation),
					string(taskpkg.ActorKindExtension),
					string(taskpkg.ActorKindNetworkPeer),
					string(taskpkg.ActorKindDaemon),
				)
				assertEnumValues(t, propertySchema(t, propertySchema(t, taskSchema, "origin"), "kind"),
					string(taskpkg.OriginKindCLI),
					string(taskpkg.OriginKindWeb),
					string(taskpkg.OriginKindUDS),
					string(taskpkg.OriginKindHTTP),
					string(taskpkg.OriginKindAutomation),
					string(taskpkg.OriginKindExtension),
					string(taskpkg.OriginKindNetwork),
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
				assertParameter(t, listTasks, "approval_state", openapi3.ParameterInQuery, false)
				assertParameter(t, listTasks, "query", openapi3.ParameterInQuery, false)
				assertParameter(t, listTasks, "sort", openapi3.ParameterInQuery, false)
				assertParameter(t, listTasks, "cursor", openapi3.ParameterInQuery, false)
				assertEnumValues(
					t,
					parameterSchema(t, listTasks, "scope", openapi3.ParameterInQuery),
					"all", "global", "workspace",
				)
				listTasksSchema := jsonResponseSchema(t, listTasks, 200)
				assertRequired(t, listTasksSchema, "tasks", "page", "facets")
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
					"coordination_channel",
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

				listTaskRuns := operationFor(t, doc, "/api/tasks/{id}/runs", "GET")
				assertParameter(t, listTaskRuns, "status", openapi3.ParameterInQuery, false)
				assertParameter(t, listTaskRuns, "session_id", openapi3.ParameterInQuery, false)
				assertParameter(t, listTaskRuns, "participation_channel", openapi3.ParameterInQuery, false)

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
				assertNotRequired(t, runDetailSchema, "task", "session", "network")
				runNetworkSchema := propertySchema(t, runDetailSchema, "network")
				assertRequired(t, runNetworkSchema, "conversation", "usage")
				assertRequired(
					t,
					propertySchema(t, runNetworkSchema, "conversation"),
					"workspace_id",
					"channel",
					"surface",
					"thread_id",
					"stream_url",
				)

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
					string(taskpkg.OriginKindNetwork),
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
				assertNotRequired(t, publishTaskSchema, "idempotency_key", "network_participation", "metadata")
				publishTaskResponse := jsonResponseSchema(t, publishTask, 200)
				assertRequired(t, publishTaskResponse, "task", "run")
				assertResponseStatus(t, publishTask, 409)
				assertResponseStatus(t, publishTask, 422)

				startTask := operationFor(t, doc, "/api/tasks/{id}/start", "POST")
				assertParameter(t, startTask, "id", openapi3.ParameterInPath, true)
				startTaskSchema := jsonRequestSchema(t, startTask)
				assertNotRequired(t, startTaskSchema, "idempotency_key", "network_participation", "metadata")
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
				assertNotRequired(t, approveSchema, "idempotency_key", "network_participation", "metadata")
				approveResponse := jsonResponseSchema(t, approve, 201)
				assertRequired(t, approveResponse, "task", "run")
				assertResponseStatus(t, approve, 409)
				assertResponseStatus(t, approve, 422)

				dashboard := operationFor(t, doc, "/api/observe/tasks/dashboard", "GET")
				assertTagsContain(t, dashboard, "observe", "tasks")
				assertParameter(t, dashboard, "scope", openapi3.ParameterInQuery, false)
				assertParameter(t, dashboard, "workspace", openapi3.ParameterInQuery, false)
				assertParameter(t, dashboard, "owner_kind", openapi3.ParameterInQuery, false)
				assertParameter(t, dashboard, "owner_ref", openapi3.ParameterInQuery, false)
				assertParameter(t, dashboard, "participation_channel", openapi3.ParameterInQuery, false)
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
					"coordination_channel",
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

		path := filepath.Join(t.TempDir(), "openapi", "agh.json")
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

	t.Run("Should write the document and keep enum helpers populated", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "openapi", "agh.json")
		if err := WriteFile(path); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("os.ReadFile() error = %v", err)
		}
		if !json.Valid(data) {
			t.Fatalf("WriteFile() output is not valid JSON: %s", string(data))
		}
		if !strings.HasSuffix(string(data), "\n") {
			t.Fatalf("WriteFile() output must end with newline: %q", string(data))
		}

		if got := hookSkillSourceValues(); len(got) == 0 {
			t.Fatal("hookSkillSourceValues() returned no values")
		}
		if got := hookExecutorKindValues(); len(got) == 0 {
			t.Fatal("hookExecutorKindValues() returned no values")
		}
		if got := toolSourceValues(); len(got) == 0 {
			t.Fatal("toolSourceValues() returned no values")
		}
		if got := hostAPIMethodValues(); len(got) == 0 || !slices.IsSorted(got) {
			t.Fatalf("hostAPIMethodValues() = %v, want non-empty sorted values", got)
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
		{name: "TaskRuntimeMode", typ: taskpkg.RuntimeModeDefault},
		{name: "AutomationSchedulerCatchUpPolicy", typ: automationpkg.SchedulerCatchUpPolicySkipMissed},
		{name: "TaskInboxLane", typ: contract.TaskInboxLaneApprovals},
		{name: "HookSkillSource", typ: hooks.HookSkillSourceBundled},
		{name: "HookExecutorKind", typ: hooks.HookExecutorNative},
		{name: "ToolSource", typ: tools.ToolSourceBuiltin},
		{name: "HostAPIMethod", typ: extensionprotocol.HostAPIMethod("memory.read")},
		{name: "ParticipationMode", typ: participation.ModeLive},
		{name: "ParticipationChannelStrategy", typ: participation.StrategyNamed},
		{name: "ParticipationSource", typ: participation.SourceExplicitRequest},
		{name: "ParticipationOwnerKind", typ: participation.OwnerKindTaskRun},
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

func TestParticipationSchemaCustomizerRepresentsRuntimeVariants(t *testing.T) {
	t.Parallel()

	requestSchema := openapi3.NewObjectSchema()
	customizeParticipationRequestSchema(requestSchema)
	if got, want := len(requestSchema.OneOf), 4; got != want {
		t.Fatalf("participation request oneOf branches = %d, want %d", got, want)
	}
	requestCases := []struct {
		name      string
		payload   string
		wantValid bool
	}{
		{name: "Should accept omitted Local intent", payload: `{}`, wantValid: true},
		{name: "Should accept explicit Local intent", payload: `{"mode":"local"}`, wantValid: true},
		{
			name: "Should accept bounded named Live intent",
			payload: `{
				"mode":"live",
				"channel_strategy":"named",
				"channel_id":"builders",
				"bounds":{"max_wakes":2,"max_input_tokens":1024}
			}`,
			wantValid: true,
		},
		{
			name:      "Should accept derived run intent without channel",
			payload:   `{"mode":"live","channel_strategy":"run"}`,
			wantValid: true,
		},
		{
			name:      "Should accept derived Loop run intent without channel",
			payload:   `{"mode":"live","channel_strategy":"loop_run"}`,
			wantValid: true,
		},
		{name: "Should reject Local channel data", payload: `{"mode":"local","channel_id":"builders"}`},
		{name: "Should reject Live without strategy", payload: `{"mode":"live"}`},
		{name: "Should reject named without channel", payload: `{"mode":"live","channel_strategy":"named"}`},
		{
			name:    "Should reject derived strategy with channel",
			payload: `{"mode":"live","channel_strategy":"run","channel_id":"builders"}`,
		},
		{
			name:    "Should reject invalid named channel",
			payload: `{"mode":"live","channel_strategy":"named","channel_id":"Invalid channel"}`,
		},
		{
			name:    "Should reject nonpositive bounds",
			payload: `{"mode":"live","channel_strategy":"run","bounds":{"max_wakes":0}}`,
		},
		{name: "Should reject unknown participation fields", payload: `{"mode":"local","legacy":true}`},
	}
	for _, testCase := range requestCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			assertOpenAPISchemaJSONValidity(t, requestSchema, testCase.payload, testCase.wantValid)
		})
	}

	specSchema := openapi3.NewObjectSchema()
	customizeParticipationSpecSchema(specSchema)
	if got, want := len(specSchema.OneOf), 2; got != want {
		t.Fatalf("participation spec oneOf branches = %d, want %d", got, want)
	}
	liveSpec := `{
		"version":"network-participation/v1",
		"mode":"live",
		"workspace_id":"ws-alpha",
		"channel_strategy":"named",
		"channel_id":"builders",
		"source":"explicit_request",
		"bounds":{
			"max_wakes":2,
			"max_wake_wall_time":"30s",
			"max_total_wall_time":"2m",
			"max_input_tokens":4096,
			"max_output_tokens":2048,
			"max_wake_depth":2,
			"coalesce_window":"250ms"
		}
	}`
	assertOpenAPISchemaJSONValidity(
		t,
		specSchema,
		`{"version":"network-participation/v1","mode":"local","source":"built_in_local"}`,
		true,
	)
	assertOpenAPISchemaJSONValidity(t, specSchema, liveSpec, true)
	assertOpenAPISchemaJSONValidity(
		t,
		specSchema,
		`{"version":"network-participation/v1","mode":"local","source":"built_in_local","bounds":{}}`,
		false,
	)
	assertOpenAPISchemaJSONValidity(
		t,
		specSchema,
		`{"version":"network-participation/v1","mode":"live","workspace_id":"ws-alpha","channel_strategy":"run","channel_id":"run-1","source":"explicit_request"}`,
		false,
	)
}

func TestNetworkCoordinationMutationSchemaMatchesRuntimeValidation(t *testing.T) {
	t.Parallel()

	doc, err := Document()
	if err != nil {
		t.Fatalf("Document() error = %v", err)
	}

	tests := []struct {
		name             string
		operationID      string
		mutationProperty string
	}{
		{
			name:             "Should describe coordination setting variants",
			operationID:      "putNetworkCoordination",
			mutationProperty: "enabled",
		},
		{
			name:             "Should describe invitation variants",
			operationID:      "putNetworkCoordinationInvitation",
			mutationProperty: "dismissed",
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var operation *openapi3.Operation
			for _, candidate := range doc.Paths.Map() {
				for _, method := range []string{http.MethodPut} {
					if current := candidate.GetOperation(method); current != nil &&
						current.OperationID == testCase.operationID {
						operation = current
					}
				}
			}
			if operation == nil {
				t.Fatalf("missing operation %q", testCase.operationID)
			}

			schema := jsonRequestSchema(t, operation)
			if got, want := len(schema.OneOf), 2; got != want {
				t.Fatalf("coordination request oneOf branches = %d, want %d", got, want)
			}
			assertOpenAPISchemaJSONValidity(
				t,
				schema,
				fmt.Sprintf(`{"scope":"workspace",%q:true,"expected_revision":0}`, testCase.mutationProperty),
				true,
			)
			assertOpenAPISchemaJSONValidity(
				t,
				schema,
				fmt.Sprintf(
					`{"scope":"task","task_id":"task-1",%q:false,"expected_revision":2}`,
					testCase.mutationProperty,
				),
				true,
			)
			assertOpenAPISchemaJSONValidity(
				t,
				schema,
				fmt.Sprintf(`{"scope":"workspace",%q:null,"expected_revision":0}`, testCase.mutationProperty),
				false,
			)
			assertOpenAPISchemaJSONValidity(
				t,
				schema,
				fmt.Sprintf(
					`{"scope":"workspace","task_id":"task-1",%q:true,"expected_revision":0}`,
					testCase.mutationProperty,
				),
				false,
			)
			assertOpenAPISchemaJSONValidity(
				t,
				schema,
				fmt.Sprintf(`{"scope":"task",%q:true,"expected_revision":0}`, testCase.mutationProperty),
				false,
			)
			assertResponseStatus(t, operation, http.StatusServiceUnavailable)
		})
	}

	getCoordination := operationFor(
		t,
		doc,
		"/api/workspaces/{workspace_id}/network-coordination",
		http.MethodGet,
	)
	usage := operationFor(t, doc, "/api/workspaces/{workspace_id}/network/usage", http.MethodGet)
	assertResponseStatus(t, getCoordination, http.StatusServiceUnavailable)
	assertResponseStatus(t, usage, http.StatusServiceUnavailable)
}

func assertOpenAPISchemaJSONValidity(
	t *testing.T,
	schema *openapi3.Schema,
	payload string,
	wantValid bool,
) {
	t.Helper()
	var value any
	if err := json.Unmarshal([]byte(payload), &value); err != nil {
		t.Fatalf("json.Unmarshal(schema payload) error = %v", err)
	}
	err := schema.VisitJSON(value)
	if wantValid && err != nil {
		t.Fatalf("schema rejected valid payload %s: %v", payload, err)
	}
	if !wantValid && err == nil {
		t.Fatalf("schema accepted invalid payload %s", payload)
	}
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
			want: []string{"human", "agent_session", "automation", "extension", "network_peer", "daemon"},
		},
		{
			name: "task owner kind values",
			got:  taskOwnerKindValues(),
			want: []string{"human", "agent_session", "automation", "extension", "network_peer", "pool"},
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
				"network",
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

	t.Run("Should host api method values", func(t *testing.T) {
		t.Parallel()

		got := hostAPIMethodValues()
		if !slices.IsSorted(got) {
			t.Fatalf("values are not sorted: %v", got)
		}
		for _, want := range []string{"bridges/messages/ingest", "bridges/instances/get", "bridges/instances/report_state"} {
			if !contains(got, want) {
				t.Fatalf("expected %q in host api method values %v", want, got)
			}
		}
	})
}

func TestOperationsRemainUniqueWithExpandedTaskSurface(t *testing.T) {
	t.Parallel()

	t.Run("Should keep expanded task operations unique by route and operation id", func(t *testing.T) {
		t.Parallel()

		seenRouteMethods := make(map[string]struct{}, len(Operations()))
		seenOperationIDs := make(map[string]struct{}, len(Operations()))

		for _, operation := range Operations() {
			routeMethodKey := operation.Method + " " + operation.Path
			if _, ok := seenRouteMethods[routeMethodKey]; ok {
				t.Fatalf("duplicate route+method %q", routeMethodKey)
			}
			seenRouteMethods[routeMethodKey] = struct{}{}

			if _, ok := seenOperationIDs[operation.OperationID]; ok {
				t.Fatalf("duplicate operation id %q", operation.OperationID)
			}
			seenOperationIDs[operation.OperationID] = struct{}{}
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

func assertOperationAbsent(t *testing.T, doc *openapi3.T, path string, method string) {
	t.Helper()

	pathItem := doc.Paths.Value(path)
	if pathItem == nil {
		return
	}
	if operation := pathItem.GetOperation(method); operation != nil {
		t.Fatalf("unexpected operation %s %s", method, path)
	}
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

	propertyRef := schema.Properties[name]
	if propertyRef == nil || propertyRef.Value == nil {
		t.Fatalf("missing property %q", name)
	}
	return propertyRef.Value
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

	if schema.Type == nil || !schema.Type.Includes(want) {
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
