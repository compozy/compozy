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

const allTriggerFixtures = [...automationTriggerFixtures, ...automationTriggerDetailFixtures];
const allRunFixtures = [...automationRunFixtures, ...automationTriggerDetailRunFixtures];
const jobById = new Map(automationJobFixtures.map(job => [job.id, job]));
const triggerById = new Map(allTriggerFixtures.map(trigger => [trigger.id, trigger]));

export const handlers: HttpHandler[] = [
  compozyApiMock.get("/api/automation/jobs", () =>
    HttpResponse.json({
      jobs: automationJobFixtures,
      page: {
        has_more: false,
        limit: 50,
        total: automationJobFixtures.length,
      },
    })
  ),
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
      runs: automationRunFixtures.filter(run => run.job_id === id),
    });
  }),
  compozyApiMock.get("/api/automation/triggers", () =>
    HttpResponse.json({
      page: {
        has_more: false,
        limit: 50,
        total: allTriggerFixtures.length,
      },
      triggers: allTriggerFixtures,
    })
  ),
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
