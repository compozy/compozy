import { HttpResponse, type HttpHandler } from "msw";
import { compozyApiMock } from "@/storybook/openapi-msw";

import {
  schedulerBacklogFixture,
  schedulerDrainResultFixture,
  schedulerPausedStatusFixture,
  schedulerStatusFixture,
} from "./fixtures";

export const handlers: HttpHandler[] = [
  compozyApiMock.get("/api/scheduler", () =>
    HttpResponse.json({ scheduler: schedulerStatusFixture })
  ),
  compozyApiMock.post("/api/scheduler/pause", async ({ request }) => {
    const body = (await request.json().catch(() => ({}))) as { reason?: string };
    return HttpResponse.json({
      scheduler: {
        ...schedulerPausedStatusFixture,
        paused_reason: body.reason ?? schedulerPausedStatusFixture.paused_reason,
      },
    });
  }),
  compozyApiMock.post("/api/scheduler/resume", () =>
    HttpResponse.json({ scheduler: { ...schedulerStatusFixture, paused: false } })
  ),
  compozyApiMock.post("/api/scheduler/drain", () => HttpResponse.json(schedulerDrainResultFixture)),
  compozyApiMock.get("/api/scheduler/backlog", () =>
    HttpResponse.json({ backlog: schedulerBacklogFixture })
  ),
];
