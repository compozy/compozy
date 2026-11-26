import { describe, expect, it } from "vitest";

import { settingsKeys } from "../query-keys";

describe("settingsKeys", () => {
  it("builds stable section keys", () => {
    expect(settingsKeys.section("general")).toEqual([
      "computer",
      "local",
      "settings",
      "section",
      "general",
    ]);
    expect(settingsKeys.section("hooks-extensions")).toEqual([
      "computer",
      "local",
      "settings",
      "section",
      "hooks-extensions",
    ]);
  });

  it("isolates persona, attention, and palette keys by profile", () => {
    expect(settingsKeys.personaSection({ scope: "profile", profile: "marketing" })).toEqual([
      "computer",
      "local",
      "settings",
      "section",
      "persona",
      "profile",
      "",
      "marketing",
    ]);
    expect(settingsKeys.cmdPaletteSection({ scope: "profile", profile: "marketing" })).toEqual([
      "computer",
      "local",
      "settings",
      "section",
      "cmd-palette",
      "profile",
      "",
      "marketing",
    ]);
    expect(settingsKeys.attentionSection({ scope: "profile", profile: "marketing" })).toEqual([
      "computer",
      "local",
      "settings",
      "section",
      "attention",
      "profile",
      "marketing",
    ]);
  });

  it("Should isolate window-manager layout reviews by profile", () => {
    const marketing = settingsKeys.windowManagerLayoutReview("workspace-a", "marketing", 7, "same");
    const research = settingsKeys.windowManagerLayoutReview("workspace-a", "research", 7, "same");

    expect(marketing).not.toEqual(research);
    expect(marketing).toContain("marketing");
    expect(research).toContain("research");
  });

  it("isolates provider collection keys from section keys", () => {
    const detail = settingsKeys.providerDetail("openai");
    expect(detail).toEqual([
      "computer",
      "local",
      "settings",
      "collection",
      "providers",
      "detail",
      "openai",
    ]);
    expect(settingsKeys.providersList()).toEqual([
      "computer",
      "local",
      "settings",
      "collection",
      "providers",
      "list",
    ]);
  });

  it("isolates layered hooks collection keys", () => {
    expect(settingsKeys.hooksList()).toEqual([
      "computer",
      "local",
      "settings",
      "collection",
      "hooks",
      "list",
      "",
      "",
      "",
    ]);
    expect(settingsKeys.hooksList({ scope: "profile", profile: "marketing" })).toEqual([
      "computer",
      "local",
      "settings",
      "collection",
      "hooks",
      "list",
      "profile",
      "",
      "marketing",
    ]);
  });

  it("scopes MCP list keys by scope, workspace, and profile identity", () => {
    expect(settingsKeys.mcpList()).toEqual([
      "computer",
      "local",
      "settings",
      "collection",
      "mcp-servers",
      "list",
      "",
      "",
      "",
    ]);

    expect(settingsKeys.mcpList({ scope: "user" })).toEqual([
      "computer",
      "local",
      "settings",
      "collection",
      "mcp-servers",
      "list",
      "user",
      "",
      "",
    ]);

    expect(settingsKeys.mcpList({ scope: "workspace", workspace_id: "ws_alpha" })).toEqual([
      "computer",
      "local",
      "settings",
      "collection",
      "mcp-servers",
      "list",
      "workspace",
      "ws_alpha",
      "",
    ]);

    expect(settingsKeys.mcpList({ scope: "profile", profile: "marketing" })).toEqual([
      "computer",
      "local",
      "settings",
      "collection",
      "mcp-servers",
      "list",
      "profile",
      "",
      "marketing",
    ]);
  });

  it("builds restart keys that include the operation id", () => {
    expect(settingsKeys.restartRoot()).toEqual(["computer", "local", "settings", "restart"]);
    expect(settingsKeys.restartStatus("op_001")).toEqual([
      "computer",
      "local",
      "settings",
      "restart",
      "op_001",
    ]);
  });

  it("builds apply record keys from normalized filters", () => {
    expect(settingsKeys.applyRoot()).toEqual(["computer", "local", "settings", "apply"]);
    expect(settingsKeys.applyRecords({ status: "blocked", actor: " http ", limit: 8 })).toEqual([
      "computer",
      "local",
      "settings",
      "apply",
      "records",
      "blocked",
      "http",
      8,
    ]);
  });
});
