import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";

import { DEV_EXTENSION_WORKSPACE_ID, devExtensionFixture, extensionFixtures } from "../fixtures";
import { handlers, resetExtensionMockState } from "../handlers";

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
