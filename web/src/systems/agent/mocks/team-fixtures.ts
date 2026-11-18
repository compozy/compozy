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
    kind: "always_on",
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
    teamItemFixture({}, { scopes: ["ws-compozy", "ws-advos"] }),
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

const CREW = [
  ["triage", "Triage Lead", "Watches issues and proposes the next step"],
  ["release-captain", "Release Captain", "Keeps the release train moving and the notes honest"],
  ["merge-gate", "Merge Gate", "Merges pull requests once their reviews and checks hold"],
  ["ci-medic", "CI Medic", "Reads failed CI runs and opens a fix for each flake"],
  ["docs-keeper", "Docs Keeper", "Updates the docs whenever a public surface changes"],
  ["deps-bot", "Deps Bot", "Bumps dependencies weekly and reads every changelog"],
  ["qa-walker", "QA Walker", "Walks the QA scenarios after each merge to main"],
  ["security-watch", "Security Watch", "Checks advisories against the lockfiles every morning"],
  ["perf-sentinel", "Perf Sentinel", "Compares benchmark runs and flags regressions"],
  ["inbox-zero", "Inbox Zero", "Sorts the support inbox and drafts first replies"],
  ["spec-scout", "Spec Scout", "Reads new specs and lists their open questions"],
  ["changelog", "Changelog Writer", "Turns merged pull requests into release notes"],
  ["design-review", "Design Review", "Compares UI changes against the design tokens"],
  ["copy-editor", "Copy Editor", "Checks product copy against the voice guide"],
  ["metrics-digest", "Metrics Digest", "Posts a daily digest of usage and errors"],
  ["backlog-groomer", "Backlog Groomer", "Closes stale issues and merges duplicates"],
  ["flake-hunter", "Flake Hunter", "Reruns suspicious tests until a flake shows itself"],
  ["onboarding", "Onboarding Guide", "Answers first-week questions from new contributors"],
  ["cost-watch", "Cost Watch", "Tracks provider spend and warns before a limit"],
  ["night-shift", "Night Shift", "Picks up small fixes while everyone is asleep"],
] as const;

/** A busy always-on crew, for density checks: twenty agents across three projects. */
export function crewItems(): AgentGroupedCatalogItem[] {
  return CREW.map(([name, label, description]) =>
    teamItemFixture(
      {
        name,
        label,
        description,
        definition_digest: `sha256:${name}`,
        resolved_appearance: { version: 1, avatar: "generated", seed: name, generation: 1 },
      },
      { kind: "always_on", scopes: ["@user", "ws-compozy", "ws-advos", "ws-reloaddev"] }
    )
  );
}

/** The crew's instances, plus one whose definition was deleted and that the page must not show. */
export function crewInstances(): AgentInstancePayload[] {
  const deleted = agentInstanceFixture({
    id: "ainst-orchestrator-full",
    agent_name: "orchestrator_full",
    label: "Orchestrator Full",
    state: "paused",
    pause_reason: "definition_missing",
    now: "",
  });
  return [deleted, ...crewInstancesOfCrew()];
}

function crewInstancesOfCrew(): AgentInstancePayload[] {
  return CREW.flatMap(([name, label], index) => {
    const workspace = TEAM_WORKSPACES[index % TEAM_WORKSPACES.length]!;
    const base = {
      id: `ainst-${name}`,
      agent_name: name,
      label,
      workspace_id: workspace.id,
      scope: workspace.name,
    };
    if (index % 7 === 1) return [agentInstanceFixture({ ...base, needs_attention: true })];
    if (index % 5 === 2) {
      return [agentInstanceFixture({ ...base, state: "paused", pause_reason: "manual", now: "" })];
    }
    if (index % 3 === 0) {
      return [agentInstanceFixture({ ...base, activity: "working", now: "Reading new issues" })];
    }
    return [agentInstanceFixture({ ...base, now: "" })];
  });
}
