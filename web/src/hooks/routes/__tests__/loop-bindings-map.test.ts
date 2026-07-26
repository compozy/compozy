import { describe, expect, it } from "vitest";

import type { AutomationJob, AutomationTrigger } from "@/systems/automation";

import { buildLoopBindingIndex, jobToBindingRow, triggerToBindingRow } from "../loop-bindings-map";

const WS = "ws_default";

function job(overrides: Partial<AutomationJob>): AutomationJob {
  return {
    id: "job_1",
    name: "nightly",
    agent_name: "",
    prompt: "",
    scope: "workspace",
    source: "dynamic",
    target_kind: "loop",
    enabled: true,
    schedule: { mode: "cron", expr: "0 3 * * *" },
    next_run: "2026-07-05T21:00:00Z",
    retry: { strategy: "none", max_retries: 0, base_delay: "" },
    fire_limit: { max: 12, window: "1h" },
    created_at: "2026-07-05T00:00:00Z",
    updated_at: "2026-07-05T00:00:00Z",
    loop_target: { loop_name: "software-delivery", workspace_id: WS },
    ...overrides,
  } as AutomationJob;
}

function trigger(overrides: Partial<AutomationTrigger>): AutomationTrigger {
  return {
    id: "trg_1",
    name: "on-push",
    agent_name: "",
    prompt: "",
    event: "hook.review.completed",
    scope: "workspace",
    source: "dynamic",
    target_kind: "loop",
    enabled: true,
    retry: { strategy: "none", max_retries: 0, base_delay: "" },
    fire_limit: { max: 12, window: "1h" },
    webhook_secret_present: false,
    created_at: "2026-07-05T00:00:00Z",
    updated_at: "2026-07-05T00:00:00Z",
    loop_target: { loop_name: "software-delivery", workspace_id: WS },
    ...overrides,
  } as AutomationTrigger;
}

describe("loop-bindings-map", () => {
  it("Should map a loop-target job to a schedule binding with a cron + next-fire meta line", () => {
    const row = jobToBindingRow(job({}), WS);
    expect(row).toMatchObject({ id: "job_1", name: "nightly", kind: "schedule", enabled: true });
    expect(row?.meta).toContain("Cron 0 3 * * *");
    expect(row?.meta).toContain("next");
  });

  it("Should ignore agent-target jobs and cross-workspace loop targets", () => {
    expect(jobToBindingRow(job({ loop_target: null }), WS)).toBeNull();
    expect(
      jobToBindingRow(job({ loop_target: { loop_name: "x", workspace_id: "other" } }), WS)
    ).toBeNull();
  });

  it("Should derive the trigger binding kind (webhook vs trigger) from the trigger itself", () => {
    expect(triggerToBindingRow(trigger({}), WS)?.kind).toBe("trigger");
    const webhook = triggerToBindingRow(
      trigger({ webhook_id: "wbh_1", endpoint_slug: "push" }),
      WS
    );
    expect(webhook?.kind).toBe("webhook");
    expect(webhook?.meta).toBe("endpoint push");
  });

  it("Should index loops by name with distinct sorted kinds and per-loop rows", () => {
    const index = buildLoopBindingIndex(
      [trigger({ webhook_id: "wbh_1", endpoint_slug: "push" })],
      [job({}), job({ id: "job_2", name: "morning" })],
      WS
    );
    const entry = index.get("software-delivery");
    expect(entry?.kinds).toEqual(["schedule", "webhook"]);
    expect(entry?.rows).toHaveLength(3);
    // A different-workspace loop target must never leak into the index.
    const isolated = buildLoopBindingIndex(
      [],
      [job({ loop_target: { loop_name: "software-delivery", workspace_id: "other" } })],
      WS
    );
    expect(isolated.size).toBe(0);
  });
});
