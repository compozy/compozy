// Suite: Automation detail models
// Invariant: the run list, Last ran and Inspect render only what the daemon recorded — an open
// link needs the id it points at, a duration needs both timestamps, a scheduled reservation is
// not a run that started, and the sample event overlays every exact-match path.
// Boundary IN: persisted runs, jobs and triggers. Boundary OUT: rendering (automation-detail-panel).
import { describe, expect, it } from "vitest";

import { automationLastRanAt } from "../automation-detail";
import { buildAutomationRunView } from "../automation-run-model";
import {
  buildAutomationDiagnostics,
  buildTriggerEnvelopeSample,
} from "../automation-inspect-model";
import {
  deployWebhookTrigger,
  makeDetailRun,
  makeDetailTrigger,
  morningDigestJob,
  nightlyDeliveryJob,
} from "../../mocks/detail-fixtures";

describe("buildAutomationRunView", () => {
  it.each([
    { name: "session", run: makeDetailRun(), link: { kind: "session", id: "sess_9f2a1c" } },
    {
      name: "task",
      run: makeDetailRun({ status: "delegated", session_id: undefined, task_id: "task_42" }),
      link: { kind: "task", id: "task_42" },
    },
    {
      name: "nothing recorded",
      run: makeDetailRun({ status: "scheduled", session_id: undefined }),
      link: null,
    },
  ])("Should open only what the daemon recorded: $name", ({ run, link }) => {
    const view = buildAutomationRunView(run, morningDigestJob);
    if (link === null) expect(view.link).toBeNull();
    else expect(view.link).toMatchObject(link);
  });

  it.each([
    { name: "both timestamps", run: makeDetailRun(), duration: "42s" },
    {
      name: "no end",
      run: makeDetailRun({ status: "running", ended_at: undefined }),
      duration: "—",
    },
    { name: "no start", run: makeDetailRun({ started_at: undefined }), duration: "—" },
  ])("Should show a duration only with $name", ({ run, duration }) => {
    expect(buildAutomationRunView(run, morningDigestJob).duration).toBe(duration);
  });

  it("Should hand a job's Loop run to its loop run in the Loop's project", () => {
    const view = buildAutomationRunView(
      makeDetailRun({
        status: "delegated",
        job_id: "nightly-delivery",
        session_id: undefined,
        loop_run_id: "looprun_8f3a2b",
        ended_at: undefined,
      }),
      nightlyDeliveryJob
    );

    expect(view.statusLabel).toBe("Handed off");
    expect(view.glyph).toBe("delegated");
    expect(view.link).toEqual({
      kind: "loop-run",
      id: "looprun_8f3a2b",
      label: "Open loop run",
      workspaceId: "ws_checkout_api",
    });
    expect(view.drawerLines[0]?.text).toBe(
      "Handed to software-delivery. The Loop owns the rest of the run."
    );
  });
});

describe("automationLastRanAt", () => {
  it("Should skip scheduled reservations and runs without a start", () => {
    expect(
      automationLastRanAt([
        makeDetailRun({ id: "a", started_at: "2026-10-05T09:00:00Z" }),
        makeDetailRun({ id: "b", status: "scheduled", started_at: "2026-10-09T09:00:00Z" }),
        makeDetailRun({ id: "c", started_at: undefined }),
        makeDetailRun({ id: "d", started_at: "2026-10-06T09:00:00Z" }),
      ])
    ).toBe("2026-10-06T09:00:00Z");
    expect(automationLastRanAt([])).toBeNull();
  });
});

describe("Inspect model", () => {
  it("Should overlay nested data.* filter paths and envelope fields onto the sample event", () => {
    const sample = buildTriggerEnvelopeSample(
      makeDetailTrigger({
        event: "ext.release.completed",
        filter: { source: "extension", "data.metadata.step": "complete" },
      })
    );

    expect(sample.kind).toBe("ext.release.completed");
    expect(sample.source).toBe("extension");
    expect(sample.workspace_id).toBe("ws_checkout_api");
    expect(sample.data).toEqual(expect.objectContaining({ metadata: { step: "complete" } }));
    expect(buildTriggerEnvelopeSample(deployWebhookTrigger).data).toEqual(
      expect.objectContaining({ action: "deploy", branch: "main", endpoint: "deploy--wbh_abc123" })
    );
  });

  it.each([
    { entity: morningDigestJob, kind: "job · cron", target: "agent · summarizer" },
    { entity: nightlyDeliveryJob, kind: "job · cron", target: "loop · software-delivery" },
    { entity: deployWebhookTrigger, kind: "trigger · webhook", target: "agent · deployer" },
  ])("Should name $entity.name in the daemon's terms", ({ entity, kind, target }) => {
    const tiles = Object.fromEntries(
      buildAutomationDiagnostics(entity).map(tile => [tile.id, tile.value])
    );
    expect(tiles.kind).toBe(kind);
    expect(tiles.target).toBe(target);
  });
});
