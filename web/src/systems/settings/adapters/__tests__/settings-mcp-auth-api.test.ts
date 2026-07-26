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
  it("begins MCP auth with the scoped target", async () => {
    mockJsonResponse({
      authorization_url: "https://auth.linear.app/oauth/authorize?state=x",
      callback_url: "http://127.0.0.1:2123/api/mcp/oauth/callback",
      expires_at: "2026-07-15T00:05:00Z",
      manual_supported: true,
      state: "agh_mcp_x",
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

  it("exchanges MCP auth with a pasted code", async () => {
    mockJsonResponse({ server_name: "linear", scope: "workspace", status: "authenticated" });

    await exchangeSettingsMCPAuth("linear", { scope: "workspace" }, { code: "abc123" });

    await expectFetchRequest({
      body: { code: "abc123" },
      method: "POST",
      path: "/api/settings/mcp-servers/linear/auth/exchange?scope=workspace",
    });
  });

  it("exchanges MCP auth with a full redirect URL", async () => {
    mockJsonResponse({ server_name: "linear", scope: "workspace", status: "authenticated" });

    await exchangeSettingsMCPAuth(
      "linear",
      { scope: "global" },
      { redirect_url: "http://127.0.0.1:2123/api/mcp/oauth/callback?code=abc&state=x" }
    );

    await expectFetchRequest({
      body: { redirect_url: "http://127.0.0.1:2123/api/mcp/oauth/callback?code=abc&state=x" },
      method: "POST",
      path: "/api/settings/mcp-servers/linear/auth/exchange?scope=global",
    });
  });

  it("logs out MCP auth for the scoped target", async () => {
    mockJsonResponse({ server_name: "linear", scope: "global", status: "needs_login" });

    await logoutSettingsMCPAuth("linear", { scope: "global" });

    await expectFetchRequest({
      method: "POST",
      path: "/api/settings/mcp-servers/linear/auth/logout?scope=global",
    });
  });
});
