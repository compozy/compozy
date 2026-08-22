import { describe, expect, it } from "vitest";

import { automationKeys } from "../query-keys";

describe("automationKeys", () => {
  it("separates job, trigger, and run namespaces", () => {
    // What matters is that the three families can never collide, not how many
    // filter dimensions each currently folds in.
    expect(automationKeys.jobList().slice(0, 3)).toEqual(["automation", "jobs", "list"]);
    expect(automationKeys.triggerList().slice(0, 3)).toEqual(["automation", "triggers", "list"]);
    expect(automationKeys.runList().slice(0, 3)).toEqual(["automation", "runs", "list"]);
    const namespaces = [
      automationKeys.jobList(),
      automationKeys.triggerList(),
      automationKeys.runList(),
    ].map(key => JSON.stringify(key));
    expect(new Set(namespaces).size).toBe(3);
  });

  it("distinguishes workspace-scoped filters in list keys", () => {
    expect(automationKeys.jobList({ scope: "workspace", workspace_id: "ws_alpha" })).toEqual([
      "automation",
      "jobs",
      "list",
      "workspace",
      "ws_alpha",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
    ]);
    expect(automationKeys.jobList({ scope: "workspace", workspace_id: "ws_beta" })).toEqual([
      "automation",
      "jobs",
      "list",
      "workspace",
      "ws_beta",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
    ]);
  });

  it("includes package, enabled, search and loop filters but never a cursor", () => {
    expect(
      automationKeys.jobList({
        enabled: false,
        limit: 25,
        loop: "delivery",
        q: "review",
        source: "package",
      })
    ).toEqual([
      "automation",
      "jobs",
      "list",
      "",
      "",
      "package",
      "false",
      "review",
      "25",
      "delivery",
      "",
      "",
    ]);
  });

  it("includes status filters in run-history keys", () => {
    expect(automationKeys.jobRuns("job_1", { status: "running", limit: 5 })).toEqual([
      "automation",
      "jobs",
      "runs",
      "job_1",
      "running",
      "",
      "",
      "5",
    ]);
    expect(automationKeys.triggerRuns("trg_1", { status: "failed", limit: 2 })).toEqual([
      "automation",
      "triggers",
      "runs",
      "trg_1",
      "failed",
      "",
      "",
      "2",
    ]);
  });

  it("keys suggestion lists by exact workspace and status", () => {
    expect(automationKeys.suggestionList("ws_alpha", "pending")).toEqual([
      "automation",
      "suggestions",
      "list",
      "ws_alpha",
      "pending",
    ]);
    expect(automationKeys.suggestionList("ws_beta", "dismissed")).toEqual([
      "automation",
      "suggestions",
      "list",
      "ws_beta",
      "dismissed",
    ]);
  });
});
