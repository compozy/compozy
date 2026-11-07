import { describe, expect, expectTypeOf, it } from "vitest";

import * as sdk from "../index.js";
import type {
  AgentSoulPayload,
  AgentSoulPutRequest,
  HeartbeatStatusResponse,
  HeartbeatWakeResponse,
  HostAPIMethodMap,
  SessionHealthPayload,
} from "../generated/contracts.js";

describe("generated authored context contracts", () => {
  it("exports Soul authoring and read model contract types", () => {
    expectTypeOf<AgentSoulPutRequest>().toMatchTypeOf<{
      agent_name: string;
      body: string;
      expected_digest: string;
      workspace_id?: string;
      idempotency_key?: string;
    }>();
    expectTypeOf<AgentSoulPayload["validation_status"]>().toEqualTypeOf<
      "missing" | "inactive" | "valid" | "invalid"
    >();
    expectTypeOf<AgentSoulPayload>().toMatchTypeOf<{
      body?: string;
      diagnostics?: {
        severity: "info" | "warning" | "error";
      }[];
    }>();
  });

  it("exports Heartbeat status health and wake contract types", () => {
    type WakeDecision = HeartbeatWakeResponse["decision"];
    type WakeEvent = NonNullable<HeartbeatStatusResponse["wake_events"]>[number];

    expectTypeOf<WakeEvent["source"]>().toEqualTypeOf<
      "scheduler" | "manual" | "harness_reentry" | "timer" | "instance_check"
    >();

    expectTypeOf<SessionHealthPayload["state"]>().toEqualTypeOf<
      "idle" | "prompting" | "stopped" | "detached"
    >();
    expectTypeOf<SessionHealthPayload["health"]>().toEqualTypeOf<
      "healthy" | "degraded" | "stale" | "dead" | "unknown"
    >();
    expectTypeOf<WakeDecision["result"]>().toEqualTypeOf<
      | "sent"
      | "skipped"
      | "coalesced"
      | "rate_limited"
      | "failed"
      | "quiet"
      | "cancelled"
      | "reported"
    >();
    expectTypeOf<WakeDecision["reason"]>().toEqualTypeOf<
      | "wake_sent"
      | "heartbeat_disabled"
      | "heartbeat_invalid"
      | "heartbeat_no_policy"
      | "heartbeat_rate_limited"
      | "heartbeat_no_eligible_session"
      | "cooldown_active"
      | "quiet_window"
      | "session_not_found"
      | "session_unhealthy"
      | "session_not_attachable"
      | "session_prompt_active"
      | "session_prompt_active_race"
      | "synthetic_prompt_failed"
      | "wake_coalesced"
      | "source_poll_in_flight"
      | "quiet"
      | "reported"
      | "cancelled"
      | "digest_empty"
      | "digest_unchanged"
      | "policy_disabled"
      | "definition_not_always_on"
      | "definition_missing"
      | "no_slot"
      | "hook_cancelled"
      | "duplicate_suppressed"
      | "triage_unsupported"
      | "triage_output_invalid"
      | "triage_timeout"
      | "triage_interrupted"
      | "triage_config_unavailable"
      | "triage_failed"
    >();
    expectTypeOf<HeartbeatStatusResponse["session_health"]>().toMatchTypeOf<
      SessionHealthPayload | null | undefined
    >();
  });

  it("exports authored context Host API method typings", () => {
    expectTypeOf<
      HostAPIMethodMap["agents/soul/put"]["params"]
    >().toMatchTypeOf<AgentSoulPutRequest>();
    expectTypeOf<HostAPIMethodMap["agents/soul/get"]["result"]>().toMatchTypeOf<AgentSoulPayload>();
    expectTypeOf<
      HostAPIMethodMap["agents/heartbeat/status"]["result"]
    >().toMatchTypeOf<HeartbeatStatusResponse>();
    expectTypeOf<
      HostAPIMethodMap["agents/heartbeat/wake"]["result"]
    >().toMatchTypeOf<HeartbeatWakeResponse>();
  });

  it("does not export Soul or Heartbeat editor helpers from the SDK package barrel", () => {
    const exportedNames = Object.keys(sdk as Record<string, unknown>);
    const forbiddenName =
      /(Soul|Heartbeat|SessionHealth)(?:Editor|Form|Composer|Settings|Panel|Inspector|Workbench|Builder)/;
    const editorExports = exportedNames.filter(name => forbiddenName.test(name));
    expect(editorExports).toEqual([]);
  });
});
