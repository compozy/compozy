import { describe, expect, it } from "vitest";
import type { AgentEventPayload, RuntimeActivityPayload } from "../../types";
import {
  isOperationalStatusEvent,
  isSessionErrorEvent,
  isRuntimeActivityEvent,
  isTranscriptMarkerEvent,
} from "../runtime-activity-notice";

const runtime: RuntimeActivityPayload = {
  turn_id: "turn_001",
  turn_source: "user",
  current_tool: "Bash",
  idle_seconds: 42,
  elapsed_ms: 660_000,
  elapsed_seconds: 660,
  last_activity_kind: "tool_call",
  last_activity_detail: "running command",
};

function providerErrorEvent(
  overrides: Partial<NonNullable<AgentEventPayload["provider_error"]>> = {}
): AgentEventPayload {
  return {
    type: "error",
    error: "provider authentication required",
    failure: { kind: "prompt_failure", summary: "provider authentication required" },
    provider_error: {
      code: "provider_auth_required",
      provider: "claude-code",
      next_action: "login",
      guidance: "run provider auth login for this provider",
      occurrence_count: 1,
      first_seen_at: "2026-09-05T14:02:00Z",
      last_seen_at: "2026-09-05T14:02:00Z",
      ...overrides,
    },
  };
}

describe("runtime activity predicates", () => {
  it("recognizes runtime progress and warning events only when activity exists", () => {
    expect(isRuntimeActivityEvent({ type: "runtime_progress", runtime })).toBe(true);
    expect(isRuntimeActivityEvent({ type: "runtime_warning", runtime })).toBe(true);
    expect(isRuntimeActivityEvent({ type: "runtime_progress" })).toBe(false);
    expect(isRuntimeActivityEvent({ type: "agent_message", runtime })).toBe(false);
  });

  it("recognizes fatal session errors when error or failure details exist", () => {
    expect(
      isSessionErrorEvent({
        type: "error",
        error: '{"code":-32603,"message":"Internal error"}',
      })
    ).toBe(true);
    expect(isSessionErrorEvent({ type: "error", failure: { kind: "process_exit" } })).toBe(false);
    expect(
      isSessionErrorEvent({
        type: "error",
        failure: { kind: "process_exit", summary: "peer disconnected before response" },
      })
    ).toBe(true);
    expect(isSessionErrorEvent({ type: "runtime_warning", error: "failed" })).toBe(false);
  });

  it("does not read a stop the daemon attributed as a session failure", () => {
    // The live tail carries a supervisor stop as an `error` event whose text is the stop detail.
    const inactivity: AgentEventPayload = {
      type: "error",
      stop_reason: "timeout",
      error: "inactivity",
      timestamp: "2026-09-06T13:55:28.539506Z",
    };
    expect(isSessionErrorEvent(inactivity)).toBe(false);
    expect(
      isSessionErrorEvent({ ...inactivity, stop_reason: "user_canceled", error: "stopped" })
    ).toBe(false);
    // A failure stop reason, a failure record, or a provider diagnostic still is one.
    expect(isSessionErrorEvent({ ...inactivity, stop_reason: "agent_crashed" })).toBe(true);
    expect(
      isSessionErrorEvent({
        ...inactivity,
        failure: { kind: "provider_exit", summary: "exit 137" },
      })
    ).toBe(true);
  });

  it("recognizes an actionable provider error even without free-text detail", () => {
    const { error: _error, failure: _failure, ...bare } = providerErrorEvent();
    expect(isSessionErrorEvent(bare)).toBe(true);
    expect(
      isSessionErrorEvent({ ...bare, provider_error: { ...bare.provider_error!, code: "unknown" } })
    ).toBe(false);
  });

  it("recognizes transcript marker events", () => {
    expect(isTranscriptMarkerEvent({ type: "transcript_marker.created" })).toBe(true);
    expect(isTranscriptMarkerEvent({ type: "transcript_marker.redacted" })).toBe(true);
    expect(isTranscriptMarkerEvent({ type: "runtime_warning" })).toBe(false);
  });

  it("recognizes prompt lifecycle events as operational status", () => {
    for (const type of [
      "prompt_queued",
      "prompt_steered",
      "prompt_accepted",
      "prompt_dropped",
      "canceled",
    ]) {
      expect(isOperationalStatusEvent({ type })).toBe(true);
    }
    expect(isOperationalStatusEvent({ type: "runtime_warning" })).toBe(false);
  });

  it("recognizes text-only errors while preserving empty and attributed-stop filtering", () => {
    expect(isSessionErrorEvent({ type: "error", text: "peer disconnected" })).toBe(true);
    expect(isSessionErrorEvent({ type: "error", text: "  " })).toBe(false);
    expect(isSessionErrorEvent({ type: "error" })).toBe(false);
    expect(isSessionErrorEvent({ type: "agent_message", text: "peer disconnected" })).toBe(false);
    expect(isSessionErrorEvent({ type: "error", text: "stopped", stop_reason: "timeout" })).toBe(
      false
    );
  });
});
