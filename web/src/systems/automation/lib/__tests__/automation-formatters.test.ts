import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  automationLastRunLabel,
  automationLastRunMeta,
  automationRunSkipReason,
  automationScopeTone,
  automationSkipReasonLabel,
  automationSourceLabel,
  automationSourceTone,
  automationRunStateGlyph,
  catchUpPolicyLabel,
  describeFireLimit,
  describeRetry,
  describeTrigger,
  describeRetryPlain,
  formatDate,
  formatDateTime,
  formatPromptPreview,
  formatRelativeTime,
  formatRunDuration,
  formatRunTitle,
  humanizeFireWindow,
} from "../automation-formatters";

const triggerFixture = {
  profile_id: "00000000000000000000000000",
  profile_name: "default",
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

  it("describes webhook and non-webhook triggers", () => {
    expect(
      describeTrigger({
        ...triggerFixture,
        event: "ext.github.push",
      })
    ).toBe("ext.github.push");
    expect(describeTrigger(triggerFixture)).toBe("webhook:push-review");
    expect(
      describeTrigger({
        ...triggerFixture,
        endpoint_slug: undefined,
      })
    ).toBe("webhook:wbh_push_review");
    expect(
      describeTrigger({
        ...triggerFixture,
        endpoint_slug: undefined,
        webhook_id: undefined,
      })
    ).toBe("webhook");
  });

  it("formats retry, fire-limit, run-title, status, and source labels", () => {
    expect(describeRetry({ strategy: "none", max_retries: 3, base_delay: "2s" })).toBe(
      "No retries"
    );
    expect(describeRetry({ strategy: "backoff", max_retries: 4, base_delay: "5s" })).toBe(
      "Up to 4 retries, first after 5s"
    );
    expect(describeFireLimit({ max: 12, window: "1h" })).toBe("Up to 12 runs per hour");
    expect(describeFireLimit({ max: 1, window: "1h" })).toBe("Up to 1 run per hour");
    expect(formatRunTitle({ status: "running", attempt: 2 } as never)).toBe("Running · attempt 2");
    expect(
      formatRunDuration({
        started_at: "2026-04-11T10:00:00Z",
        ended_at: "2026-04-11T10:02:14Z",
      } as never)
    ).toBe("2m 14s");
    expect(
      formatPromptPreview("Review the session transcript and summarize follow-up actions.", 20)
    ).toBe("Review the session...");
    expect(automationRunStateGlyph("scheduled")).toBe("queued");
    expect(automationRunStateGlyph("running")).toBe("running");
    expect(automationRunStateGlyph("delegated")).toBe("delegated");
    expect(automationRunStateGlyph("completed")).toBe("done");
    expect(automationRunStateGlyph("failed")).toBe("failed");
    expect(automationRunStateGlyph("canceled")).toBe("stopped");
    expect(automationScopeTone("workspace")).toBe("neutral");
    expect(automationScopeTone("global")).toBe("neutral");
    expect(automationSourceTone("dynamic")).toBe("neutral");
    expect(automationSourceTone("config")).toBe("neutral");
    expect(automationSourceLabel("config")).toBe("From config");
    expect(automationSourceLabel("package")).toBe("From package");
    expect(automationSourceLabel("dynamic")).toBe("Created here");
  });

  it("Should format reliability in plain words across singular, plural, and unknown windows", () => {
    expect(humanizeFireWindow("1h")).toBe("hour");
    expect(humanizeFireWindow("30m")).toBe("30 minutes");
    expect(humanizeFireWindow("calendar-day")).toBe("calendar-day");
    expect(describeRetryPlain({ strategy: "none", max_retries: 0, base_delay: "" })).toBe(
      "No retries"
    );
    expect(describeRetryPlain({ strategy: "backoff", max_retries: 2, base_delay: "5s" })).toBe(
      "Up to 2, waiting longer each time"
    );
  });

  it("labels catch-up policies and never surfaces the removed skip value", () => {
    expect(catchUpPolicyLabel()).toBe("Default");
    expect(catchUpPolicyLabel(undefined)).toBe("Default");
    expect(catchUpPolicyLabel("skip_missed")).toBe("Skip missed");
    expect(catchUpPolicyLabel("coalesce")).toBe("Catch up once");
    expect(catchUpPolicyLabel("replay")).toBe("Run every missed time");
    expect(catchUpPolicyLabel("run_once_on_catchup")).toBe("Run once, then continue");
    expect(catchUpPolicyLabel("skip_missed")).not.toBe("skip");
  });

  it("recognizes durable skip reasons only on canceled runs and maps the label", () => {
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

    expect(automationSkipReasonLabel("self_overlap")).toBe("Skipped");
    expect(automationSkipReasonLabel("misfire_grace_exceeded")).toBe("Missed");
  });

  it("labels a last run's durable skip before its status", () => {
    expect(automationLastRunLabel({ status: "canceled", skipReason: "self_overlap" })).toBe(
      "Skipped"
    );
    expect(
      automationLastRunLabel({ status: "canceled", skipReason: "misfire_grace_exceeded" })
    ).toBe("Missed");
    expect(automationLastRunLabel({ status: "canceled" })).toBe("Canceled");
  });

  it("words the row's last-run truth with danger only for failures", () => {
    const at = "2026-10-07T02:00:01Z";
    expect(automationLastRunMeta({ status: "failed", startedAt: at })).toEqual({
      tone: "danger",
      glyph: "fail",
      text: "Last run failed",
      at,
    });
    expect(automationLastRunMeta({ status: "canceled", skipReason: "self_overlap" })).toEqual({
      tone: "neutral",
      glyph: "skip",
      text: "Last run skipped — the one before was still going",
    });
    expect(
      automationLastRunMeta({ status: "canceled", skipReason: "misfire_grace_exceeded" })?.text
    ).toBe("Last run missed — CompozyOS was off at the start time");
    expect(automationLastRunMeta({ status: "canceled", startedAt: at })?.text).toBe(
      "Last run canceled"
    );
    expect(automationLastRunMeta({ status: "running", startedAt: at })).toEqual({
      tone: "neutral",
      glyph: null,
      text: "Running now",
    });
    expect(automationLastRunMeta({ status: "delegated", startedAt: at })?.text).toBe(
      "Last run handed off"
    );
    expect(automationLastRunMeta({ status: "completed", startedAt: at })?.text).toBe(
      "Last run completed"
    );
    expect(automationLastRunMeta(undefined)).toBeNull();
  });
});
