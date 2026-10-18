import { HttpResponse, type HttpHandler } from "msw";
import { compozyApiMock } from "@/storybook/openapi-msw";

import {
  automationJobFixtures,
  automationRunFixtures,
  automationSuggestionFixtures,
  automationTriggerDetailFixtures,
  automationTriggerDetailRunFixtures,
  automationTriggerFixtures,
  primaryAutomationJobFixture,
  primaryAutomationTriggerFixture,
} from "./fixtures";
import {
  automationStoryJobs,
  automationStoryRuns,
  automationStoryTriggers,
} from "./story-fixtures";
import { automationDoesOf } from "../lib/automation-sentence";
import type { AutomationJob, AutomationTrigger } from "../types";

const allTriggerFixtures = [
  ...automationTriggerFixtures,
  ...automationTriggerDetailFixtures,
  ...automationStoryTriggers,
];
const allRunFixtures = [
  ...automationRunFixtures,
  ...automationTriggerDetailRunFixtures,
  ...automationStoryRuns,
];
const jobById = new Map(
  [...automationJobFixtures, ...automationStoryJobs].map(job => [job.id, job])
);

/** Server-side list filters the daemon applies (`q`, `enabled`, `scope`, `source`, `target`, `loop`). */
function filterAutomations<T extends AutomationJob | AutomationTrigger>(
  items: readonly T[],
  url: string
): T[] {
  const params = new URL(url).searchParams;
  const q = params.get("q")?.trim().toLowerCase();
  const enabled = params.get("enabled");
  const scope = params.get("scope");
  const source = params.get("source");
  const target = params.get("target");
  const loop = params.get("loop");
  return items.filter(
    item =>
      (!q ||
        [item.name, item.agent_name, item.prompt, "event" in item ? item.event : ""].some(field =>
          field.toLowerCase().includes(q)
        )) &&
      (enabled === null || String(item.enabled) === enabled) &&
      (!scope || item.scope === scope) &&
      (!source || item.source === source) &&
      (!target || automationDoesOf(item) === target) &&
      (!loop || item.loop_target?.loop_name === loop)
  );
}
const triggerById = new Map(allTriggerFixtures.map(trigger => [trigger.id, trigger]));

export const handlers: HttpHandler[] = [
  compozyApiMock.get("/api/automation/jobs", ({ request }) => {
    const jobs = filterAutomations(automationStoryJobs, request.url);
    return HttpResponse.json({
      jobs,
      page: { has_more: false, limit: 50, total: jobs.length },
    });
  }),
  compozyApiMock.get("/api/automation/jobs/{id}", ({ params }) => {
    const id = String(params.id);
    const job = jobById.get(id);

    if (!job) {
      return HttpResponse.json({ error: `Automation job not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({ job });
  }),
  compozyApiMock.post("/api/automation/jobs", async ({ request }) => {
    const body = (await request.json()) as Partial<typeof primaryAutomationJobFixture>;
    return HttpResponse.json(
      {
        job: {
          ...primaryAutomationJobFixture,
          ...body,
          id: body.name
            ? `job_${String(body.name).replace(/[^a-zA-Z0-9]+/g, "_")}`
            : primaryAutomationJobFixture.id,
        },
      },
      { status: 201 }
    );
  }),
  compozyApiMock.patch("/api/automation/jobs/{id}", async ({ params, request }) => {
    const id = String(params.id);
    const job = jobById.get(id);

    if (!job) {
      return HttpResponse.json({ error: `Automation job not found: ${id}` }, { status: 404 });
    }

    const body = (await request.json()) as Partial<typeof primaryAutomationJobFixture>;
    return HttpResponse.json({ job: { ...job, ...body, id } });
  }),
  compozyApiMock.delete("/api/automation/jobs/{id}", ({ params }) => {
    const id = String(params.id);

    if (!jobById.has(id)) {
      return HttpResponse.json({ error: `Automation job not found: ${id}` }, { status: 404 });
    }

    return new HttpResponse(null, { status: 204 });
  }),
  compozyApiMock.post("/api/automation/jobs/{id}/trigger", ({ params }) => {
    const id = String(params.id);

    if (!jobById.has(id)) {
      return HttpResponse.json({ error: `Automation job not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({
      run: {
        ...automationRunFixtures[0],
        id: `run_${id}_manual`,
        job_id: id,
        status: "scheduled",
      },
    });
  }),
  compozyApiMock.get("/api/automation/jobs/{id}/runs", ({ params }) => {
    const id = String(params.id);

    if (!jobById.has(id)) {
      return HttpResponse.json({ error: `Automation job not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({
      runs: allRunFixtures.filter(run => run.job_id === id),
    });
  }),
  compozyApiMock.get("/api/automation/triggers", ({ request }) => {
    const triggers = filterAutomations(automationStoryTriggers, request.url);
    return HttpResponse.json({
      page: { has_more: false, limit: 50, total: triggers.length },
      triggers,
    });
  }),
  compozyApiMock.get("/api/automation/triggers/{id}", ({ params }) => {
    const id = String(params.id);
    const trigger = triggerById.get(id);

    if (!trigger) {
      return HttpResponse.json({ error: `Automation trigger not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({ trigger });
  }),
  compozyApiMock.post("/api/automation/triggers", async ({ request }) => {
    const body = (await request.json()) as Partial<typeof primaryAutomationTriggerFixture>;
    return HttpResponse.json(
      {
        trigger: {
          ...primaryAutomationTriggerFixture,
          ...body,
          id: body.name
            ? `trg_${String(body.name).replace(/[^a-zA-Z0-9]+/g, "_")}`
            : primaryAutomationTriggerFixture.id,
        },
      },
      { status: 201 }
    );
  }),
  compozyApiMock.patch("/api/automation/triggers/{id}", async ({ params, request }) => {
    const id = String(params.id);
    const trigger = triggerById.get(id);

    if (!trigger) {
      return HttpResponse.json({ error: `Automation trigger not found: ${id}` }, { status: 404 });
    }

    const body = (await request.json()) as Partial<typeof primaryAutomationTriggerFixture>;
    return HttpResponse.json({ trigger: { ...trigger, ...body, id } });
  }),
  compozyApiMock.delete("/api/automation/triggers/{id}", ({ params }) => {
    const id = String(params.id);

    if (!triggerById.has(id)) {
      return HttpResponse.json({ error: `Automation trigger not found: ${id}` }, { status: 404 });
    }

    return new HttpResponse(null, { status: 204 });
  }),
  compozyApiMock.get("/api/automation/triggers/{id}/runs", ({ params }) => {
    const id = String(params.id);

    if (!triggerById.has(id)) {
      return HttpResponse.json({ error: `Automation trigger not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({
      runs: allRunFixtures.filter(run => run.trigger_id === id),
    });
  }),
  compozyApiMock.get("/api/automation/runs", () => HttpResponse.json({ runs: allRunFixtures })),
  compozyApiMock.get(
    "/api/workspaces/{workspace_id}/automation/suggestions",
    ({ params, request }) => {
      const status = new URL(request.url).searchParams.get("status")?.trim() || "pending";
      return HttpResponse.json({
        suggestions: automationSuggestionFixtures.filter(
          suggestion =>
            suggestion.workspace_id === String(params.workspace_id) && suggestion.status === status
        ),
      });
    }
  ),
];
