import { afterEach, beforeEach, describe, it, vi } from "vitest";

import { expectFetchRequest, mockJsonResponse } from "@/test/fetch-test-utils";
import {
  beginSettingsMCPAuth,
  exchangeSettingsMCPAuth,
  logoutSettingsMCPAuth,
} from "../settings-mcp-auth-api";

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("MCP auth adapter", () => {
  // Invariant: every OAuth request preserves the chosen definition owner.
  // Owner: Settings HTTP adapter; canonical suite: settings-mcp-auth-api.test.ts.
  it("carries the extension owner through begin, exchange, and logout", async () => {
    const filter = { scope: "user" as const, owner: " extension:linear " };
    const requests = [
      {
        action: "begin",
        body: { mode: "automatic" as const },
        call: () => beginSettingsMCPAuth("linear", filter, { mode: "automatic" }),
      },
      {
        action: "exchange",
        body: { redirect_url: "https://callback.example/?code=x&state=y" },
        call: () =>
          exchangeSettingsMCPAuth("linear", filter, {
            redirect_url: "https://callback.example/?code=x&state=y",
          }),
      },
      { action: "logout", body: undefined, call: () => logoutSettingsMCPAuth("linear", filter) },
    ];
    for (const request of requests) {
      vi.mocked(globalThis.fetch).mockClear();
      mockJsonResponse({});
      await request.call();
      await expectFetchRequest({
        method: "POST",
        body: request.body,
        path: `/api/settings/mcp-servers/linear/auth/${request.action}?scope=user&owner=extension%3Alinear`,
      });
    }
  });
  it("begins MCP auth with the scoped target", async () => {
    mockJsonResponse({
      authorization_url: "https://auth.linear.app/oauth/authorize?state=x",
      callback_url: "http://127.0.0.1:2123/api/mcp/oauth/callback",
      expires_at: "2026-07-15T00:05:00Z",
      manual_supported: true,
      state: "compozy_mcp_x",
    });

    await beginSettingsMCPAuth(
      "linear",
      { scope: "workspace", workspace_id: "  ws_alpha  " },
      { mode: "automatic" }
    );

    await expectFetchRequest({
      body: { mode: "automatic" },
      method: "POST",
      path: "/api/settings/mcp-servers/linear/auth/begin?scope=workspace&workspace_id=ws_alpha",
    });
  });

  it("exchanges MCP auth with a complete redirected URL", async () => {
    mockJsonResponse({ server_name: "linear", scope: "workspace", status: "authenticated" });

    await exchangeSettingsMCPAuth(
      "linear",
      { scope: "workspace" },
      { redirect_url: "http://127.0.0.1:2123/api/mcp/oauth/callback?code=abc123&state=x" }
    );

    await expectFetchRequest({
      body: { redirect_url: "http://127.0.0.1:2123/api/mcp/oauth/callback?code=abc123&state=x" },
      method: "POST",
      path: "/api/settings/mcp-servers/linear/auth/exchange?scope=workspace",
    });
  });

  it("logs out MCP auth for the scoped target", async () => {
    mockJsonResponse({ server_name: "linear", scope: "user", status: "needs_login" });

    await logoutSettingsMCPAuth("linear", { scope: "user" });

    await expectFetchRequest({
      method: "POST",
      path: "/api/settings/mcp-servers/linear/auth/logout?scope=user",
    });
  });

  it("carries the selected profile through begin, exchange, and logout", async () => {
    const filter = { scope: "profile" as const, profile: " marketing " };
    mockJsonResponse({
      authorization_url: "https://auth.linear.app/oauth/authorize?state=x",
      callback_url: "http://127.0.0.1:2123/api/mcp/oauth/callback",
      expires_at: "2026-07-15T00:05:00Z",
      manual_supported: true,
      state: "compozy_mcp_x",
    });
    await beginSettingsMCPAuth("linear", filter, { mode: "automatic" });
    await expectFetchRequest({
      body: { mode: "automatic" },
      method: "POST",
      path: "/api/settings/mcp-servers/linear/auth/begin?scope=profile&profile=marketing",
    });

    vi.mocked(globalThis.fetch).mockClear();
    mockJsonResponse({ server_name: "linear", scope: "profile", status: "authenticated" });
    const exchange = {
      redirect_url: "http://127.0.0.1:2123/api/mcp/oauth/callback?code=abc123&state=x",
    };
    await exchangeSettingsMCPAuth("linear", filter, exchange);
    await expectFetchRequest({
      body: exchange,
      method: "POST",
      path: "/api/settings/mcp-servers/linear/auth/exchange?scope=profile&profile=marketing",
    });

    vi.mocked(globalThis.fetch).mockClear();
    mockJsonResponse({ server_name: "linear", scope: "profile", status: "needs_login" });
    await logoutSettingsMCPAuth("linear", filter);
    await expectFetchRequest({
      method: "POST",
      path: "/api/settings/mcp-servers/linear/auth/logout?scope=profile&profile=marketing",
    });
  });
});
