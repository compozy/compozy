import { http, HttpResponse, type HttpHandler } from "msw";
import { describe, expect, it } from "vitest";

import { handlers as appShellHandlers } from "@/systems/app-shell/mocks";
import { emptyDashboardFixture, handlers as dashboardHandlers } from "@/systems/dashboard/mocks";
import { handlers as memoryHandlers } from "@/systems/memory/mocks";
import { handlers as reviewsHandlers } from "@/systems/reviews/mocks";
import { handlers as runsHandlers } from "@/systems/runs/mocks";
import { handlers as specHandlers } from "@/systems/spec/mocks";
import { handlers as workflowsHandlers } from "@/systems/workflows/mocks";

const { storybookSystemHandlerGroups, storybookSystemHandlers } =
  await import("../../.storybook/preview");
const { composeStorybookHandlerGroup, flattenStorybookHandlerGroups, storybookMswParameters } =
  await import("./msw");

function handlerSignature(handler: HttpHandler) {
  const method = String(handler.info.method);
  const path = String(handler.info.path);
  return `${method} ${path}`;
}

describe("web Storybook MSW contract", () => {
  it("publishes grouped default handlers for every daemon web UI domain", () => {
    expect(storybookSystemHandlerGroups).toEqual({
      appShell: appShellHandlers,
      dashboard: dashboardHandlers,
      workflows: workflowsHandlers,
      runs: runsHandlers,
      reviews: reviewsHandlers,
      spec: specHandlers,
      memory: memoryHandlers,
    });
    expect(storybookSystemHandlers).toEqual(
      flattenStorybookHandlerGroups(storybookSystemHandlerGroups)
    );

    for (const handlers of Object.values(storybookSystemHandlerGroups)) {
      expect(handlers.length).toBeGreaterThan(0);
    }
  });

  it("replaces only matching method/path pairs when a story overrides one handler in a group", () => {
    const override = http.get("/api/ui/dashboard", () =>
      HttpResponse.json({ dashboard: emptyDashboardFixture })
    );
    const composed = composeStorybookHandlerGroup("dashboard", [override]);
    const signatures = composed.map(handlerSignature);

    expect(composed[0]).toBe(override);
    expect(signatures.filter(signature => signature === "GET /api/ui/dashboard")).toHaveLength(1);
    expect(composed.length).toBe(dashboardHandlers.length);
  });

  it("wraps grouped overrides in Storybook's expected msw parameter shape", () => {
    const params = storybookMswParameters({
      dashboard: [
        http.get("/api/ui/dashboard", () =>
          HttpResponse.json({ dashboard: emptyDashboardFixture })
        ),
      ],
    });

    expect(params).toEqual({
      msw: {
        handlers: {
          dashboard: expect.any(Array),
        },
      },
    });
  });

  it("does not register duplicate method/path handlers across the combined system set", () => {
    const signatures = storybookSystemHandlers.map(handlerSignature);
    const uniqueSignatures = new Set(signatures);

    expect(uniqueSignatures.size).toBe(signatures.length);
  });
});
