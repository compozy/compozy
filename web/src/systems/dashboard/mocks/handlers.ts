import { http, HttpResponse, type HttpHandler } from "msw";

import type { DashboardPayload } from "../types";
import { dashboardFixture } from "./fixtures";

export interface DashboardHandlerOptions {
  dashboard?: DashboardPayload;
}

export function createDashboardHandlers(options: DashboardHandlerOptions = {}): HttpHandler[] {
  const dashboard = options.dashboard ?? dashboardFixture;

  return [http.get("/api/ui/dashboard", () => HttpResponse.json({ dashboard }))];
}

export const handlers = createDashboardHandlers();
