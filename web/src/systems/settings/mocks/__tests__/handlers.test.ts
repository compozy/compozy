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

  it.each([
    {
      label: "user",
      query: "scope=user",
      expected: { scope: "user" },
    },
    {
      label: "profile",
      query: "scope=profile&profile=marketing&workspace_id=ws-alpha",
      expected: { scope: "profile", profile: "marketing", workspace_id: "ws-alpha" },
    },
    {
      label: "workspace",
      query: "scope=workspace&workspace_id=ws-beta",
      expected: { scope: "workspace", workspace_id: "ws-beta" },
    },
  ])("Should preserve the requested $label hook layer", async ({ query, expected }) => {
    const response = await fetch(`${API}/api/settings/hooks?${query}`);

    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toMatchObject({
      available_scopes: ["user", "profile", "workspace"],
      collection: "hooks",
      ...expected,
    });
  });

  it.each([
    {
      label: "user",
      query: "scope=user",
      expected: { scope: "user", config: { agent: "general", provider: "claude" } },
    },
    {
      label: "profile",
      query: "scope=profile&profile=marketing&workspace_id=ws-alpha",
      expected: {
        scope: "profile",
        profile: "marketing",
        workspace_id: "ws-alpha",
        config: { agent: "campaigns", provider: "openai" },
      },
    },
  ])("Should preserve the requested $label persona layer", async ({ query, expected }) => {
    const response = await fetch(`${API}/api/settings/persona?${query}`);

    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toMatchObject({
      available_scopes: ["user", "profile", "workspace"],
      section: "persona",
      ...expected,
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
        scope: "user",
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
        scope: "user",
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
        body:
          action === "exchange"
            ? JSON.stringify({
                redirect_url: "http://127.0.0.1:2123/api/mcp/oauth/callback?code=test&state=x",
              })
            : undefined,
      }
    );

    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toMatchObject(expected);
  });
});
