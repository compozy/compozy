import type { AgentAppearancePayload, AgentInstancePayload } from "../types";

function generated(seed: string, color = "#2aa8a8"): AgentAppearancePayload {
  return { version: 1, avatar: "generated", seed, generation: 1, color };
}

/** `_dx.md`'s instance shape, as the sidebar and Kanban read it. */
export function agentInstanceFixture(
  overrides: Partial<AgentInstancePayload> = {}
): AgentInstancePayload {
  return {
    id: "ainst-41c7e0fa",
    agent_name: "triage",
    label: "Triage Lead",
    provider: "claude",
    resolved_appearance: generated("triage"),
    workspace_id: "ws-compozy",
    scope: "compozy",
    state: "active",
    activity: "idle",
    now: "Sorting 14 new issues",
    needs_attention: false,
    pause_reason: "",
    main_session_id: "sess-main-triage",
    checks_enabled: true,
    last_wake_at: "2026-10-31T23:00:00Z",
    next_wake_at: "2026-10-31T23:30:00Z",
    ...overrides,
  };
}

export const agentInstanceFixtures: AgentInstancePayload[] = [
  agentInstanceFixture({ needs_attention: true, activity: "waiting" }),
  agentInstanceFixture({
    id: "ainst-23a26081",
    agent_name: "orchestrator",
    label: "Orchestrator",
    resolved_appearance: generated("orchestrator", "#e8b04a"),
    activity: "working",
    now: "Waiting on two workers",
    main_session_id: "sess-main-orchestrator",
  }),
  agentInstanceFixture({
    id: "ainst-5b1c92de",
    agent_name: "release-captain",
    label: "Release Captain",
    provider: "codex",
    resolved_appearance: { version: 1, avatar: "emoji", emoji: "🧭", color: "#2aa8a8" },
    workspace_id: "",
    scope: "@user",
    now: "Nothing to do right now",
    main_session_id: "sess-main-release",
  }),
];
