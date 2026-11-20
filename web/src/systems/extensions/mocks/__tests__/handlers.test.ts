import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";

import { DEV_EXTENSION_WORKSPACE_ID, devExtensionFixture, extensionFixtures } from "../fixtures";
import { handlers, resetExtensionMockState, seedBundledExtensionMockState } from "../handlers";

// Invariant: the mock refuses a permanent removal scoped to a workspace or profile with the daemon's 400 before any
// branch mutates state, and keeps a scoped plain removal working. Owner: extensions mock handlers (Storybook/dev).
const server = setupServer(...handlers);
const origin = "http://localhost";

const installedName = extensionFixtures[0]?.name ?? "";

beforeAll(() => server.listen({ onUnhandledFrame: "error" }));
afterEach(() => {
  server.resetHandlers();
  resetExtensionMockState();
});
afterAll(() => server.close());

async function listWorkspaceExtensionNames(workspace: string): Promise<string[]> {
  const response = await fetch(`${origin}/api/extensions?workspace=${workspace}`);
  const body = (await response.json()) as { extensions: { name: string }[] };
  return body.extensions.map(extension => extension.name);
}

async function listEnabledForProfile(name: string, profile: string): Promise<boolean | undefined> {
  const response = await fetch(`${origin}/api/extensions?profile=${profile}`);
  const body = (await response.json()) as { extensions: { name: string; enabled: boolean }[] };
  return body.extensions.find(extension => extension.name === name)?.enabled;
}

describe("extension DELETE mock scope validation", () => {
  it("Should refuse permanent removal with a workspace and leave the dev overlay in place", async () => {
    const overlay = devExtensionFixture.name;
    const response = await fetch(
      `${origin}/api/extensions/${overlay}?workspace=${DEV_EXTENSION_WORKSPACE_ID}&permanent=true`,
      { method: "DELETE" }
    );

    expect(response.status).toBe(400);
    expect(await listWorkspaceExtensionNames(DEV_EXTENSION_WORKSPACE_ID)).toContain(overlay);
  });

  it("Should refuse permanent removal with only a profile and record no tombstone", async () => {
    expect(installedName).not.toBe("");
    const response = await fetch(
      `${origin}/api/extensions/${installedName}?profile=team&permanent=true`,
      {
        method: "DELETE",
      }
    );

    expect(response.status).toBe(400);
    const removed = (await (await fetch(`${origin}/api/extensions/removed-builtins`)).json()) as {
      removed: unknown[];
    };
    expect(removed.removed).toHaveLength(0);
  });

  it("Should keep a scoped plain removal working and remove only the workspace overlay", async () => {
    const overlay = devExtensionFixture.name;
    const response = await fetch(
      `${origin}/api/extensions/${overlay}?workspace=${DEV_EXTENSION_WORKSPACE_ID}`,
      {
        method: "DELETE",
      }
    );

    expect(response.status).toBe(200);
    expect(await listWorkspaceExtensionNames(DEV_EXTENSION_WORKSPACE_ID)).not.toContain(overlay);
  });
});

// Invariant: restore leaves a built-in enabled both in its response and in the next list, because permanent removal
// deletes the row's enablement exceptions, as the extensions_profile_enablement_delete trigger does for the daemon.
// Owner: extensions mock handlers (Storybook/dev).
describe("extension restore mock enablement", () => {
  it("Should restore a built-in disabled before removal as enabled in the response and the next list", async () => {
    const name = "spec-cycle";
    seedBundledExtensionMockState({
      ...structuredClone(extensionFixtures[0]!),
      name,
      source: "bundled",
    });

    const disabled = await fetch(`${origin}/api/extensions/${name}/enablement`, {
      body: JSON.stringify({ enabled: false, profile: "default" }),
      headers: { "Content-Type": "application/json" },
      method: "PUT",
    });
    expect(disabled.status).toBe(200);
    expect(await listEnabledForProfile(name, "default")).toBe(false);

    const removed = await fetch(`${origin}/api/extensions/${name}?permanent=true`, {
      method: "DELETE",
    });
    expect(removed.status).toBe(200);

    const restored = await fetch(`${origin}/api/extensions/${name}/restore`, { method: "POST" });
    expect(restored.status).toBe(200);
    expect(((await restored.json()) as { enabled: boolean }).enabled).toBe(true);
    expect(await listEnabledForProfile(name, "default")).toBe(true);
  });
});
