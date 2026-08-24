import { describe, expect, it } from "vitest";

import { automationKeys } from "../query-keys";

describe("automationKeys", () => {
  it("separates job, trigger, and run namespaces", () => {
    // What matters is that the three families can never collide, not how many
    // filter dimensions each currently folds in.
    expect(automationKeys.jobList().slice(0, 3)).toEqual(["automation", "jobs", "list"]);
    expect(automationKeys.triggerList().slice(0, 3)).toEqual(["automation", "triggers", "list"]);
    expect(automationKeys.runList().slice(0, 3)).toEqual(["automation", "runs", "list"]);
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
    ]);
  });

  it("uses one canonical lens segment for named, default, and aggregate reads", () => {
    const named = automationKeys.jobList({ profile: "growth" });
    const omitted = automationKeys.jobList();
    const explicitFalse = automationKeys.jobList({ all_profiles: false });
    const aggregate = automationKeys.jobList({ all_profiles: true });

    expect(named.at(-1)).toBe("growth");
    expect(omitted).toEqual(explicitFalse);
    expect(aggregate.at(-1)).toBe("@all");
    expect(aggregate).not.toEqual(named);
  });

  it("includes status filters in run-history keys", () => {
    expect(
      automationKeys.jobRuns("job_1", { status: "running", limit: 5, profile: "marketing" })
    ).toEqual(["automation", "jobs", "runs", "job_1", "running", "", "", "5", "marketing"]);
    expect(
      automationKeys.triggerRuns("trg_1", { status: "failed", limit: 2, all_profiles: true })
    ).toEqual(["automation", "triggers", "runs", "trg_1", "failed", "", "", "2", "@all"]);
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
