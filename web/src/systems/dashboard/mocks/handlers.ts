import { HttpResponse, type HttpHandler } from "msw";
import { compozyApiMock } from "@/storybook/openapi-msw";

import { homeActivityFixture, homeOverviewFixture } from "./fixtures";

export const handlers: HttpHandler[] = [
  compozyApiMock.get("/api/observe/overview", () =>
    HttpResponse.json({ overview: homeOverviewFixture })
  ),
  compozyApiMock.get("/api/logs", ({ request }) => {
    const limit = Number(new URL(request.url).searchParams.get("limit"));
    const events = limit > 0 ? homeActivityFixture.slice(0, limit) : homeActivityFixture;
    return HttpResponse.json({ events });
  }),
];
