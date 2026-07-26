import { describe, expect, it } from "vitest";

import type { SessionPayload } from "@/systems/session";

import { deriveAgentFleetSignals } from "../fleet-signals";

function session(overrides: Partial<SessionPayload> = {}): SessionPayload {
  return {
    id: "sess-1",
    agent_name: "coder",
    provider: "claude",
    workspace_id: "ws_alpha",
    workspace_path: "/ws",
    state: "stopped",
    badge: "idle",
    attachable: true,
    available_commands: [],
    created_at: "2026-04-01T00:00:00Z",
    updated_at: "2026-04-01T01:00:00Z",
    ...overrides,
  };
}

describe("deriveAgentFleetSignals", () => {
  it("Should mark status active when any session is active", () => {
    const signals = deriveAgentFleetSignals([
      session({ id: "a", state: "stopped" }),
      session({ id: "b", state: "active" }),
    ]);
    expect(signals.status).toBe("active");
    expect(signals.active).toBe(1);
    expect(signals.total).toBe(2);
  });

  it("Should count stopped-with-failure as failed", () => {
    const signals = deriveAgentFleetSignals([
      session({
        id: "fail",
        state: "stopped",
        stop_reason: "error",
      }),
      session({
        id: "ok",
        state: "stopped",
      }),
    ]);
    expect(signals.failed).toBe(1);
    expect(signals.status).toBe("idle");
  });

  it("Should prefer last_activity_at and fall back to updated_at", () => {
    const signals = deriveAgentFleetSignals([
      session({
        id: "old",
        updated_at: "2026-04-02T00:00:00Z",
        activity: {
          elapsed_ms: 10_000,
          last_activity_at: "2026-04-01T12:00:00Z",
          elapsed_seconds: 10,
          idle_seconds: 0,
          iteration_current: 0,
          iteration_max: 0,
        },
      }),
      session({
        id: "newer-updated",
        updated_at: "2026-04-03T00:00:00Z",
      }),
    ]);
    expect(signals.lastActivityMs).toBe(new Date("2026-04-03T00:00:00Z").getTime());
    expect(signals.runtimeSeconds).toBe(10);
  });
});
