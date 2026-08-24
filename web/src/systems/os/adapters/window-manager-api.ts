import { apiBaseUrl, runtimeFetch } from "@/lib/api-client";

import {
  parseWindowManagerCommandResult,
  parseWindowManagerError,
  parseWindowManagerRegisteredClientView,
  parseWindowManagerSnapshot,
} from "../lib/window-manager-schemas";
import type {
  LayoutRevision,
  WindowManagerClientKind,
  WindowManagerCommandInput,
  WindowManagerCommandResult,
  WindowManagerErrorPayload,
  WindowManagerRegisteredClientView,
  WindowManagerSnapshot,
} from "../lib/window-manager-types";

export class WindowManagerApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly payload: WindowManagerErrorPayload | null
  ) {
    super(message);
    this.name = "WindowManagerApiError";
  }
}

function managerPath(workspaceId: string): string {
  const normalized = workspaceId.trim();
  if (normalized === "") {
    throw new WindowManagerApiError("Workspace ID is required.", 400, null);
  }
  return `/api/workspaces/${encodeURIComponent(normalized)}/window-manager`;
}

/**
 * Desks belong to one profile, so every window-manager call names the profile it
 * acts as. Omitting it would resolve `default` at the daemon boundary rather than
 * the profile this client is in, and quietly rearrange the wrong desk.
 */
function managerUrl(
  workspaceId: string,
  profile: string,
  suffix = "",
  params: Record<string, string> = {}
): string {
  const normalizedProfile = profile.trim();
  if (normalizedProfile === "") {
    throw new WindowManagerApiError("Profile is required.", 400, null);
  }
  const query = new URLSearchParams({ ...params, profile: normalizedProfile });
  return `${managerPath(workspaceId)}${suffix}?${query.toString()}`;
}

async function responseJson(response: Response): Promise<unknown> {
  try {
    return await response.json();
  } catch (error) {
    throw new WindowManagerApiError(
      error instanceof Error ? error.message : "CompozyOS returned invalid JSON.",
      response.status,
      null
    );
  }
}

async function requireSuccess(response: Response): Promise<unknown> {
  const body = await responseJson(response);
  if (response.ok) return body;

  const parsed = parseWindowManagerError(body);
  throw new WindowManagerApiError(parsed.error, response.status, parsed);
}

function jsonRequest(body: unknown, signal?: AbortSignal): RequestInit {
  return {
    body: JSON.stringify(body),
    headers: { "content-type": "application/json" },
    signal,
  };
}

export async function fetchWindowManagerSnapshot(
  workspaceId: string,
  profile: string,
  signal?: AbortSignal
): Promise<WindowManagerSnapshot> {
  const response = await runtimeFetch(`${apiBaseUrl}${managerUrl(workspaceId, profile)}`, {
    signal,
  });
  return parseWindowManagerSnapshot(await requireSuccess(response));
}

export async function registerWindowManagerClient(
  workspaceId: string,
  profile: string,
  clientId: string,
  activeDesktopId?: string,
  kind: WindowManagerClientKind = "browser",
  signal?: AbortSignal
): Promise<WindowManagerRegisteredClientView> {
  const response = await runtimeFetch(
    `${apiBaseUrl}${managerUrl(workspaceId, profile, "/clients")}`,
    {
      ...jsonRequest(
        {
          workspace_id: workspaceId,
          client_id: clientId,
          kind,
          ...(activeDesktopId ? { active_desktop_id: activeDesktopId } : {}),
          context: { workspace_trusted: true },
        },
        signal
      ),
      method: "POST",
    }
  );
  return parseWindowManagerRegisteredClientView(await requireSuccess(response));
}

export async function unregisterWindowManagerClient(
  workspaceId: string,
  profile: string,
  clientId: string,
  signal?: AbortSignal
): Promise<void> {
  const response = await runtimeFetch(
    `${apiBaseUrl}${managerUrl(workspaceId, profile, `/clients/${encodeURIComponent(clientId)}`)}`,
    { method: "DELETE", signal }
  );
  if (response.status === 204) return;
  await requireSuccess(response);
}

function commandBody(
  workspaceId: string,
  clientId: string,
  revision: LayoutRevision,
  command: WindowManagerCommandInput
): Record<string, unknown> {
  const rebase = command.rebase
    ? {
        ...(command.rebase.windowId ? { window_id: command.rebase.windowId } : {}),
        ...(command.rebase.sourceNodeId ? { source_node_id: command.rebase.sourceNodeId } : {}),
        ...(command.rebase.targetNodeId ? { target_node_id: command.rebase.targetNodeId } : {}),
        ...(command.rebase.splitId ? { split_id: command.rebase.splitId } : {}),
        ...(command.rebase.boundaryIndex !== undefined
          ? { boundary_index: command.rebase.boundaryIndex }
          : {}),
      }
    : undefined;

  return {
    workspace_id: workspaceId,
    command_id: command.commandId,
    expected_revision: revision,
    client_id: clientId,
    actor: { kind: "web", id: clientId },
    origin: "web",
    ...(rebase ? { rebase } : {}),
    payload: command.payload,
  };
}

export async function executeWindowManagerCommand(
  workspaceId: string,
  profile: string,
  clientId: string,
  revision: LayoutRevision,
  command: WindowManagerCommandInput,
  signal?: AbortSignal
): Promise<WindowManagerCommandResult> {
  const response = await runtimeFetch(
    `${apiBaseUrl}${managerUrl(workspaceId, profile, "/commands")}`,
    {
      ...jsonRequest(commandBody(workspaceId, clientId, revision, command), signal),
      method: "POST",
    }
  );
  return parseWindowManagerCommandResult(await requireSuccess(response));
}

export function buildWindowManagerStreamUrl(
  workspaceId: string,
  profile: string,
  clientId: string,
  afterRevision: LayoutRevision
): string {
  return managerUrl(workspaceId, profile, "/stream", {
    after_revision: String(afterRevision),
    client_id: clientId,
  });
}
