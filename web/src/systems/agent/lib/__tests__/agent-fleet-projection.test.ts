import { describe, expect, it } from "vitest";

import type { AgentCatalogItem, AgentPayload } from "../../types";
import { agentFixtures, FIXTURE_AGENT_DEFINITION_DIGEST } from "../../mocks/fixtures";
import {
  agentFleetChipsToFilters,
  agentFleetFiltersToChips,
  buildAgentFleetFilterFields,
} from "../agent-fleet-filters";
import {
  countAgentFleetCallInstances,
  formatAgentFleetCallInstanceLabel,
} from "../agent-fleet-call-instances";
import {
  formatAgentFleetAriaLabel,
  formatAgentFleetCardCategory,
  formatAgentFleetMeta,
  formatAgentLayer,
  agentShadowLayers,
  formatCategoryMetaSegment,
  projectAgentFleetRows,
} from "../agent-fleet-projection";
import { hasActiveAgentFleetFilters, validateAgentsFleetSearch } from "../agent-fleet-search";

function agent(overrides: Partial<AgentPayload> & Pick<AgentPayload, "name">): AgentPayload {
  return {
    provider: "claude",
    prompt: "test",
    description: "",
    scope: "global",
    shadowed: false,
    origin: "global",
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
    ...overrides,
  };
}

function catalogItem(
  definition: AgentPayload,
  sessions: AgentCatalogItem["sessions"] = {
    active: 0,
    failed: 0,
    runtime_seconds: 0,
    total: 0,
  }
): AgentCatalogItem {
  return { agent: definition, sessions };
}

describe("validateAgentsFleetSearch", () => {
  it("Should parse q, category, status, and view and drop invalid values", () => {
    expect(
      validateAgentsFleetSearch({
        q: "  release  ",
        category: "Engineering / Release",
        status: "idle",
        view: "cards",
      })
    ).toEqual({
      q: "release",
      category: "Engineering / Release",
      status: "idle",
      view: "cards",
    });
    expect(validateAgentsFleetSearch({ status: "running", q: "   ", view: "grid" })).toEqual({
      q: undefined,
      category: undefined,
      status: undefined,
      view: undefined,
    });
  });

  it("Should report active filters when any search facet is set", () => {
    expect(hasActiveAgentFleetFilters({})).toBe(false);
    expect(hasActiveAgentFleetFilters({ q: "x" })).toBe(true);
    expect(hasActiveAgentFleetFilters({ category: "Ops" })).toBe(true);
    expect(hasActiveAgentFleetFilters({ status: "active" })).toBe(true);
    expect(hasActiveAgentFleetFilters({ view: "cards" })).toBe(false);
  });
});

describe("agent fleet projection", () => {
  /**
   * Invariant: every shadowed fixture has a winning definition, and only the
   * winner can start a session. The fleet projection owns this action truth.
   */
  it("Should pair a shadowed definition with its winner and disable its session action", () => {
    const releaseDefinitions = agentFixtures.filter(
      agent => agent.name === "release-manager-agent"
    );
    const rows = projectAgentFleetRows({
      items: releaseDefinitions.map(definition => catalogItem(definition)),
      sessionsAvailable: true,
    });

    expect(rows).toHaveLength(2);
    expect(
      rows.map(row => [row.agent.origin, row.agent.shadowed, row.hasShadowing, row.canStartSession])
    ).toEqual([
      ["workspace", false, false, true],
      ["global", true, true, false],
    ]);
  });

  it("Should retain the backend page order and map exact session counts", () => {
    const agents = [agent({ name: "zeta" }), agent({ name: "alpha" }), agent({ name: "Beta" })];
    const rows = projectAgentFleetRows({
      items: [
        catalogItem(agents[1]!, { active: 0, failed: 0, runtime_seconds: 0, total: 4 }),
        catalogItem(agents[2]!, { active: 2, failed: 0, runtime_seconds: 120, total: 6 }),
        catalogItem(agents[0]!, { active: 0, failed: 0, runtime_seconds: 0, total: 0 }),
      ],
      sessionsAvailable: true,
    });
    expect(rows.map(row => row.agent.name)).toEqual(["alpha", "Beta", "zeta"]);
    expect(rows[0]?.signals).toEqual({ status: "idle", active: 0, total: 4 });
    expect(rows[1]?.signals).toEqual({ status: "active", active: 2, total: 6 });
  });

  it("Should omit invented status when sessions are unavailable", () => {
    const rows = projectAgentFleetRows({
      items: [
        catalogItem(
          agent({
            name: "coder",
            diagnostics: [{ error_kind: "parse", message: "bad", path: "AGENT.md" }],
          }),
          null
        ),
      ],
      sessionsAvailable: false,
    });
    expect(rows).toHaveLength(1);
    expect(rows[0]?.signals).toBeNull();
    expect(rows[0]?.sessionsAvailable).toBe(false);
    expect(rows[0]?.ariaLabel).toBe("coder");
    expect(rows[0]?.callInstances).toBeNull();
    expect(rows[0]?.hasDiagnostics).toBe(true);
  });

  it("Should render meta with category provider model and middle-truncate deep categories", () => {
    expect(
      formatAgentFleetMeta(
        agent({
          name: "release-captain",
          category_path: ["Engineering", "Release"],
          provider: "anthropic",
          model: "claude-sonnet-4-5",
          origin: "workspace",
        })
      )
    ).toBe("Engineering / Release · anthropic · claude-sonnet-4-5");

    expect(
      formatAgentFleetCardCategory(
        agent({
          name: "release-captain",
          category_path: ["Engineering", "Release"],
          provider: "anthropic",
          model: "claude-sonnet-4-5",
          origin: "workspace",
        })
      )
    ).toBe("Engineering / Release");

    expect(
      formatAgentFleetCardCategory(
        agent({
          name: "triage-bot",
          provider: "openai",
          origin: "global",
        })
      )
    ).toBe("openai");

    expect(
      formatCategoryMetaSegment(["Engineering", "Platform", "Infrastructure", "Release", "Canary"])
    ).toBe("Engineering / … / Canary");

    expect(
      formatCategoryMetaSegment([
        "ExtremelyLongFirstSegmentNameThatAloneWouldBlowPastTheLimit",
        "mid",
        "ExtremelyLongLastSegmentNameThatAlsoExceedsTheSharedBudgetAlone",
      ]).length
    ).toBeLessThanOrEqual(40);

    expect(
      formatCategoryMetaSegment([
        "ExtremelyLongSingleSegmentCategoryNameThatExceedsTheEllipsisLimitByDesign",
      ])
    ).toMatch(/…/);

    expect(formatAgentFleetAriaLabel(agent({ name: "release-captain" }), null)).toBe(
      "release-captain"
    );
    expect(
      formatAgentFleetAriaLabel(agent({ name: "release-captain" }), { running: 2, parked: 1 })
    ).toBe("release-captain, 2 running, 1 parked");
  });

  it("Should attach shared aria and card meta on projected rows", () => {
    const rows = projectAgentFleetRows({
      items: [
        catalogItem(
          agent({
            name: "triage-bot",
            provider: "openai",
            origin: "workspace",
          })
        ),
      ],
      sessionsAvailable: true,
    });
    expect(rows[0]?.cardCategory).toBe("openai");
    expect(rows[0]?.cardOrigin).toBe("Workspace");
    expect(rows[0]?.ariaLabel).toBe("triage-bot");
  });

  it("Should attach running and parked call-instance counts and omit zeros", () => {
    const rows = projectAgentFleetRows({
      items: [
        catalogItem(agent({ name: "reviewer" })),
        catalogItem(agent({ name: "scout" })),
        catalogItem(agent({ name: "docs-writer" })),
      ],
      sessionsAvailable: true,
      callInstances: new Map([
        ["reviewer", { running: 2, parked: 0 }],
        ["scout", { running: 0, parked: 1 }],
      ]),
    });

    expect(rows[0]?.callInstances).toEqual({ running: 2, parked: 0 });
    expect(rows[0]?.ariaLabel).toBe("reviewer, 2 running");
    expect(rows[1]?.callInstances).toEqual({ running: 0, parked: 1 });
    expect(rows[1]?.ariaLabel).toBe("scout, 1 parked");
    expect(rows[2]?.callInstances).toEqual({ running: 0, parked: 0 });
    expect(rows[2]?.ariaLabel).toBe("docs-writer");
    expect(formatAgentFleetCallInstanceLabel({ running: 0, parked: 0 })).toBeNull();
    expect(
      countAgentFleetCallInstances([
        { agent: "reviewer", state: "running" },
        { agent: "reviewer", state: "running" },
        { agent: "reviewer", state: "completed" },
        { agent: "scout", state: "queued" },
      ]).get("reviewer")
    ).toEqual({ running: 2, parked: 0 });
  });

  it("Should preserve the effective layer and every unique shadow layer", () => {
    const definition = agent({
      name: "profile-reviewer",
      origin: "workspace",
      layer: "project_profile",
      shadows: [
        { layer: "project", path: "/repo/.compozy/agents/profile-reviewer/AGENT.md" },
        { layer: "profile", path: "/home/profiles/dev/agents/profile-reviewer/AGENT.md" },
        { layer: "project", path: "/repo/.compozy/agents/duplicate/AGENT.md" },
      ],
    });
    const [row] = projectAgentFleetRows({
      items: [catalogItem(definition)],
      sessionsAvailable: true,
    });

    expect(formatAgentLayer(definition)).toBe("project_profile");
    expect(agentShadowLayers(definition)).toEqual(["project", "profile"]);
    expect(row?.layer).toBe("project_profile");
    expect(row?.shadowLayers).toEqual(["project", "profile"]);
    expect(row?.hasShadowing).toBe(true);
    expect(row?.canStartSession).toBe(true);
  });
});

describe("agent fleet filters", () => {
  it("Should build category and status fields and bridge chips both ways", () => {
    const fields = buildAgentFleetFilterFields(["Ops", "Engineering / Release"]);
    expect(fields).toEqual([
      {
        key: "category",
        label: "Category",
        type: "select",
        options: [
          { value: "Ops", label: "Ops" },
          { value: "Engineering / Release", label: "Engineering / Release" },
        ],
      },
      {
        key: "status",
        label: "Status",
        type: "select",
        options: [
          { value: "active", label: "Active" },
          { value: "idle", label: "Idle" },
        ],
      },
    ]);

    expect(
      agentFleetFiltersToChips({
        category: "Ops",
        status: "active",
      })
    ).toEqual([
      {
        id: "agent-fleet-filter-category",
        field: "category",
        operator: "is",
        values: ["Ops"],
      },
      {
        id: "agent-fleet-filter-status",
        field: "status",
        operator: "is",
        values: ["active"],
      },
    ]);

    expect(
      agentFleetChipsToFilters([
        {
          id: "agent-fleet-filter-category",
          field: "category",
          operator: "is",
          values: ["  Ops  "],
        },
        {
          id: "agent-fleet-filter-status",
          field: "status",
          operator: "is",
          values: ["idle"],
        },
      ])
    ).toEqual({ category: "Ops", status: "idle" });

    expect(
      agentFleetChipsToFilters([
        {
          id: "agent-fleet-filter-status",
          field: "status",
          operator: "is",
          values: ["running"],
        },
      ])
    ).toEqual({ category: undefined, status: undefined });
  });
});
