// Suite: Automation request normalization
// Invariant: Loop-target requests carry the one workspace authorized by their Automation scope.
// Boundary IN: controlled Automation drafts and request projection.
// Boundary OUT: transport and daemon validation, owned by API suites.
import { describe, expect, it } from "vitest";

import {
  createAutomationJobDraft,
  createAutomationTriggerDraft,
  setJobTargetMode,
} from "../automation-drafts";
import {
  buildAutomationJobRequest,
  normalizeAutomationSchedule,
  projectAutomationJobRequest,
  projectAutomationTriggerRequest,
} from "../automation-requests";

function loopJobDraft() {
  return {
    ...setJobTargetMode(createAutomationJobDraft("ws_alpha"), "loop"),
    name: "daily-loop",
    loop_target: {
      workspace_id: "ws_stale",
      loop_name: "software-delivery",
      inputs: {},
      input_mapping: {},
    },
  };
}

describe("automation request normalization", () => {
  it("Should bind workspace-scoped Job Loop targets to the outer workspace", () => {
    const draft = loopJobDraft();

    const request = buildAutomationJobRequest(draft);

    expect(request.loop_target?.workspace_id).toBe("ws_alpha");
    expect(draft.loop_target.workspace_id).toBe("ws_stale");
  });

  it("Should preserve an explicit global Job Loop target workspace", () => {
    const draft = {
      ...loopJobDraft(),
      scope: "global" as const,
      workspace_id: undefined,
    };

    expect(buildAutomationJobRequest(draft).loop_target?.workspace_id).toBe("ws_stale");
  });

  it("Should project Job edits onto the exact mutable PATCH contract", () => {
    const projection = projectAutomationJobRequest(loopJobDraft(), "edit");

    expect(projection.method).toBe("PATCH");
    expect(projection.path).toBe("/api/automation/jobs/{id}");
    expect(projection.payload).not.toHaveProperty("scope");
    expect(projection.payload).not.toHaveProperty("workspace_id");
    expect(projection.payload).not.toHaveProperty("target_kind");
    expect(projection.payload).not.toHaveProperty("agent_name");
    expect(projection.payload).toMatchObject({
      loop_target: { workspace_id: "ws_alpha" },
      name: "daily-loop",
    });
  });

  it("Should project Trigger edits onto the exact mutable PATCH contract", () => {
    const projection = projectAutomationTriggerRequest(
      {
        ...createAutomationTriggerDraft("ws_alpha"),
        name: "review-completions",
      },
      "edit"
    );

    expect(projection.method).toBe("PATCH");
    expect(projection.path).toBe("/api/automation/triggers/{id}");
    expect(projection.payload).not.toHaveProperty("scope");
    expect(projection.payload).not.toHaveProperty("workspace_id");
    expect(projection.payload).not.toHaveProperty("target_kind");
    expect(projection.payload).not.toHaveProperty("agent_name");
    expect(projection.payload).toMatchObject({ name: "review-completions" });
  });
});

// Suite: Automation schedule normalization
// Invariant: The request serializes exactly the catch-up/grace fields the daemon
// accepts — recurring-only, target-aware default omitted, jitter default from a
// zero/empty grace, and never carried on a one-shot `at`.
describe("automation schedule normalization", () => {
  it("Should strip catch-up and grace from a one-shot at schedule", () => {
    const schedule = normalizeAutomationSchedule({
      mode: "at",
      time: "2026-04-17T18:20",
      catch_up_policy: "replay",
      misfire_grace_seconds: 45,
    });

    expect(schedule.mode).toBe("at");
    expect(typeof schedule.time).toBe("string");
    expect(schedule).not.toHaveProperty("catch_up_policy");
    expect(schedule).not.toHaveProperty("misfire_grace_seconds");
  });

  it("Should keep a recurring catch-up policy and a positive grace", () => {
    const schedule = normalizeAutomationSchedule({
      mode: "cron",
      expr: "0 9 * * *",
      catch_up_policy: "skip_missed",
      misfire_grace_seconds: 30,
    });

    expect(schedule.catch_up_policy).toBe("skip_missed");
    expect(schedule.misfire_grace_seconds).toBe(30);
  });

  it("Should drop a zero, negative, or omitted grace so the scheduler default applies", () => {
    expect(
      normalizeAutomationSchedule({ mode: "cron", expr: "0 9 * * *", misfire_grace_seconds: 0 })
        .misfire_grace_seconds
    ).toBeUndefined();
    expect(
      normalizeAutomationSchedule({ mode: "every", interval: "30m", misfire_grace_seconds: -5 })
        .misfire_grace_seconds
    ).toBeUndefined();
    expect(
      normalizeAutomationSchedule({ mode: "every", interval: "30m" }).misfire_grace_seconds
    ).toBeUndefined();
  });

  it("Should drop a fractional or non-finite grace so only whole positive seconds are sent", () => {
    expect(
      normalizeAutomationSchedule({ mode: "cron", expr: "0 9 * * *", misfire_grace_seconds: 3.5 })
        .misfire_grace_seconds
    ).toBeUndefined();
    expect(
      normalizeAutomationSchedule({
        mode: "every",
        interval: "30m",
        misfire_grace_seconds: Number.POSITIVE_INFINITY,
      }).misfire_grace_seconds
    ).toBeUndefined();
    expect(
      normalizeAutomationSchedule({
        mode: "cron",
        expr: "0 9 * * *",
        misfire_grace_seconds: Number.NaN,
      }).misfire_grace_seconds
    ).toBeUndefined();
  });

  it("Should leave an absent catch-up policy omitted for the daemon's target-aware default", () => {
    expect(
      normalizeAutomationSchedule({ mode: "cron", expr: "0 9 * * *" }).catch_up_policy
    ).toBeUndefined();
  });
});
