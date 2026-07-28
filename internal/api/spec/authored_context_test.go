package spec

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestAuthoredContextOpenAPIContracts(t *testing.T) {
	t.Run("Should register shared Soul Heartbeat health and wake operations", func(t *testing.T) {
		t.Parallel()

		doc := authoredContextDocument(t)
		for _, target := range []struct {
			path   string
			method string
			status int
		}{
			{path: "/api/agent/soul", method: "GET", status: 200},
			{path: "/api/agents/{name}/soul", method: "PUT", status: 200},
			{path: "/api/agents/{name}/heartbeat", method: "GET", status: 200},
			{path: "/api/agents/{name}/heartbeat/status", method: "GET", status: 200},
			{path: "/api/agents/{name}/heartbeat/wake", method: "POST", status: 200},
			{path: "/api/workspaces/{workspace_id}/sessions/{session_id}/health", method: "GET", status: 200},
			{path: "/api/workspaces/{workspace_id}/sessions/{session_id}/status", method: "GET", status: 200},
			{path: "/api/workspaces/{workspace_id}/sessions/{session_id}/inspect", method: "GET", status: 200},
		} {
			operation := operationFor(t, doc, target.path, target.method)
			assertResponseStatus(t, operation, target.status)
		}
	})

	t.Run("Should keep compact agent context Soul projection body-free", func(t *testing.T) {
		t.Parallel()

		doc := authoredContextDocument(t)
		contextOperation := operationFor(t, doc, "/api/agent/context", "GET")
		contextSchema := jsonResponseSchema(t, contextOperation, 200)
		contextPayloadSchema := propertySchema(t, contextSchema, "context")
		soulSchema := propertySchema(t, contextPayloadSchema, "soul")
		assertRequired(t, soulSchema, "enabled", "present", "active", "valid", "tone", "principles")
		assertEnumValues(t, propertySchema(t, soulSchema, "validation_status"),
			"missing",
			"inactive",
			"valid",
			"invalid",
		)
		if property := soulSchema.Properties["body"]; property != nil {
			t.Fatalf("compact Soul schema exposed full body property: %#v", property)
		}
	})

	t.Run("Should keep path-bound Soul request identity path-only", func(t *testing.T) {
		t.Parallel()

		doc := authoredContextDocument(t)
		validateSoul := operationFor(t, doc, "/api/agents/{name}/soul/validate", "POST")
		validateSoulSchema := jsonRequestSchema(t, validateSoul)
		assertNotRequired(t, validateSoulSchema, "workspace_id", "body")
		assertPropertyAbsent(t, validateSoulSchema, "agent_name")

		putSoul := operationFor(t, doc, "/api/agents/{name}/soul", "PUT")
		putSoulSchema := jsonRequestSchema(t, putSoul)
		assertRequired(t, putSoulSchema, "body", "expected_digest")
		assertNotRequired(t, putSoulSchema, "workspace_id", "idempotency_key")
		assertPropertyAbsent(t, putSoulSchema, "agent_name")

		deleteSoul := operationFor(t, doc, "/api/agents/{name}/soul", "DELETE")
		deleteSoulSchema := jsonRequestSchema(t, deleteSoul)
		assertRequired(t, deleteSoulSchema, "expected_digest")
		assertPropertyAbsent(t, deleteSoulSchema, "agent_name")
		if property := deleteSoulSchema.Properties["if_match"]; property != nil {
			t.Fatalf("Soul delete request schema exposed transport-specific if_match: %#v", property)
		}

		rollbackSoul := operationFor(t, doc, "/api/agents/{name}/soul/rollback", "POST")
		rollbackSoulSchema := jsonRequestSchema(t, rollbackSoul)
		assertRequired(t, rollbackSoulSchema, "revision_id", "expected_digest")
		assertNotRequired(t, rollbackSoulSchema, "idempotency_key")
		assertPropertyAbsent(t, rollbackSoulSchema, "agent_name")
	})

	t.Run("Should keep path-bound Heartbeat request identity path-only", func(t *testing.T) {
		t.Parallel()

		doc := authoredContextDocument(t)
		validateHeartbeat := operationFor(t, doc, "/api/agents/{name}/heartbeat/validate", "POST")
		validateHeartbeatSchema := jsonRequestSchema(t, validateHeartbeat)
		assertRequired(t, validateHeartbeatSchema, "body")
		assertNotRequired(t, validateHeartbeatSchema, "workspace_id")
		assertPropertyAbsent(t, validateHeartbeatSchema, "agent_name")

		putHeartbeat := operationFor(t, doc, "/api/agents/{name}/heartbeat", "PUT")
		putHeartbeatSchema := jsonRequestSchema(t, putHeartbeat)
		assertRequired(t, putHeartbeatSchema, "body", "expected_digest")
		assertNotRequired(t, putHeartbeatSchema, "workspace_id", "idempotency_key")
		assertPropertyAbsent(t, putHeartbeatSchema, "agent_name")

		deleteHeartbeat := operationFor(t, doc, "/api/agents/{name}/heartbeat", "DELETE")
		deleteHeartbeatSchema := jsonRequestSchema(t, deleteHeartbeat)
		assertRequired(t, deleteHeartbeatSchema, "expected_digest")
		assertPropertyAbsent(t, deleteHeartbeatSchema, "agent_name")

		rollbackHeartbeat := operationFor(t, doc, "/api/agents/{name}/heartbeat/rollback", "POST")
		rollbackHeartbeatSchema := jsonRequestSchema(t, rollbackHeartbeat)
		assertRequired(t, rollbackHeartbeatSchema, "expected_digest")
		assertNotRequired(t, rollbackHeartbeatSchema, "revision_id", "target_digest")
		assertPropertyAbsent(t, rollbackHeartbeatSchema, "agent_name")
		if property := rollbackHeartbeatSchema.Properties["if_match"]; property != nil {
			t.Fatalf("Heartbeat rollback request schema exposed transport-specific if_match: %#v", property)
		}

		wakeHeartbeat := operationFor(t, doc, "/api/agents/{name}/heartbeat/wake", "POST")
		wakeHeartbeatSchema := jsonRequestSchema(t, wakeHeartbeat)
		assertRequired(t, wakeHeartbeatSchema, "session_id", "source")
		assertNotRequired(t, wakeHeartbeatSchema, "workspace_id", "dry_run", "idempotency_key")
		assertPropertyAbsent(t, wakeHeartbeatSchema, "agent_name")
	})

	t.Run("Should describe closed diagnostics health and wake enums", func(t *testing.T) {
		t.Parallel()

		doc := authoredContextDocument(t)
		getSoul := operationFor(t, doc, "/api/agent/soul", "GET")
		soulSchema := jsonResponseSchema(t, getSoul, 200)
		assertEnumValues(t, propertySchema(t, soulSchema, "validation_status"),
			"missing",
			"inactive",
			"valid",
			"invalid",
		)
		diagnosticsSchema := propertySchema(t, soulSchema, "diagnostics")
		if diagnosticsSchema.Items == nil || diagnosticsSchema.Items.Value == nil {
			t.Fatal("Soul diagnostics schema missing items")
		}
		assertEnumValues(t, propertySchema(t, diagnosticsSchema.Items.Value, "severity"),
			"info",
			"warning",
			"error",
		)

		healthOperation := operationFor(t, doc, "/api/workspaces/{workspace_id}/sessions/{session_id}/health", "GET")
		healthResponseSchema := jsonResponseSchema(t, healthOperation, 200)
		healthSchema := propertySchema(t, healthResponseSchema, "health")
		assertEnumValues(t, propertySchema(t, healthSchema, "state"),
			"idle",
			"prompting",
			"stopped",
			"detached",
		)
		assertEnumValues(t, propertySchema(t, healthSchema, "health"),
			"healthy",
			"degraded",
			"stale",
			"dead",
			"unknown",
		)
		assertEnumValues(t, propertySchema(t, healthSchema, "ineligibility_reason"),
			"session_prompt_active",
			"session_not_attachable",
			"session_unhealthy",
			"session_health_stale",
			"session_health_hung",
			"session_health_dead",
			"session_health_unknown",
		)
		sessionStatusOperation := operationFor(
			t,
			doc,
			"/api/workspaces/{workspace_id}/sessions/{session_id}/status",
			"GET",
		)
		sessionStatusSchema := jsonResponseSchema(t, sessionStatusOperation, 200)
		assertRequired(t,
			sessionStatusSchema,
			"session_id",
			"workspace_id",
			"agent_name",
			"state",
			"health",
			"active_prompt",
			"attachable",
			"eligible_for_wake",
			"updated_at",
		)
		assertNotRequired(t, sessionStatusSchema, "ineligibility_reason", "wake_state")
		assertEnumValues(t, propertySchema(t, sessionStatusSchema, "state"),
			"idle",
			"prompting",
			"stopped",
			"detached",
		)
		assertEnumValues(t, propertySchema(t, sessionStatusSchema, "health"),
			"healthy",
			"degraded",
			"stale",
			"dead",
			"unknown",
		)
		assertEnumValues(t, propertySchema(t, sessionStatusSchema, "ineligibility_reason"),
			"session_prompt_active",
			"session_not_attachable",
			"session_unhealthy",
			"session_health_stale",
			"session_health_hung",
			"session_health_dead",
			"session_health_unknown",
		)

		wakeOperation := operationFor(t, doc, "/api/agents/{name}/heartbeat/wake", "POST")
		wakeResponseSchema := jsonResponseSchema(t, wakeOperation, 200)
		decisionSchema := propertySchema(t, wakeResponseSchema, "decision")
		assertEnumValues(t, propertySchema(t, decisionSchema, "result"),
			"sent",
			"skipped",
			"coalesced",
			"rate_limited",
			"failed",
		)
		assertEnumValues(t, propertySchema(t, decisionSchema, "reason"),
			"wake_sent",
			"heartbeat_disabled",
			"heartbeat_invalid",
			"heartbeat_no_policy",
			"heartbeat_rate_limited",
			"heartbeat_no_eligible_session",
			"cooldown_active",
			"quiet_window",
			"session_not_found",
			"session_unhealthy",
			"session_not_attachable",
			"session_prompt_active",
			"session_prompt_active_race",
			"synthetic_prompt_failed",
			"wake_coalesced",
		)
	})

	t.Run("Should expose HTTP and UDS transport parity on new operations", func(t *testing.T) {
		t.Parallel()

		doc := authoredContextDocument(t)
		for _, target := range []struct {
			path   string
			method string
		}{
			{path: "/api/agent/soul", method: "GET"},
			{path: "/api/agents/{name}/heartbeat/status", method: "GET"},
			{path: "/api/workspaces/{workspace_id}/sessions/{session_id}/inspect", method: "GET"},
		} {
			operation := operationFor(t, doc, target.path, target.method)
			assertOperationTransports(t, operation, TransportHTTP, TransportUDS)
		}
	})
}

func authoredContextDocument(t *testing.T) *openapi3.T {
	t.Helper()

	doc, err := Document()
	if err != nil {
		t.Fatalf("Document() error = %v", err)
	}
	return doc
}
