import { apiBaseUrl, runtimeFetch } from "@/lib/api-client";

import { parseSettingsWindowManagerConfig } from "../lib/window-manager-config-schema";
import {
  parseWindowManagerClientView,
  parseWindowManagerCommandResult,
  parseWindowManagerError,
  parseWindowManagerSnapshot,
} from "../lib/window-manager-schemas";
import type {
  LayoutRevision,
  WindowManagerClientView,
  WindowManagerCommandInput,
  WindowManagerCommandResult,
  WindowManagerConfig,
  WindowManagerErrorPayload,
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

export async function fetchWindowManagerConfig(signal?: AbortSignal): Promise<WindowManagerConfig> {
  const response = await runtimeFetch(`${apiBaseUrl}/api/settings/window-manager`, { signal });
  const body = await responseJson(response);
  if (!response.ok) {
    throw new WindowManagerApiError(
      "Unable to load window-management settings.",
      response.status,
      null
    );
  }
  return parseSettingsWindowManagerConfig(body);
}

function managerPath(workspaceId: string): string {
  const normalized = workspaceId.trim();
  if (normalized === "") {
    throw new WindowManagerApiError("Workspace ID is required.", 400, null);
  }
  return `/api/workspaces/${encodeURIComponent(normalized)}/window-manager`;
}

async function responseJson(response: Response): Promise<unknown> {
  try {
    return await response.json();
  } catch (error) {
    throw new WindowManagerApiError(
      error instanceof Error ? error.message : "The daemon returned invalid JSON.",
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
  signal?: AbortSignal
): Promise<WindowManagerSnapshot> {
  const response = await runtimeFetch(`${apiBaseUrl}${managerPath(workspaceId)}`, { signal });
  return parseWindowManagerSnapshot(await requireSuccess(response));
}

export async function registerWindowManagerClient(
  workspaceId: string,
  clientId: string,
  activeDesktopId?: string,
  signal?: AbortSignal
): Promise<WindowManagerClientView> {
  const response = await runtimeFetch(`${apiBaseUrl}${managerPath(workspaceId)}/clients`, {
    ...jsonRequest(
      {
        workspace_id: workspaceId,
        client_id: clientId,
        ...(activeDesktopId ? { active_desktop_id: activeDesktopId } : {}),
      },
      signal
    ),
    method: "POST",
  });
  return parseWindowManagerClientView(await requireSuccess(response));
}

export async function unregisterWindowManagerClient(
  workspaceId: string,
  clientId: string,
  signal?: AbortSignal
): Promise<void> {
  const response = await runtimeFetch(
    `${apiBaseUrl}${managerPath(workspaceId)}/clients/${encodeURIComponent(clientId)}`,
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
  clientId: string,
  revision: LayoutRevision,
  command: WindowManagerCommandInput,
  signal?: AbortSignal
): Promise<WindowManagerCommandResult> {
  const response = await runtimeFetch(`${apiBaseUrl}${managerPath(workspaceId)}/commands`, {
    ...jsonRequest(commandBody(workspaceId, clientId, revision, command), signal),
    method: "POST",
  });
  return parseWindowManagerCommandResult(await requireSuccess(response));
}

export function buildWindowManagerStreamUrl(
  workspaceId: string,
  clientId: string,
  afterRevision: LayoutRevision
): string {
  const path = `${managerPath(workspaceId)}/stream`;
  return `${path}?after_revision=${encodeURIComponent(String(afterRevision))}&client_id=${encodeURIComponent(clientId)}`;
}
