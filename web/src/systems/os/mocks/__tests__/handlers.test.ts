// Suite: OS Storybook handlers
// Invariant: window-manager commands require a registered client and expose truthful outcomes.
// Owning layer: the MSW boundary used by OS stories.
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";

import { windowManagerStoryWindowId } from "../fixtures";
import { handlers, resetWindowManagerMockState } from "../handlers";

const server = setupServer(...handlers);
const API = "http://localhost/api/workspaces/workspace-custom/window-manager";

beforeAll(() => {
  server.listen({ onUnhandledRequest: "error" });
});

afterEach(() => {
  resetWindowManagerMockState();
  server.resetHandlers();
});

afterAll(() => {
  server.close();
});

function commandBody(clientId: string, commandId = "window.close", payload: unknown = {}) {
  return {
    workspace_id: "workspace-custom",
    client_id: clientId,
    expected_revision: 12,
    command_id: commandId,
    actor: { kind: "web", id: clientId },
    origin: "storybook",
    payload,
  };
}

async function register(clientId: string): Promise<Response> {
  return fetch(`${API}/clients`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ workspace_id: "workspace-custom", client_id: clientId }),
  });
}

async function command(body: object): Promise<Response> {
  return fetch(`${API}/commands`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
}

describe("OS window-manager MSW handlers", () => {
  it("Should reject commands from an unregistered client", async () => {
    const response = await command(commandBody("client:missing"));

    expect(response.status).toBe(404);
    await expect(response.json()).resolves.toMatchObject({
      code: "window_manager_client_not_found",
      workspace_id: "workspace-custom",
    });
  });

  it("Should apply the supported minimize command after registration", async () => {
    const registration = await register("client:storybook");
    expect(registration.status).toBe(201);
    await expect(registration.json()).resolves.toMatchObject({
      client_id: "client:storybook",
      workspace_id: "workspace-custom",
    });

    const response = await command(
      commandBody("client:storybook", "window.close", {
        window_id: windowManagerStoryWindowId,
        minimize: true,
      })
    );

    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toMatchObject({
      applied: true,
      snapshot: {
        workspace_id: "workspace-custom",
        revision: 13,
        windows: { [windowManagerStoryWindowId]: { minimized: true } },
      },
      changes: { window_ids: [windowManagerStoryWindowId] },
      client: { client_id: "client:storybook", workspace_id: "workspace-custom" },
    });
  });

  it("Should diagnose unsupported commands instead of reporting a false success", async () => {
    expect((await register("client:storybook")).status).toBe(201);

    const response = await command(
      commandBody("client:storybook", "window.zoom", {
        window_id: windowManagerStoryWindowId,
      })
    );

    expect(response.status).toBe(422);
    await expect(response.json()).resolves.toMatchObject({
      code: "window_manager_invalid_command",
      workspace_id: "workspace-custom",
      diagnostics: [{ code: "unsupported_mock_command", path: "command_id" }],
    });
  });
});
