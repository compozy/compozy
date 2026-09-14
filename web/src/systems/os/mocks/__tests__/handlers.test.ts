// Suite: OS Storybook handlers
// Invariant: window-manager commands require a registered client and expose truthful outcomes.
// Boundary IN: the MSW boundary used by OS and full-app route stories.
// Boundary OUT: browser wiring and rendered story behavior (web/e2e/__tests__/storybook-bootstrap.spec.ts).
import { setupServer } from "msw/node";
import { afterAll, beforeAll, beforeEach, describe, expect, it } from "vitest";

import { windowManagerStoryDesktopId, windowManagerStoryWindowId } from "../fixtures";
import { handlers, resetWindowManagerMockState, windowManagerStreamHandler } from "../handlers";
import { parseWindowManagerStreamFrame } from "../../lib/window-manager-stream-schema";
import { parseWindowManagerRegisteredClientView } from "../../lib/window-manager-schemas";

const server = setupServer(windowManagerStreamHandler, ...handlers);
const API = "http://localhost/api/workspaces/workspace-custom/window-manager";

beforeAll(() => {
  server.listen({ onUnhandledRequest: "error" });
});

beforeEach(() => {
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

async function snapshot(): Promise<Response> {
  return fetch(API);
}

async function command(body: object): Promise<Response> {
  return fetch(`${API}/commands`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
}

describe("OS window-manager MSW handlers", () => {
  it("Should stream the registered workspace snapshot and reject unknown clients", async () => {
    resetWindowManagerMockState("/marketplace");
    const registration = await register("client:stream");
    expect(parseWindowManagerRegisteredClientView(await registration.json())).toMatchObject({
      clientId: "client:stream",
      workspaceId: "workspace-custom",
      attachmentToken: expect.any(String),
    });
    for (const clientId of ["client:stream", "client:missing"]) {
      const socket = new WebSocket(`${API.replace("http:", "ws:")}/stream?client_id=${clientId}`);
      try {
        const frame = await new Promise<ReturnType<typeof parseWindowManagerStreamFrame>>(
          (resolve, reject) => {
            socket.onmessage = event => {
              try {
                resolve(parseWindowManagerStreamFrame(JSON.parse(String(event.data))));
              } catch (error) {
                reject(error);
              }
            };
            socket.onerror = () => reject(new Error("Window-manager stream failed"));
            socket.onclose = () =>
              reject(new Error(`Window-manager stream closed before a frame for ${clientId}`));
          }
        );
        if (clientId === "client:missing") {
          expect(frame).toMatchObject({
            type: "error",
            error: { code: "window_manager_client_not_found" },
          });
        } else {
          expect(frame).toMatchObject({
            type: "snapshot",
            workspaceId: "workspace-custom",
            client: { clientId },
            snapshot: {
              workspaceId: "workspace-custom",
              windows: {
                "w-story-marketplace": { route: { pathname: "/marketplace" } },
              },
            },
          });
        }
      } finally {
        socket.close();
      }
    }
  });

  it("Should keep an app-route snapshot consistent across window-manager operations", async () => {
    resetWindowManagerMockState("/tasks?view=cards");

    const response = await snapshot();

    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toMatchObject({
      workspace_id: "workspace-custom",
      desktops: [
        {
          groups: [{ root: { kind: "leaf", window_id: "w-story-tasks" } }],
        },
      ],
      windows: {
        "w-story-tasks": {
          id: "w-story-tasks",
          app: "tasks",
          route: { pathname: "/tasks", search: { view: "cards" } },
        },
      },
    });

    const registration = await register("client:route-story");
    expect(registration.status).toBe(201);
    await expect(registration.json()).resolves.toMatchObject({
      focused_window_id: "w-story-tasks",
    });

    const commandResponse = await command(
      commandBody("client:route-story", "window.close", {
        window_id: "w-story-tasks",
        minimize: true,
      })
    );
    expect(commandResponse.status).toBe(200);
    await expect(commandResponse.json()).resolves.toMatchObject({
      snapshot: {
        windows: {
          "w-story-tasks": {
            minimized: true,
            route: { pathname: "/tasks", search: { view: "cards" } },
          },
        },
      },
      client: {
        focused_window_id: null,
      },
    });
  });

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

  it("Should open and focus the app window required by a route-story deep link", async () => {
    expect((await register("client:storybook")).status).toBe(201);

    const response = await command(
      commandBody("client:storybook", "window.open", {
        window: {
          id: "app:loops",
          app: "loops",
          route: { pathname: "/loop-runs/looprun_running", search: {} },
          desktop_id: windowManagerStoryDesktopId,
          floating_rect: { x: 0.12, y: 0.08, width: 0.68, height: 0.78 },
          insert_tiled: false,
        },
      })
    );

    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toMatchObject({
      applied: true,
      snapshot: {
        workspace_id: "workspace-custom",
        revision: 13,
        windows: {
          "app:loops": {
            app: "loops",
            route: { pathname: "/loop-runs/looprun_running", search: {} },
            desktop_id: windowManagerStoryDesktopId,
            placement: "floating",
          },
        },
      },
      changes: {
        desktop_ids: [windowManagerStoryDesktopId],
        window_ids: ["app:loops"],
      },
      client: {
        client_id: "client:storybook",
        workspace_id: "workspace-custom",
        active_desktop_id: windowManagerStoryDesktopId,
        focused_window_id: "app:loops",
      },
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
