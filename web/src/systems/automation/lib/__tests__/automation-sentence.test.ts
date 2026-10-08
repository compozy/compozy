// Suite: Automation sentence and view projection
// Invariant: every surface reads one deterministic sentence and one view model per automation.
// Boundary IN: persisted jobs/triggers (board story fixtures) and editor drafts.
// Boundary OUT: rendering of segments.
import { describe, expect, it } from "vitest";

import { automationStoryJobs, automationStoryTriggers } from "../../mocks/story-fixtures";
import {
  automationSentenceIsIncomplete,
  automationSentenceText,
  describeAutomation,
  describeSchedule,
  type AutomationDraft,
} from "../automation-sentence";
import { toAutomationView } from "../automation-view";
import type { AutomationJob, AutomationTrigger } from "../../types";

const ctx = { workspaceName: () => "checkout-api" };
const job = (id: string) => automationStoryJobs.find(item => item.id === id) as AutomationJob;
const trigger = (id: string) =>
  automationStoryTriggers.find(item => item.id === id) as AutomationTrigger;
const text = (input: Parameters<typeof describeAutomation>[0]) =>
  automationSentenceText(describeAutomation(input, ctx));
const emphasized = (input: Parameters<typeof describeAutomation>[0]) =>
  describeAutomation(input, ctx)
    .filter(segment => segment.emphasis)
    .map(segment => segment.text);

function draft(overrides: Partial<AutomationDraft>): AutomationDraft {
  return {
    start: "schedule",
    schedule: { mode: "cron", expr: "0 9 * * 1-5" },
    target: { kind: "agent", agentName: "summarizer" },
    ...overrides,
  };
}

describe("describeAutomation", () => {
  it("Should read the board's seven automations as one sentence each", () => {
    expect(text(job("morning-digest"))).toBe(
      "Every weekday at 09:00 UTC, ask summarizer to summarize yesterday's sessions."
    );
    expect(emphasized(job("morning-digest"))).toEqual(["09:00 UTC", "summarizer"]);
    expect(text(job("nightly-delivery"))).toBe(
      "Every day at 02:00 UTC, start the Loop software-delivery on main."
    );
    expect(text(job("dependency-review"))).toBe(
      "Every Monday at 08:00 UTC, create a task Review dependency updates for reviewers."
    );
    // A blank task title is valid: the daemon titles the task with the job name.
    const untitled = job("dependency-review");
    expect(text({ ...untitled, task: { ...untitled.task!, title: "  " } })).toBe(
      "Every Monday at 08:00 UTC, create a task dependency-review for reviewers."
    );
    expect(text(job("release-checklist"))).toBe(
      "Every 30 minutes, ask release-manager to check the release checklist."
    );
    expect(text(trigger("summarize-failures"))).toBe(
      "When a session stops in checkout-api with an error, ask summarizer to explain what went wrong."
    );
    expect(text(trigger("rerun-delivery"))).toBe(
      "When a session stops in checkout-api with an error, start the Loop software-delivery."
    );
    expect(text(trigger("deploy-webhook"))).toBe(
      "When another app calls the deploy link with action deploy on main, ask deployer to ship it."
    );
  });

  it("Should read one-shot schedules in the automation time zone", () => {
    expect(
      describeSchedule({ mode: "at", time: "2026-10-08T09:00:00Z" }, { timeZone: "UTC" })
    ).toBe("Once on Thu Oct 8 at 09:00 UTC");
    expect(describeSchedule({ mode: "cron", expr: "*/15 * * * *" })).toBe("Every 15 minutes");
    expect(describeSchedule({ mode: "cron", expr: "0 9 1-7 * 1" })).toBe("On a custom schedule");
    expect(describeSchedule({ mode: "every", interval: "1h" })).toBe("Every hour");
    expect(describeSchedule(null)).toBe("Manual");
  });

  it("Should keep long or templated prompts out of the sentence", () => {
    const long = { ...job("morning-digest"), prompt: "x".repeat(200) };
    const templated = { ...job("morning-digest"), prompt: "Review {{ .Data.id }}" };
    expect(text(long)).toBe("Every weekday at 09:00 UTC, ask summarizer.");
    expect(text(templated)).toBe("Every weekday at 09:00 UTC, ask summarizer.");
  });

  it("Should mark missing draft parts so the editor reads Needs a fix", () => {
    const noAgent = describeAutomation(draft({ target: { kind: "agent" } }), ctx);
    expect(noAgent.find(segment => segment.missing)?.text).toBe("ask an agent");
    expect(automationSentenceIsIncomplete(noAgent)).toBe(true);

    const noDays = describeAutomation(
      draft({ schedule: { mode: "cron", expr: "0 9 * * *", days: [] } }),
      ctx
    );
    expect(noDays.find(segment => segment.missing)?.text).toBe("on some days");

    const emptyHook = describeAutomation(
      draft({ start: "event", event: "hook..completed", workspaceId: "ws" }),
      ctx
    );
    expect(emptyHook.find(segment => segment.missing)?.text).toBe("a hook");

    expect(automationSentenceIsIncomplete(describeAutomation(draft({}), ctx))).toBe(false);
    expect(text(draft({}))).toBe("Every weekday at 09:00 UTC, ask summarizer.");
  });
});

describe("toAutomationView", () => {
  it("Should project a job with its route, next run and Run now", () => {
    const view = toAutomationView(job("morning-digest"), ctx);
    expect(view).toMatchObject({
      kind: "job",
      start: "schedule",
      does: "agent",
      canRunNow: true,
      canEdit: true,
      detailPath: "/automations/jobs/morning-digest",
      nextRunAt: job("morning-digest").scheduler?.next_run_at,
      lastRun: { id: "run_morning_digest_118", status: "completed" },
    });
    const fallback = toAutomationView({ ...job("morning-digest"), scheduler: null }, ctx);
    expect(fallback.nextRunAt).toBe(job("morning-digest").next_run);
    expect(toAutomationView(job("dependency-review"), ctx).does).toBe("task");
  });

  it("Should project webhooks and managed sources without Run now or Edit", () => {
    const webhook = toAutomationView(trigger("deploy-webhook"), ctx);
    expect(webhook).toMatchObject({
      kind: "trigger",
      start: "webhook",
      canRunNow: false,
      detailPath: "/automations/triggers/deploy-webhook",
      publicLinkLive: true,
    });
    expect(toAutomationView(job("release-checklist"), ctx)).toMatchObject({
      source: "config",
      canEdit: false,
      lastRun: { skipReason: "self_overlap" },
    });
  });
});
