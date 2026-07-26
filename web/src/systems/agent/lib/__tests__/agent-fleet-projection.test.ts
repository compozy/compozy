import { describe, expect, it } from "vitest";

import type { AgentCatalogItem, AgentPayload } from "../../types";
import { FIXTURE_AGENT_DEFINITION_DIGEST } from "../../mocks/fixtures";
import {
  agentFleetChipsToFilters,
  agentFleetFiltersToChips,
  buildAgentFleetFilterFields,
} from "../agent-fleet-filters";
import {
  formatAgentFleetAriaLabel,
  formatAgentFleetCardCategory,
  formatAgentFleetMeta,
  formatCategoryMetaSegment,
  projectAgentFleetRows,
} from "../agent-fleet-projection";
import { hasActiveAgentFleetFilters, validateAgentsFleetSearch } from "../agent-fleet-search";

function agent(overrides: Partial<AgentPayload> & Pick<AgentPayload, "name">): AgentPayload {
  return {
    provider: "claude",
    prompt: "test",
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
    expect(rows[0]?.ariaLabel).toBe("coder, session status unavailable");
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

    expect(
      formatAgentFleetAriaLabel(
        agent({ name: "release-captain" }),
        {
          status: "active",
          active: 2,
          total: 6,
        },
        true
      )
    ).toBe("release-captain, Active, 2 of 6 sessions active");
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
    expect(rows[0]?.ariaLabel).toBe("triage-bot, Idle, 0 of 0 sessions active");
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
