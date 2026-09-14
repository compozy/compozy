import { HttpResponse, ws, type HttpHandler } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";

import { cmdPaletteCatalogFixture, cmdPaletteClientsFixture } from "./cmd-palette-fixtures";
import {
  windowManagerSnapshotFixture,
  windowManagerStorySnapshot,
  windowManagerStoryWindowId,
} from "./fixtures";
import { StorybookWindowManagerMockRuntime } from "./window-manager-mock-runtime";

const defaultStoryEntry =
  windowManagerSnapshotFixture.windows[windowManagerStoryWindowId]?.route.pathname ?? "/";
let initialStoryEntry = defaultStoryEntry;
const runtime = new StorybookWindowManagerMockRuntime(workspaceId =>
  windowManagerStorySnapshot(initialStoryEntry, workspaceId)
);
const layoutStream = ws.link("*/api/workspaces/:workspaceId/window-manager/stream");

export const windowManagerStreamHandler = layoutStream.addEventListener(
  "connection",
  ({ client, params }) => {
    const workspaceId = String(params.workspaceId);
    const clientId = client.url.searchParams.get("client_id") ?? "";
    const registered = runtime.client(workspaceId, clientId);
    if (registered === null) {
      client.send(
        JSON.stringify({
          type: "error",
          error: windowManagerError(
            workspaceId,
            "window_manager_client_not_found",
            "Register the window-manager client before connecting."
          ),
        })
      );
      return;
    }
    const snapshot = runtime.snapshot(workspaceId);
    client.send(
      JSON.stringify({
        type: "snapshot",
        workspace_id: workspaceId,
        revision: snapshot.revision,
        snapshot,
        client: registered,
      })
    );
    const heartbeat = setInterval(() => {
      client.send(
        JSON.stringify({
          type: "heartbeat",
          workspace_id: workspaceId,
          revision: runtime.snapshot(workspaceId).revision,
        })
      );
    }, 30_000);
    client.addEventListener("close", () => clearInterval(heartbeat));
  }
);

export function resetWindowManagerMockState(initialEntry?: string): void {
  for (const client of layoutStream.clients) client.close();
  initialStoryEntry = initialEntry ?? defaultStoryEntry;
  runtime.reset();
}

function requestClientId(value: unknown): string | null {
  if (value === null || typeof value !== "object") return null;
  const clientId = Reflect.get(value, "client_id");
  return typeof clientId === "string" && clientId.trim() !== "" ? clientId.trim() : null;
}

function windowManagerError(
  workspaceId: string,
  code:
    | "window_manager_client_not_found"
    | "window_manager_invalid_command"
    | "window_manager_window_not_found",
  error: string,
  diagnostics: { code: string; message: string; path?: string }[] = []
) {
  return { code, diagnostics, error, workspace_id: workspaceId };
}

export const handlers: HttpHandler[] = [
  compozyApiMock.get("/api/workspaces/{workspace_id}/window-manager", ({ params }) =>
    HttpResponse.json(runtime.snapshot(String(params.workspace_id)))
  ),
  compozyApiMock.post(
    "/api/workspaces/{workspace_id}/window-manager/clients",
    async ({ params, request }) => {
      const workspaceId = String(params.workspace_id);
      const clientId = requestClientId(await request.json());
      if (clientId === null) {
        return HttpResponse.json(
          windowManagerError(
            workspaceId,
            "window_manager_client_not_found",
            "client_id is required"
          ),
          { status: 422 }
        );
      }
      return HttpResponse.json(
        {
          ...runtime.register(workspaceId, clientId),
          attachment_token: `storybook:${workspaceId}:${clientId}`,
        },
        {
          status: 201,
        }
      );
    }
  ),
  compozyApiMock.post(
    "/api/workspaces/{workspace_id}/window-manager/commands",
    async ({ params, request }) => {
      const workspaceId = String(params.workspace_id);
      const body = await request.json();
      const clientId = requestClientId(body);
      if (clientId === null) {
        return HttpResponse.json(
          windowManagerError(
            workspaceId,
            "window_manager_client_not_found",
            "Register the window-manager client before issuing commands."
          ),
          { status: 404 }
        );
      }

      const outcome = runtime.execute(workspaceId, clientId, body);
      if (outcome.kind === "client-not-found") {
        return HttpResponse.json(
          windowManagerError(
            workspaceId,
            "window_manager_client_not_found",
            "Register the window-manager client before issuing commands."
          ),
          { status: 404 }
        );
      }
      if (outcome.kind === "invalid-command") {
        return HttpResponse.json(
          windowManagerError(workspaceId, "window_manager_invalid_command", outcome.message, [
            {
              code: "unsupported_mock_command",
              path: "command_id",
              message: outcome.message,
            },
          ]),
          { status: 422 }
        );
      }
      if (outcome.kind === "window-not-found") {
        return HttpResponse.json(
          windowManagerError(
            workspaceId,
            "window_manager_window_not_found",
            `Window not found: ${outcome.windowId}`
          ),
          { status: 404 }
        );
      }

      return HttpResponse.json({
        snapshot: outcome.snapshot,
        applied: true,
        changes: outcome.changes,
        diagnostics: [],
        client: outcome.client,
      });
    }
  ),
  compozyApiMock.get("/api/cmd-palette/commands", () =>
    HttpResponse.json(cmdPaletteCatalogFixture)
  ),
  compozyApiMock.get("/api/cmd-palette/clients", () => HttpResponse.json(cmdPaletteClientsFixture)),
  compozyApiMock.post("/api/cmd-palette/commands/{id}/invoke", () =>
    HttpResponse.json({
      status: "ok",
      invocation_id: "inv-mock",
      profile_lens: { profile_lens_id: "00000000000000000000000000", profile_name: "default" },
    })
  ),
];
