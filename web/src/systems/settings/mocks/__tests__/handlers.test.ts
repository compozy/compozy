import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";

import { handlers } from "../handlers";

const server = setupServer(...handlers);
const API = "http://localhost";

beforeAll(() => {
  server.listen({ onUnhandledRequest: "error" });
});

afterEach(() => {
  server.resetHandlers();
});

afterAll(() => {
  server.close();
});

describe("settings shell MSW handlers", () => {
  it("Should preserve requested workspace and profile identities", async () => {
    const layoutResponse = await fetch(
      `${API}/api/workspaces/workspace-custom/window-manager/layout`
    );
    const profileResponse = await fetch(
      `${API}/api/workspaces/workspace-custom/window-manager/layout-profiles/profile-custom`,
      { method: "PUT", headers: { "content-type": "application/json" }, body: "{}" }
    );

    expect(layoutResponse.status).toBe(200);
    await expect(layoutResponse.json()).resolves.toMatchObject({
      workspace_id: "workspace-custom",
    });
    expect(profileResponse.status).toBe(200);
    await expect(profileResponse.json()).resolves.toMatchObject({
      record: {
        id: "profile-custom",
        scope: { kind: "workspace", id: "workspace-custom" },
        spec: {
          id: "profile-custom",
          document: { workspace_id: "workspace-custom" },
        },
      },
    });
  });
});

describe("settings MCP auth MSW handlers", () => {
  it.each([
    {
      label: "global exchange",
      action: "exchange",
      name: "linear-global",
      query: "scope=global",
      expected: {
        server_name: "linear-global",
        scope: "global",
        status: "authenticated",
        token_present: true,
      },
    },
    {
      label: "workspace exchange",
      action: "exchange",
      name: "not-linear",
      query: "scope=workspace&workspace_id=ws-beta",
      expected: {
        server_name: "not-linear",
        scope: "workspace",
        workspace_id: "ws-beta",
        status: "authenticated",
        token_present: true,
      },
    },
    {
      label: "global logout",
      action: "logout",
      name: "custom-provider",
      query: "scope=global",
      expected: {
        server_name: "custom-provider",
        scope: "global",
        status: "needs_login",
        token_present: false,
        diagnostic: "login required",
      },
    },
    {
      label: "workspace logout",
      action: "logout",
      name: "workspace-provider",
      query: "scope=workspace&workspace_id=ws-gamma",
      expected: {
        server_name: "workspace-provider",
        scope: "workspace",
        workspace_id: "ws-gamma",
        status: "needs_login",
        token_present: false,
        diagnostic: "login required",
      },
    },
  ])("Should derive the exact target for $label", async ({ action, name, query, expected }) => {
    const response = await fetch(
      `${API}/api/settings/mcp-servers/${encodeURIComponent(name)}/auth/${action}?${query}`,
      {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: action === "exchange" ? JSON.stringify({ code: "test-code" }) : undefined,
      }
    );

    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toMatchObject(expected);
  });
});
