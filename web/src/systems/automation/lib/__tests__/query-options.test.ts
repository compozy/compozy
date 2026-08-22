import { describe, expect, it } from "vitest";

import {
  automationJobDetailOptions,
  automationJobRunsOptions,
  automationJobsListOptions,
  automationRunsListOptions,
  automationSuggestionsListOptions,
  automationTriggerDetailOptions,
  automationTriggerRunsOptions,
  automationTriggersListOptions,
} from "../query-options";

describe("automation list options", () => {
  it("uses focus/invalidation reconciliation instead of polling every loaded page", () => {
    const jobOptions = automationJobsListOptions({ scope: "workspace", workspace_id: "ws_alpha" });
    const triggerOptions = automationTriggersListOptions({
      scope: "workspace",
      workspace_id: "ws_alpha",
    });

    expect(jobOptions.staleTime).toBe(15_000);
    expect(jobOptions.refetchInterval).toBeUndefined();
    expect(triggerOptions.staleTime).toBe(15_000);
    expect(triggerOptions.refetchInterval).toBeUndefined();
  });

  it("includes workspace filters in query keys", () => {
    const options = automationJobsListOptions({
      scope: "workspace",
      workspace_id: "ws_alpha",
      source: "dynamic",
      enabled: true,
      q: "review",
      limit: 10,
    });

    expect(options.queryKey).toEqual([
      "automation",
      "jobs",
      "list",
      "workspace",
      "ws_alpha",
      "dynamic",
      "true",
      "review",
      "10",
      "",
      // The profile axis rides the same key: two profiles reading one workspace
      // are two catalogs, so an absent selector must not share an entry with a
      // named one.
      "",
      "",
    ]);
    expect(options.initialPageParam).toBeUndefined();
  });

  it("separates two profiles' job catalogs and the aggregate from all of them", () => {
    const marketing = automationJobsListOptions({ workspace_id: "ws_alpha", profile: "marketing" });
    const consulting = automationJobsListOptions({
      workspace_id: "ws_alpha",
      profile: "consulting",
    });
    const aggregate = automationJobsListOptions({ workspace_id: "ws_alpha", all_profiles: true });

    expect(marketing.queryKey).not.toEqual(consulting.queryKey);
    expect(aggregate.queryKey).not.toEqual(marketing.queryKey);
    expect(aggregate.queryKey).not.toEqual(
      automationJobsListOptions({ workspace_id: "ws_alpha", profile: "default" }).queryKey
    );
  });

  it("caches the complete suggestion envelope by workspace and status", () => {
    const options = automationSuggestionsListOptions("ws_alpha", "pending");

    expect(options.queryKey).toEqual(["automation", "suggestions", "list", "ws_alpha", "pending"]);
    expect(options.enabled).toBe(true);
    expect("select" in options).toBe(false);
    expect(automationSuggestionsListOptions("", "pending").enabled).toBe(false);
  });
});

describe("automation detail and run options", () => {
  it("disables detail queries when ids are missing", () => {
    expect(automationJobDetailOptions("").enabled).toBe(false);
    expect(automationTriggerDetailOptions("").enabled).toBe(false);
  });

  it("uses a faster refetch cadence for run history", () => {
    const options = automationJobRunsOptions("job_1", { status: "running", limit: 5 });

    expect(options.refetchInterval).toBe(15_000);
    expect(options.enabled).toBe(true);
    expect(options.queryKey).toEqual([
      "automation",
      "jobs",
      "runs",
      "job_1",
      "running",
      "",
      "",
      "5",
    ]);
  });

  it("uses the trigger-run cadence for trigger history and global run history", () => {
    const triggerRuns = automationTriggerRunsOptions(
      "trg_1",
      { status: "failed", limit: 4 },
      false
    );
    const runs = automationRunsListOptions(
      {
        job_id: "job_1",
        trigger_id: "trg_1",
        status: "running",
        since: "2026-04-11T09:00:00Z",
        until: "2026-04-11T10:00:00Z",
        limit: 10,
      },
      false
    );

    expect(triggerRuns.refetchInterval).toBe(15_000);
    expect(triggerRuns.enabled).toBe(false);
    expect(triggerRuns.queryKey).toEqual([
      "automation",
      "triggers",
      "runs",
      "trg_1",
      "failed",
      "",
      "",
      "4",
    ]);

    expect(runs.refetchInterval).toBe(15_000);
    expect(runs.enabled).toBe(false);
    expect(runs.queryKey).toEqual([
      "automation",
      "runs",
      "list",
      "job_1",
      "trg_1",
      "running",
      "2026-04-11T09:00:00Z",
      "2026-04-11T10:00:00Z",
      "10",
    ]);
  });
});
