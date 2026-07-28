import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  automationRunSkipReason,
  automationScopeTone,
  automationSkipReasonDetail,
  automationSkipReasonLabel,
  automationSkipReasonTone,
  automationSourceLabel,
  automationSourceTone,
  automationStatusTone,
  catchUpPolicyLabel,
  describeFireLimit,
  describeRetry,
  describeSchedule,
  describeTrigger,
  formatAutomationListSummary,
  formatDate,
  formatDateTime,
  formatPromptPreview,
  formatRelativeTime,
  formatRunDuration,
  formatRunTitle,
} from "../automation-formatters";

const triggerFixture = {
  id: "trg_push_review",
  name: "push-review",
  agent_name: "reviewer",
  prompt: "Review push event {{ .Data.branch }}.",
  event: "webhook",
  filter: { "data.branch": "main" },
  scope: "workspace" as const,
  workspace_id: "ws_alpha",
  source: "dynamic" as const,
  target_kind: "agent",
  enabled: true,
  retry: { strategy: "backoff" as const, max_retries: 4, base_delay: "5s" },
  fire_limit: { max: 12, window: "1h" },
  endpoint_slug: "push-review",
  webhook_id: "wbh_push_review",
  webhook_secret_present: true,
  created_at: "2026-04-11T08:00:00Z",
  updated_at: "2026-04-11T08:10:00Z",
};

describe("automation formatter helpers", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-04-11T10:00:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("formats relative times across empty, invalid, future, and past values", () => {
    expect(formatRelativeTime()).toBe("Not scheduled");
    expect(formatRelativeTime("not-a-date")).toBe("not-a-date");
    expect(formatRelativeTime("2026-04-11T10:00:00Z")).toBe("Now");
    expect(formatRelativeTime("2026-04-11T10:20:00Z")).toBe("In 20m");
    expect(formatRelativeTime("2026-04-11T08:00:00Z")).toBe("2h ago");
    expect(formatRelativeTime("2026-04-14T10:00:00Z")).toBe("In 3d");
  });

  it("formats calendar times and falls back when dates are missing or invalid", () => {
    expect(formatDate()).toBe("Unavailable");
    expect(formatDate("not-a-date")).toBe("not-a-date");
    expect(formatDate("2026-04-11T08:10:00Z")).toContain("Apr 11, 2026");
    expect(formatDateTime()).toBe("Unavailable");
    expect(formatDateTime("not-a-date")).toBe("not-a-date");
    expect(formatDateTime("2026-04-11T08:10:00Z")).toContain("Apr 11, 2026");
  });

  it("describes schedules for every supported mode", () => {
    expect(describeSchedule()).toBe("Manual");
    expect(describeSchedule({ mode: "cron", expr: "0 9 * * *" })).toBe("Cron 0 9 * * *");
    expect(describeSchedule({ mode: "cron" })).toBe("Cron");
    expect(describeSchedule({ mode: "every", interval: "30m" })).toBe("Every 30m");
    expect(describeSchedule({ mode: "every" })).toBe("Every interval");
    expect(describeSchedule({ mode: "at", time: "2026-04-11T12:00:00Z" })).toContain("At Apr 11");
    expect(describeSchedule({ mode: "at" })).toBe("One-shot");
  });

  it("describes webhook and non-webhook triggers", () => {
    expect(describeTrigger({ ...triggerFixture, event: "ext.github.push" })).toBe(
      "ext.github.push"
    );
    expect(describeTrigger(triggerFixture)).toBe("webhook:push-review");
    expect(describeTrigger({ ...triggerFixture, endpoint_slug: undefined })).toBe(
      "webhook:wbh_push_review"
    );
    expect(
      describeTrigger({ ...triggerFixture, endpoint_slug: undefined, webhook_id: undefined })
    ).toBe("webhook");
  });

  it("formats retry, fire-limit, run-title, status, and source labels", () => {
    expect(describeRetry({ strategy: "none", max_retries: 3, base_delay: "2s" })).toBe(
      "No retries"
    );
    expect(describeRetry({ strategy: "backoff", max_retries: 4, base_delay: "5s" })).toBe(
      "4 retries from 5s"
    );
    expect(describeFireLimit({ max: 12, window: "1h" })).toBe("12 fires / 1h");
    expect(formatRunTitle({ status: "running", attempt: 2 } as never)).toBe("RUNNING · attempt 2");
    expect(
      formatRunDuration({
        started_at: "2026-04-11T10:00:00Z",
        ended_at: "2026-04-11T10:02:14Z",
      } as never)
    ).toBe("2m 14s");
    expect(
      formatPromptPreview("Review the session transcript and summarize follow-up actions.", 20)
    ).toBe("Review the session...");
    expect(automationStatusTone("running")).toBe("info");
    expect(automationStatusTone("completed")).toBe("success");
    expect(automationStatusTone("enabled")).toBe("success");
    expect(automationStatusTone("scheduled")).toBe("warning");
    expect(automationStatusTone("failed")).toBe("danger");
    expect(automationStatusTone("canceled")).toBe("neutral");
    expect(automationStatusTone("disabled")).toBe("neutral");
    expect(automationScopeTone("workspace")).toBe("info");
    expect(automationScopeTone("global")).toBe("neutral");
    expect(automationSourceTone("dynamic")).toBe("info");
    expect(automationSourceTone("config")).toBe("neutral");
    expect(automationSourceLabel("config")).toBe("CONFIG");
    expect(automationSourceLabel("package")).toBe("PACKAGE");
    expect(automationSourceLabel("dynamic")).toBe("DYNAMIC");
  });

  it("formats exact totals while disclosing partially loaded pages", () => {
    expect(
      formatAutomationListSummary({
        activeWorkspaceName: "alpha",
        kind: "jobs",
        scopeFilter: "workspace",
        searchQuery: "",
        totalCount: 4,
        visibleCount: 1,
      })
    ).toBe("Showing 1 of 4 jobs in alpha");
    expect(
      formatAutomationListSummary({
        kind: "jobs",
        scopeFilter: "global",
        searchQuery: "",
        totalCount: 4,
        visibleCount: 2,
      })
    ).toBe("Showing 2 of 4 jobs in global scope");
    expect(
      formatAutomationListSummary({
        kind: "jobs",
        scopeFilter: "all",
        searchQuery: "nightly",
        totalCount: 4,
        visibleCount: 2,
      })
    ).toBe("Showing 2 of 4 jobs matching current search");
    expect(
      formatAutomationListSummary({
        kind: "triggers",
        scopeFilter: "all",
        searchQuery: "",
        totalCount: 0,
        visibleCount: 0,
      })
    ).toBe("0 triggers found");
  });

  it("labels catch-up policies and never surfaces the removed skip value", () => {
    expect(catchUpPolicyLabel()).toBe("Default");
    expect(catchUpPolicyLabel(undefined)).toBe("Default");
    expect(catchUpPolicyLabel("skip_missed")).toBe("Skip missed");
    expect(catchUpPolicyLabel("coalesce")).toBe("Coalesce");
    expect(catchUpPolicyLabel("replay")).toBe("Replay");
    expect(catchUpPolicyLabel("run_once_on_catchup")).toBe("Run once");
    expect(catchUpPolicyLabel("skip_missed")).not.toBe("skip");
  });

  it("recognizes durable skip reasons only on canceled runs and maps label, tone, and detail", () => {
    expect(
      automationRunSkipReason({ status: "canceled", metadata: { reason: "self_overlap" } } as never)
    ).toBe("self_overlap");
    expect(
      automationRunSkipReason({
        status: "canceled",
        metadata: { reason: "misfire_grace_exceeded" },
      } as never)
    ).toBe("misfire_grace_exceeded");

    // A known reason on a non-canceled run is ignored (no invented skip status).
    expect(
      automationRunSkipReason({
        status: "completed",
        metadata: { reason: "self_overlap" },
      } as never)
    ).toBeNull();
    expect(
      automationRunSkipReason({
        status: "running",
        metadata: { reason: "misfire_grace_exceeded" },
      } as never)
    ).toBeNull();

    // Canceled runs without a known reason are not skips either.
    expect(
      automationRunSkipReason({ status: "canceled", metadata: { reason: "unknown" } } as never)
    ).toBeNull();
    expect(automationRunSkipReason({ status: "canceled", metadata: {} } as never)).toBeNull();
    expect(automationRunSkipReason({ status: "canceled" } as never)).toBeNull();

    expect(automationSkipReasonLabel("self_overlap")).toBe("Overlap");
    expect(automationSkipReasonLabel("misfire_grace_exceeded")).toBe("Grace window");
    expect(automationSkipReasonTone("self_overlap")).toBe("info");
    expect(automationSkipReasonTone("misfire_grace_exceeded")).toBe("warning");
    expect(automationSkipReasonDetail("self_overlap")).toContain("previous run");
    expect(automationSkipReasonDetail("misfire_grace_exceeded")).toContain("grace window");
  });
});
