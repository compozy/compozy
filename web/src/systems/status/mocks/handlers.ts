import { HttpResponse, type HttpHandler } from "msw";
import { compozyApiMock } from "@/storybook/openapi-msw";

import { statusFixture } from "./fixtures";

export const handlers: HttpHandler[] = [
  compozyApiMock.get("/api/status", () =>
    HttpResponse.json(statusFixture, {
      headers: { "X-Compozy-Gateway-Tier": "local" },
    })
  ),
  compozyApiMock.get("/api/doctor", () =>
    HttpResponse.json({
      schema_version: "2026-05-20",
      generated_at: statusFixture.generated_at,
      duration_ms: 12,
      status: "ok",
      summary: {
        total: 0,
        counts_by_severity: {},
      },
      items: [],
    })
  ),
];
