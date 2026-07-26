import { HttpResponse, type HttpHandler } from "msw";

import { aghApiMock } from "@/storybook/openapi-msw";

import { toolApprovalGrantsResponseFixture } from "./fixtures";

/**
 * Default (stateless) handlers for Storybook and shared fixtures. The daemon owns
 * workspace scoping, so the mock returns the fixture envelope regardless of the
 * requested `workspace_id`. Stories override GET for empty/error/loading variants;
 * behavioral set/revoke flows use stateful stores in component tests.
 */
export const handlers: HttpHandler[] = [
  aghApiMock.put("/api/tool-approval-grants", () =>
    HttpResponse.json({ grant: toolApprovalGrantsResponseFixture.grants[0]! })
  ),
  aghApiMock.get("/api/tool-approval-grants", () =>
    HttpResponse.json(toolApprovalGrantsResponseFixture)
  ),
  aghApiMock.delete("/api/tool-approval-grants/{id}", ({ params }) => {
    const id = String(params.id);
    const exists = toolApprovalGrantsResponseFixture.grants.some(grant => grant.id === id);
    if (!exists) {
      return HttpResponse.json({ error: "Workspace or approval grant not found" }, { status: 404 });
    }
    return new HttpResponse(null, { status: 204 });
  }),
];
