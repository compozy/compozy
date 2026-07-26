import { HttpResponse, type HttpHandler } from "msw";

import { aghApiMock } from "@/storybook/openapi-msw";

import { windowManagerClientFixture, windowManagerSnapshotFixture } from "./fixtures";

const registeredClients = new Set<string>();

export function resetWindowManagerMockState(): void {
  registeredClients.clear();
}

function requestClientId(value: unknown): string | null {
  if (value === null || typeof value !== "object") return null;
  const clientId = Reflect.get(value, "client_id");
  return typeof clientId === "string" && clientId.trim() !== "" ? clientId.trim() : null;
}

function commandWindowId(value: unknown): string | null {
  if (value === null || typeof value !== "object") return null;
  const payload = Reflect.get(value, "payload");
  if (payload === null || typeof payload !== "object") return null;
  const windowId = Reflect.get(payload, "window_id");
  return typeof windowId === "string" && windowId.trim() !== "" ? windowId : null;
}

function commandMinimizes(value: unknown): boolean {
  if (value === null || typeof value !== "object") return false;
  const payload = Reflect.get(value, "payload");
  return (
    payload !== null && typeof payload === "object" && Reflect.get(payload, "minimize") === true
  );
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
  aghApiMock.get("/api/workspaces/{workspace_id}/window-manager", () =>
    HttpResponse.json(windowManagerSnapshotFixture)
  ),
  aghApiMock.post(
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
      registeredClients.add(clientId);
      return HttpResponse.json(windowManagerClientFixture(clientId, workspaceId), {
        status: 201,
      });
    }
  ),
  aghApiMock.post(
    "/api/workspaces/{workspace_id}/window-manager/commands",
    async ({ params, request }) => {
      const workspaceId = String(params.workspace_id);
      const body = await request.json();
      const clientId = requestClientId(body);
      if (clientId === null || !registeredClients.has(clientId)) {
        return HttpResponse.json(
          windowManagerError(
            workspaceId,
            "window_manager_client_not_found",
            "Register the window-manager client before issuing commands."
          ),
          { status: 404 }
        );
      }
      const commandId =
        body !== null && typeof body === "object" ? Reflect.get(body, "command_id") : null;
      const windowId = commandWindowId(body);
      if (commandId !== "window.close" || !commandMinimizes(body) || windowId === null) {
        return HttpResponse.json(
          windowManagerError(
            workspaceId,
            "window_manager_invalid_command",
            `Storybook does not simulate ${String(commandId)}.`,
            [
              {
                code: "unsupported_mock_command",
                path: "command_id",
                message: `Storybook does not simulate ${String(commandId)}.`,
              },
            ]
          ),
          { status: 422 }
        );
      }
      const snapshot = structuredClone(windowManagerSnapshotFixture);
      snapshot.workspace_id = workspaceId;
      const window = snapshot.windows[windowId];
      if (!window) {
        return HttpResponse.json(
          windowManagerError(
            workspaceId,
            "window_manager_window_not_found",
            `Window not found: ${windowId}`
          ),
          { status: 404 }
        );
      }
      window.minimized = true;
      snapshot.revision += 1;
      return HttpResponse.json({
        snapshot,
        applied: true,
        changes: { window_ids: [windowId] },
        diagnostics: [],
        client: windowManagerClientFixture(clientId, workspaceId),
      });
    }
  ),
];
