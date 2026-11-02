import { HttpResponse, type HttpHandler } from "msw";
import { compozyApiMock } from "@/storybook/openapi-msw";

import { activityEntriesFixture, activityRollupFixture } from "./activity-fixtures";
import { homeActivityFixture, homeOverviewFixture } from "./fixtures";

export const handlers: HttpHandler[] = [
  compozyApiMock.get("/api/observe/overview", () =>
    HttpResponse.json({ overview: homeOverviewFixture })
  ),
  compozyApiMock.get("/api/permissions/rollup", ({ request }) =>
    HttpResponse.json(
      activityRollupFixture(new URL(request.url).searchParams.get("unattended") === "true")
    )
  ),
  compozyApiMock.get("/api/permissions", ({ request }) =>
    HttpResponse.json({
      entries: activityEntriesFixture(
        new URL(request.url).searchParams.get("unattended") === "true"
      ),
    })
  ),
  compozyApiMock.get("/api/logs", ({ request }) => {
    const limit = Number(new URL(request.url).searchParams.get("limit"));
    const events = limit > 0 ? homeActivityFixture.slice(0, limit) : homeActivityFixture;
    return HttpResponse.json({ events });
  }),
];
