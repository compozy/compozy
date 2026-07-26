import { HttpResponse, type HttpHandler } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";

import {
  schedulerBacklogFixture,
  schedulerDrainResultFixture,
  schedulerPausedStatusFixture,
  schedulerStatusFixture,
} from "./fixtures";

export const handlers: HttpHandler[] = [
  aghApiMock.get("/api/scheduler", () => HttpResponse.json({ scheduler: schedulerStatusFixture })),
  aghApiMock.post("/api/scheduler/pause", async ({ request }) => {
    const body = (await request.json().catch(() => ({}))) as { reason?: string };
    return HttpResponse.json({
      scheduler: {
        ...schedulerPausedStatusFixture,
        paused_reason: body.reason ?? schedulerPausedStatusFixture.paused_reason,
      },
    });
  }),
  aghApiMock.post("/api/scheduler/resume", () =>
    HttpResponse.json({ scheduler: { ...schedulerStatusFixture, paused: false } })
  ),
  aghApiMock.post("/api/scheduler/drain", () => HttpResponse.json(schedulerDrainResultFixture)),
  aghApiMock.get("/api/scheduler/backlog", () =>
    HttpResponse.json({ backlog: schedulerBacklogFixture })
  ),
];
