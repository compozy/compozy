import { HttpResponse, type HttpHandler } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";

import { statusFixture } from "./fixtures";

export const handlers: HttpHandler[] = [
  aghApiMock.get("/api/status", () => HttpResponse.json(statusFixture)),
  aghApiMock.get("/api/doctor", () =>
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
