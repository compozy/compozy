// Suite: palette domain search projection
// Invariant: domain rows honor workspace scope filters, per-domain landing
// routes, server catalog totals, and an optional visible cap without a numeric
// sentinel for uncapped pushed views.
// Owning layer: unit — the domain-search helpers.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

import type { CmdPaletteRankSignals } from "../cmd-palette-types";
import {
  agentRoute,
  bridgeRoute,
  knowledgeRoute,
  jobRoute,
  loopRoute,
  marketplaceCatalogTotal,
  marketplaceEntryRoute,
  networkChannelRoute,
  paletteTaskFilters,
  paletteWorkspaceCatalogFilters,
  section,
  taskRoute,
  terminalRoute,
  triggerRoute,
  vaultRoute,
  workspaceLabel,
} from "../os-palette-domain-search";

const TEST_WEIGHTS = JSON.parse(
  readFileSync(
    resolve(process.cwd(), "../internal/cmdpalette/testdata/ranking_weights_v1.json"),
    "utf8"
  )
) as CmdPaletteRankSignals["weights"];

const SIGNALS: CmdPaletteRankSignals = {
  profile_lens: { profile_lens_id: "00000000000000000000000000", profile_name: "default" },
  weights: TEST_WEIGHTS,
  usage: [],
  query_hits: [],
  pins: [],
  revision: "ps_test",
};

function seed(label: string, index: number) {
  return {
    stableKey: `task:${index}`,
    id: `task:${index}`,
    label,
    group: "Tasks",
    subtype: "path" as const,
    row: {
      key: `task:${index}`,
      label,
      app: "tasks" as const,
      route: taskRoute(`task-${index}`),
    },
  };
}

describe("os-palette-domain-search helpers", () => {
  it("Should build identity-bearing routes for concrete entities", () => {
    expect(taskRoute("task/42")).toEqual({ pathname: "/tasks/task%2F42", search: {} });
    expect(loopRoute("Release / Ops", "ws-a")).toEqual({
      pathname: "/loops/Release%20%2F%20Ops",
      search: { workspace: "ws-a" },
    });
    expect(jobRoute("job-42")).toEqual({ pathname: "/jobs/job-42", search: {} });
    expect(triggerRoute("trigger-42")).toEqual({ pathname: "/triggers/trigger-42", search: {} });
    expect(bridgeRoute("bridge-42")).toEqual({ pathname: "/bridges/bridge-42", search: {} });
    expect(agentRoute("agent/ops")).toEqual({ pathname: "/agents/agent%2Fops", search: {} });
    expect(terminalRoute("term/4f21")).toEqual({
      pathname: "/terminal/term%2F4f21",
      search: {},
    });

    const sameNameRoutes = ["task-a", "task-b"].map(id => taskRoute(id));
    expect(sameNameRoutes).toEqual([
      { pathname: "/tasks/task-a", search: {} },
      { pathname: "/tasks/task-b", search: {} },
    ]);
  });

  it("Should carry marketplace scope and installed identity into the detail route", () => {
    expect(
      marketplaceEntryRoute({
        entryId: "ops/extension",
        installedName: "ops-extension",
        kind: "extension",
        scope: "workspace",
        workspaceId: "ws-a",
      })
    ).toEqual({
      pathname: "/marketplace/extension/ops%2Fextension",
      search: {
        installed_name: "ops-extension",
        scope: "workspace",
        workspace_id: "ws-a",
      },
    });
  });

  it("Should select a Vault secret by ref instead of filtering by its display name", () => {
    expect(vaultRoute("vault:providers/ops/api-token")).toEqual({
      pathname: "/vault",
      search: { ref: "vault:providers/ops/api-token" },
    });
  });

  it("Should land a network channel on the channel threads location", () => {
    expect(networkChannelRoute("ws-a", "ops")).toEqual({
      pathname: "/network/ws-a/ops/threads",
      search: {},
    });
  });

  it("Should emit a memory search key for Knowledge rows", () => {
    expect(
      knowledgeRoute({ filename: "notes.md", scope: "workspace", workspaceId: "ws-a" })
    ).toEqual({
      pathname: "/knowledge",
      search: { memory: "notes.md", scope: "workspace", workspace: "ws-a" },
    });
  });

  it("Should prefer marketplace kind totals over loaded item counts", () => {
    expect(
      marketplaceCatalogTotal([
        { items: [{}, {}], total: 40 },
        { items: [{}], total: null },
      ])
    ).toBe(41);
  });

  it("Should scope catalog filters to one workspace or every workspace", () => {
    expect(paletteTaskFilters("workspace", "ws-a")).toEqual({
      scope: "workspace",
      workspace: "ws-a",
    });
    expect(paletteTaskFilters("global", "ws-home")).toEqual({ scope: "all" });
    expect(paletteWorkspaceCatalogFilters("workspace", "ws-a")).toEqual({
      scope: "workspace",
      workspace_id: "ws-a",
    });
    expect(paletteWorkspaceCatalogFilters("global", "ws-home")).toEqual({});
  });

  it("Should label workspace rows only while the globe is selected", () => {
    const names = new Map([["ws-a", "Alpha"]]);
    expect(workspaceLabel("workspace", "ws-a", names)).toBeUndefined();
    expect(workspaceLabel("global", "ws-a", names)).toBe("Alpha");
    expect(workspaceLabel("global", undefined, names)).toBe("Global");
  });

  it("Should keep the server catalog total when the visible list is capped", () => {
    const seeds = Array.from({ length: 4 }, (_, index) => seed(`Task ${index}`, index));
    const projected = section(
      "Tasks",
      seeds,
      { isLoading: false, isError: false, error: null },
      true,
      "",
      SIGNALS,
      { limit: 2, catalogTotal: 40 }
    );
    expect(projected.rows).toHaveLength(2);
    expect(projected.total).toBe(40);
  });

  it("Should leave a pushed domain uncapped when no limit is supplied", () => {
    const seeds = Array.from({ length: 4 }, (_, index) => seed(`Task ${index}`, index));
    const projected = section(
      "Tasks",
      seeds,
      { isLoading: false, isError: false, error: null },
      true,
      "",
      SIGNALS
    );
    expect(projected.rows).toHaveLength(4);
    expect(projected.total).toBe(4);
  });
});
