import { describe, expect, it } from "vitest";

import { settingsKeys } from "../query-keys";

describe("settingsKeys", () => {
  it("builds stable section keys", () => {
    expect(settingsKeys.section("general")).toEqual(["settings", "section", "general"]);
    expect(settingsKeys.section("hooks-extensions")).toEqual([
      "settings",
      "section",
      "hooks-extensions",
    ]);
  });

  it("isolates provider collection keys from section keys", () => {
    const detail = settingsKeys.providerDetail("openai");
    expect(detail).toEqual(["settings", "collection", "providers", "detail", "openai"]);
    expect(settingsKeys.providersList()).toEqual(["settings", "collection", "providers", "list"]);
  });

  it("isolates sandboxes and hooks collection keys", () => {
    expect(settingsKeys.sandboxesList()).toEqual(["settings", "collection", "sandboxes", "list"]);
    expect(settingsKeys.sandboxDetail("prod")).toEqual([
      "settings",
      "collection",
      "sandboxes",
      "detail",
      "prod",
    ]);
    expect(settingsKeys.hooksList()).toEqual(["settings", "collection", "hooks", "list"]);
  });

  it("scopes MCP list keys by scope and workspace identifier", () => {
    expect(settingsKeys.mcpList()).toEqual([
      "settings",
      "collection",
      "mcp-servers",
      "list",
      "",
      "",
    ]);

    expect(settingsKeys.mcpList({ scope: "global" })).toEqual([
      "settings",
      "collection",
      "mcp-servers",
      "list",
      "global",
      "",
    ]);

    expect(settingsKeys.mcpList({ scope: "workspace", workspace_id: "ws_alpha" })).toEqual([
      "settings",
      "collection",
      "mcp-servers",
      "list",
      "workspace",
      "ws_alpha",
    ]);
  });

  it("builds restart keys that include the operation id", () => {
    expect(settingsKeys.restartRoot()).toEqual(["settings", "restart"]);
    expect(settingsKeys.restartStatus("op_001")).toEqual(["settings", "restart", "op_001"]);
  });

  it("builds apply record keys from normalized filters", () => {
    expect(settingsKeys.applyRoot()).toEqual(["settings", "apply"]);
    expect(settingsKeys.applyRecords({ status: "blocked", actor: " http ", limit: 8 })).toEqual([
      "settings",
      "apply",
      "records",
      "blocked",
      "http",
      8,
    ]);
  });
});
