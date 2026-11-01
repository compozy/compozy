import type {
  AgentActivityPayload,
  AgentGroupedCatalogItem,
  AgentInstancePayload,
  AgentPayload,
} from "../types";
import { agentInstanceFixture } from "./instance-fixtures";

export const TEAM_WORKSPACES = [
  { id: "ws-compozy", name: "compozy" },
  { id: "ws-advos", name: "advos" },
  { id: "ws-reloaddev", name: "reloaddev" },
];

export function agentPayloadFixture(overrides: Partial<AgentPayload> = {}): AgentPayload {
  return {
    name: "triage",
    label: "Triage Lead",
    display_name: "Triage Lead",
    description: "Watches issues and proposes the next step",
    kind: "stays_on",
    provider: "claude",
    model: "claude-opus-5-5",
    origin: "global",
    layer: "user",
    prompt: "Keep this project's issues moving.",
    definition_digest: "sha256:triage",
    resolved_appearance: {
      version: 1,
      avatar: "generated",
      seed: "triage",
      generation: 1,
      color: "#2aa8a8",
    },
    ...overrides,
  } as AgentPayload;
}

export function teamItemFixture(
  overrides: Partial<AgentPayload> = {},
  item: Partial<AgentGroupedCatalogItem> = {}
): AgentGroupedCatalogItem {
  const agent = agentPayloadFixture(overrides);
  return {
    name: agent.name,
    label: agent.label,
    kind: agent.kind ?? "on_demand",
    definition_digest: agent.definition_digest,
    scopes: ["ws-compozy"],
    active: 0,
    agent,
    ...item,
  };
}

export function teamItems(): AgentGroupedCatalogItem[] {
  return [
    teamItemFixture(),
    teamItemFixture({
      name: "orchestrator",
      label: "Orchestrator",
      description: "Delegates each spec task to a dedicated worker",
      package_owned: true,
      layer: "additional",
      definition_digest: "sha256:orchestrator",
    }),
    teamItemFixture(
      {
        name: "general",
        label: "General",
        kind: "on_demand",
        description: "Takes on anything you ask",
        definition_digest: "sha256:general",
      },
      { active: 2, scopes: ["@user", "ws-compozy", "ws-advos"] }
    ),
    teamItemFixture({
      name: "reviewer",
      label: "Reviewer",
      kind: "on_demand",
      description: "Reads changes and says what could break",
      definition_digest: "sha256:reviewer",
    }),
  ];
}

export function teamInstances(): AgentInstancePayload[] {
  return [
    agentInstanceFixture({ activity: "working" }),
    agentInstanceFixture({
      id: "ainst-advos",
      workspace_id: "ws-advos",
      scope: "advos",
      now: "",
      state: "paused",
      pause_reason: "manual",
    }),
    agentInstanceFixture({
      id: "ainst-orch",
      agent_name: "orchestrator",
      label: "Orchestrator",
      needs_attention: true,
      now: "Waiting for your answer",
    }),
  ];
}

export function activityFixture(): AgentActivityPayload {
  return {
    reports_by_day: Array.from({ length: 30 }, (_, index) => ({
      day: `2026-10-${String(index + 1).padStart(2, "0")}`,
      quiet: 8 + (index % 5),
      told: 1 + (index % 4),
      needed_you: index % 6 === 1 ? 1 : 0,
    })),
    cost_by_project: [
      { workspace_id: "ws-compozy", scope: "compozy", usd_estimate: 8.4, tokens: 2_100_000 },
      { workspace_id: "ws-advos", scope: "advos", usd_estimate: 4, tokens: 1_000_000 },
    ],
    busy_by_hour: [1, 2, 3, 4, 5].flatMap(weekday =>
      Array.from({ length: 10 }, (_, index) => ({
        weekday,
        hour: 9 + index,
        events: ((index * 3 + weekday * 5) % 7) + 2,
      }))
    ),
  };
}
