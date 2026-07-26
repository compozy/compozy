import { HttpResponse, type HttpHandler } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";

import {
  automationJobFixtures,
  automationRunFixtures,
  automationTriggerFixtures,
  primaryAutomationJobFixture,
  primaryAutomationTriggerFixture,
} from "./fixtures";

const jobById = new Map(automationJobFixtures.map(job => [job.id, job]));
const triggerById = new Map(automationTriggerFixtures.map(trigger => [trigger.id, trigger]));

export const handlers: HttpHandler[] = [
  aghApiMock.get("/api/automation/jobs", () =>
    HttpResponse.json({
      jobs: automationJobFixtures,
      page: {
        has_more: false,
        limit: 50,
        total: automationJobFixtures.length,
      },
    })
  ),
  aghApiMock.get("/api/automation/jobs/{id}", ({ params }) => {
    const id = String(params.id);
    const job = jobById.get(id);

    if (!job) {
      return HttpResponse.json({ error: `Automation job not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({ job });
  }),
  aghApiMock.post("/api/automation/jobs", async ({ request }) => {
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
  aghApiMock.patch("/api/automation/jobs/{id}", async ({ params, request }) => {
    const id = String(params.id);
    const job = jobById.get(id);

    if (!job) {
      return HttpResponse.json({ error: `Automation job not found: ${id}` }, { status: 404 });
    }

    const body = (await request.json()) as Partial<typeof primaryAutomationJobFixture>;
    return HttpResponse.json({ job: { ...job, ...body, id } });
  }),
  aghApiMock.delete("/api/automation/jobs/{id}", ({ params }) => {
    const id = String(params.id);

    if (!jobById.has(id)) {
      return HttpResponse.json({ error: `Automation job not found: ${id}` }, { status: 404 });
    }

    return new HttpResponse(null, { status: 204 });
  }),
  aghApiMock.post("/api/automation/jobs/{id}/trigger", ({ params }) => {
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
  aghApiMock.get("/api/automation/jobs/{id}/runs", ({ params }) => {
    const id = String(params.id);

    if (!jobById.has(id)) {
      return HttpResponse.json({ error: `Automation job not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({
      runs: automationRunFixtures.filter(run => run.job_id === id),
    });
  }),
  aghApiMock.get("/api/automation/triggers", () =>
    HttpResponse.json({
      page: {
        has_more: false,
        limit: 50,
        total: automationTriggerFixtures.length,
      },
      triggers: automationTriggerFixtures,
    })
  ),
  aghApiMock.get("/api/automation/triggers/{id}", ({ params }) => {
    const id = String(params.id);
    const trigger = triggerById.get(id);

    if (!trigger) {
      return HttpResponse.json({ error: `Automation trigger not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({ trigger });
  }),
  aghApiMock.post("/api/automation/triggers", async ({ request }) => {
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
  aghApiMock.patch("/api/automation/triggers/{id}", async ({ params, request }) => {
    const id = String(params.id);
    const trigger = triggerById.get(id);

    if (!trigger) {
      return HttpResponse.json({ error: `Automation trigger not found: ${id}` }, { status: 404 });
    }

    const body = (await request.json()) as Partial<typeof primaryAutomationTriggerFixture>;
    return HttpResponse.json({ trigger: { ...trigger, ...body, id } });
  }),
  aghApiMock.delete("/api/automation/triggers/{id}", ({ params }) => {
    const id = String(params.id);

    if (!triggerById.has(id)) {
      return HttpResponse.json({ error: `Automation trigger not found: ${id}` }, { status: 404 });
    }

    return new HttpResponse(null, { status: 204 });
  }),
  aghApiMock.get("/api/automation/triggers/{id}/runs", ({ params }) => {
    const id = String(params.id);

    if (!triggerById.has(id)) {
      return HttpResponse.json({ error: `Automation trigger not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({
      runs: automationRunFixtures.filter(run => run.trigger_id === id),
    });
  }),
  aghApiMock.get("/api/automation/runs", () => HttpResponse.json({ runs: automationRunFixtures })),
];
