// Suite: the Agents page's view model
// Invariant: one member per definition, each holding only its own places; a stopped agent keeps
//   explaining itself; attention outranks everything else.
// Boundary IN: `buildAgentTeam`, overall state, filters. Boundary OUT: the catalog's grouping,
//   owned by `internal/api/core`.
import { describe, expect, it } from "vitest";

import {
  TEAM_WORKSPACES,
  teamInstances,
  teamItemFixture,
  teamItems,
} from "../../mocks/team-fixtures";
import { agentInstanceFixture } from "../../mocks/instance-fixtures";
import {
  agentNowLine,
  agentOverallState,
  agentPulse,
  agentStaysOn,
  agentUsageLine,
  agentsByAttention,
  buildAgentTeam,
  matchesAgentFilters,
  matchesAgentFocus,
} from "../agent-team";

const build = (items = teamItems(), instances = teamInstances()) =>
  buildAgentTeam({ items, instances, workspaces: TEAM_WORKSPACES });

describe("buildAgentTeam", () => {
  it("Should give each agent one member with the places it works", () => {
    const team = build();
    const triage = team.find(member => member.name === "triage")!;
    expect(team.filter(member => member.name === "triage")).toHaveLength(1);
    expect(triage.places.map(place => place.label).sort()).toEqual(["advos", "compozy"]);
  });

  it("Should keep same-slug definitions apart and label them by origin", () => {
    const team = build(
      [
        teamItemFixture({ definition_digest: "a" }, { definition_digest: "a" }),
        teamItemFixture(
          { definition_digest: "b" },
          { definition_digest: "b", scopes: ["ws-advos"] }
        ),
      ],
      [agentInstanceFixture({ id: "one", workspace_id: "ws-advos", scope: "advos" })]
    );
    expect(team.map(member => member.origin)).toEqual(["compozy", "advos"]);
    expect(team.map(member => member.places.length)).toEqual([0, 1]);
  });

  it("Should keep a card for an instance whose definition is gone", () => {
    const team = build(
      [],
      [agentInstanceFixture({ state: "paused", pause_reason: "definition_missing" })]
    );
    expect(team).toHaveLength(1);
    expect(team[0]).toMatchObject({ agent: null, kind: "stays_on" });
    expect(agentNowLine(team[0]!, null).text).toBe("Its setup was deleted");
  });

  it("Should keep the section of a definition that stopped being stays-on", () => {
    const team = build(
      [teamItemFixture({ kind: "on_demand" })],
      [agentInstanceFixture({ state: "paused", pause_reason: "definition_not_stays_on" })]
    );
    expect(agentStaysOn(team[0]!)).toBe(true);
  });

  it("Should name who owns an extension's agent", () => {
    const owner = build().find(member => member.name === "orchestrator")?.owner;
    expect(owner).toBe("An extension");
  });
});

describe("agent state across places", () => {
  it("Should let needs-you speak for an agent working elsewhere", () => {
    const member = build(
      [teamItemFixture()],
      [
        agentInstanceFixture({ id: "one", activity: "working" }),
        agentInstanceFixture({ id: "two", needs_attention: true, workspace_id: "ws-advos" }),
      ]
    )[0]!;
    expect(agentOverallState(member)).toBe("needs-you");
  });

  it("Should derive an on-demand agent's state from its running sessions", () => {
    const general = build().find(member => member.name === "general")!;
    expect(agentOverallState(general)).toBe("working");
    expect(agentOverallState({ ...general, active: 0 })).toBe("idle");
  });

  it("Should sort whoever needs you first", () => {
    const sorted = build().filter(agentStaysOn).sort(agentsByAttention);
    expect(sorted[0]?.name).toBe("orchestrator");
  });
});

describe("agent filters", () => {
  const team = build();

  it("Should match the label, the description and where it works", () => {
    const shown = (query: string) =>
      team
        .filter(member => matchesAgentFilters(member, query, [], TEAM_WORKSPACES))
        .map(member => member.name);
    expect(shown("triage")).toEqual(["triage"]);
    expect(shown("breaks")).toEqual([]);
    expect(shown("what could break")).toEqual(["reviewer"]);
    expect(shown("advos")).toContain("general");
  });

  it("Should narrow by project, model and maker", () => {
    const chip = (field: string, value: string) => ({
      id: field,
      field,
      operator: "is" as const,
      values: [value],
    });
    const shown = (field: string, value: string) =>
      team
        .filter(member => matchesAgentFilters(member, "", [chip(field, value)], TEAM_WORKSPACES))
        .map(member => member.name);
    expect(shown("project", "ws-advos")).toEqual(["triage", "general"]);
    expect(shown("owner", "extension")).toEqual(["orchestrator"]);
    expect(shown("model", "codex")).toEqual([]);
  });

  it("Should focus on the agents in a state", () => {
    const needing = team.filter(member => matchesAgentFocus(member, "needs-you"));
    expect(needing.map(member => member.name)).toEqual(["orchestrator"]);
  });
});

describe("agentUsageLine", () => {
  const now = Date.parse("2026-11-01T12:00:00Z");

  it("Should lead with what's running", () => {
    expect(agentUsageLine({ active: 2, sessions: 128 }, now)).toBe("2 running now · 128 sessions");
  });

  it("Should say when it last ran", () => {
    const line = agentUsageLine(
      { active: 0, sessions: 1, lastActivityAt: "2026-11-01T10:00:00Z" },
      now
    );
    expect(line).toBe("Last used 2h ago · 1 session");
  });

  it("Should only claim never used when metrics said so", () => {
    expect(agentUsageLine({ active: 0, sessions: 0 }, now)).toBe("Never used");
    expect(agentUsageLine({ active: 0 }, now)).toBe("Starts when you ask");
  });
});

describe("agentPulse", () => {
  it("Should say nothing when activity is unknown, rather than claim none", () => {
    expect(agentPulse(null)).toBeNull();
  });

  it("Should sum the last two weeks", () => {
    const pulse = agentPulse({
      reports_by_day: [{ day: "2026-10-30", quiet: 4, told: 3, needed_you: 1 }],
      cost_by_project: [],
      busy_by_hour: [],
    });
    expect(pulse).toEqual({ values: [8], reports: 4 });
  });
});
