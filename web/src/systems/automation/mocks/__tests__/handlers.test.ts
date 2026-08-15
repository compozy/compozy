// Suite: Automation Storybook handlers
// Invariant: workspace-scoped suggestion reads never expose another workspace's records.
// Boundary IN: the MSW boundary used by Automation and full-app route stories.
// Boundary OUT: live daemon isolation owned by runtime API and E2E suites.
import { setupServer } from "msw/node";
import { afterAll, beforeAll, beforeEach, describe, expect, it } from "vitest";

import { storyWorkspaceIds } from "@/storybook/fintech-scenario";

import { handlers } from "../handlers";
import { automationSuggestionFixtures } from "../fixtures";

const server = setupServer(...handlers);
const API = "http://localhost";

beforeAll(() => {
  server.listen({ onUnhandledRequest: "error" });
});

beforeEach(() => {
  server.resetHandlers();
});

afterAll(() => {
  server.close();
});

describe("automation MSW handlers", () => {
  it.each([
    {
      expected: automationSuggestionFixtures.filter(suggestion => suggestion.status === "pending"),
      workspaceID: storyWorkspaceIds.hq,
    },
    {
      expected: [],
      workspaceID: storyWorkspaceIds.risk,
    },
  ])("Should return only suggestions owned by $workspaceID", async ({ expected, workspaceID }) => {
    const response = await fetch(
      `${API}/api/workspaces/${encodeURIComponent(workspaceID)}/automation/suggestions`
    );

    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toEqual({ suggestions: expected });
  });

  it.each([
    { query: "?status=accepted", status: "accepted" },
    { query: "?status=", status: "pending" },
  ])("Should filter workspace suggestions with $query", async ({ query, status }) => {
    const response = await fetch(
      `${API}/api/workspaces/${encodeURIComponent(storyWorkspaceIds.hq)}/automation/suggestions${query}`
    );

    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toEqual({
      suggestions: automationSuggestionFixtures.filter(
        suggestion =>
          suggestion.workspace_id === storyWorkspaceIds.hq && suggestion.status === status
      ),
    });
  });

  it("Should return structurally valid workspace suggestion payloads", async () => {
    const response = await fetch(
      `${API}/api/workspaces/${encodeURIComponent(storyWorkspaceIds.hq)}/automation/suggestions`
    );
    const body = (await response.json()) as { suggestions: typeof automationSuggestionFixtures };

    expect(response.status).toBe(200);
    for (const suggestion of body.suggestions) {
      expect(suggestion.payload.scope).toBe("workspace");
      expect(suggestion.payload.workspace_id).toBe(storyWorkspaceIds.hq);
    }
  });
});
