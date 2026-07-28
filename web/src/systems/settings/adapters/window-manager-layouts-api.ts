import { apiBaseUrl, runtimeFetch } from "@/lib/api-client";
import { fetchWindowManagerSnapshot, type WindowManagerConfig } from "@/systems/os";

import {
  parseWindowManagerLayoutApply,
  parseWindowManagerLayoutDocument,
  parseWindowManagerLayoutPreview,
  parseWindowManagerLayoutResource,
  parseWindowManagerLayoutResources,
  parseWindowManagerLayoutValidation,
  windowManagerLayoutDocumentToWire,
  windowManagerProfileToWire,
  windowManagerSettingsConfigToWire,
} from "../lib/window-manager-layout-schema";
import type {
  WindowManagerLayoutApplyResult,
  WindowManagerLayoutDocument,
  WindowManagerLayoutPreview,
  WindowManagerLayoutProfile,
  WindowManagerLayoutResourceRecord,
  WindowManagerLayoutScopeKind,
  WindowManagerLayoutState,
  WindowManagerLayoutValidation,
} from "../lib/window-manager-layout-types";
import { windowManagerSnapshotToLayoutState } from "../lib/window-manager-layout-projection";

export class WindowManagerLayoutsApiError extends Error {
  constructor(
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = "WindowManagerLayoutsApiError";
  }
}

function workspacePath(workspaceId: string): string {
  const normalized = workspaceId.trim();
  if (normalized === "") {
    throw new WindowManagerLayoutsApiError("Workspace ID is required.", 400);
  }
  return `/api/workspaces/${encodeURIComponent(normalized)}/window-manager`;
}

async function responseJson(response: Response): Promise<unknown> {
  try {
    return await response.json();
  } catch (error) {
    throw new WindowManagerLayoutsApiError(
      error instanceof Error ? error.message : "The daemon returned invalid JSON.",
      response.status
    );
  }
}

function errorMessage(value: unknown, fallback: string): string {
  if (value !== null && typeof value === "object") {
    const candidate = Reflect.get(value, "error");
    if (typeof candidate === "string" && candidate.trim() !== "") return candidate;
  }
  return fallback;
}

async function requireJson(response: Response, fallback: string): Promise<unknown> {
  const body = await responseJson(response);
  if (response.ok) return body;
  throw new WindowManagerLayoutsApiError(
    errorMessage(body, `${fallback} (${response.status})`),
    response.status
  );
}

function jsonRequest(body: unknown, signal?: AbortSignal): RequestInit {
  return {
    body: JSON.stringify(body),
    headers: { "content-type": "application/json" },
    signal,
  };
}

export async function updateWindowManagerSettings(
  config: WindowManagerConfig,
  signal?: AbortSignal
): Promise<void> {
  const response = await runtimeFetch(`${apiBaseUrl}/api/settings/window-manager`, {
    ...jsonRequest({ config: windowManagerSettingsConfigToWire(config) }, signal),
    method: "PATCH",
  });
  await requireJson(response, "Unable to save window-manager settings");
}

export async function exportWindowManagerLayout(
  workspaceId: string,
  signal?: AbortSignal
): Promise<WindowManagerLayoutDocument> {
  const response = await runtimeFetch(`${apiBaseUrl}${workspacePath(workspaceId)}/layout`, {
    signal,
  });
  return parseWindowManagerLayoutDocument(
    await requireJson(response, "Unable to export the current layout")
  );
}

export async function getWindowManagerLayoutState(
  workspaceId: string,
  signal?: AbortSignal
): Promise<WindowManagerLayoutState> {
  return windowManagerSnapshotToLayoutState(await fetchWindowManagerSnapshot(workspaceId, signal));
}

export async function validateWindowManagerLayout(
  workspaceId: string,
  document: WindowManagerLayoutDocument,
  signal?: AbortSignal
): Promise<WindowManagerLayoutValidation> {
  const response = await runtimeFetch(
    `${apiBaseUrl}${workspacePath(workspaceId)}/layout/validate`,
    {
      ...jsonRequest(
        {
          workspace_id: workspaceId,
          document: windowManagerLayoutDocumentToWire({
            ...document,
            workspaceId,
          }),
        },
        signal
      ),
      method: "POST",
    }
  );
  return parseWindowManagerLayoutValidation(
    await requireJson(response, "Unable to validate the layout")
  );
}

export async function previewWindowManagerLayout(
  workspaceId: string,
  revision: number,
  document: WindowManagerLayoutDocument,
  clientId?: string,
  signal?: AbortSignal
): Promise<WindowManagerLayoutPreview> {
  const response = await runtimeFetch(`${apiBaseUrl}${workspacePath(workspaceId)}/preview`, {
    ...jsonRequest(
      {
        workspace_id: workspaceId,
        command_id: "layout.replace",
        expected_revision: revision,
        ...(clientId ? { client_id: clientId } : {}),
        actor: { kind: "web", id: clientId ?? "settings" },
        origin: "web-settings",
        payload: {
          document: windowManagerLayoutDocumentToWire({
            ...document,
            workspaceId,
          }),
        },
      },
      signal
    ),
    method: "POST",
  });
  return parseWindowManagerLayoutPreview(
    await requireJson(response, "Unable to preview the layout")
  );
}

export async function applyWindowManagerLayout(
  workspaceId: string,
  revision: number,
  document: WindowManagerLayoutDocument,
  clientId?: string,
  signal?: AbortSignal
): Promise<WindowManagerLayoutApplyResult> {
  const response = await runtimeFetch(`${apiBaseUrl}${workspacePath(workspaceId)}/layout`, {
    ...jsonRequest(
      {
        workspace_id: workspaceId,
        expected_revision: revision,
        ...(clientId ? { client_id: clientId } : {}),
        actor: { kind: "web", id: clientId ?? "settings" },
        origin: "web-settings",
        document: windowManagerLayoutDocumentToWire({
          ...document,
          workspaceId,
        }),
      },
      signal
    ),
    method: "PUT",
  });
  return parseWindowManagerLayoutApply(await requireJson(response, "Unable to apply the layout"));
}

export async function listWindowManagerLayoutProfiles(
  workspaceId: string,
  signal?: AbortSignal
): Promise<WindowManagerLayoutResourceRecord[]> {
  const response = await runtimeFetch(
    `${apiBaseUrl}${workspacePath(workspaceId)}/layout-profiles`,
    {
      signal,
    }
  );
  return parseWindowManagerLayoutResources(
    await requireJson(response, "Unable to list layout profiles")
  );
}

export async function putWindowManagerLayoutProfile(
  profile: WindowManagerLayoutProfile,
  scope: WindowManagerLayoutScopeKind,
  workspaceId: string,
  expectedVersion: number,
  signal?: AbortSignal
): Promise<WindowManagerLayoutResourceRecord> {
  const scopedDocument = {
    ...profile.document,
    workspaceId: scope === "workspace" ? workspaceId : "",
  };
  const response = await runtimeFetch(
    `${apiBaseUrl}${workspacePath(workspaceId)}/layout-profiles/${encodeURIComponent(profile.id)}`,
    {
      ...jsonRequest(
        {
          scope: {
            kind: scope,
            ...(scope === "workspace" ? { id: workspaceId } : {}),
          },
          expected_version: expectedVersion,
          spec: windowManagerProfileToWire({
            ...profile,
            document: scopedDocument,
          }),
        },
        signal
      ),
      method: "PUT",
    }
  );
  return parseWindowManagerLayoutResource(
    await requireJson(response, "Unable to save the layout profile")
  );
}

export async function deleteWindowManagerLayoutProfile(
  workspaceId: string,
  id: string,
  expectedVersion: number,
  signal?: AbortSignal
): Promise<void> {
  const response = await runtimeFetch(
    `${apiBaseUrl}${workspacePath(workspaceId)}/layout-profiles/${encodeURIComponent(id)}`,
    {
      ...jsonRequest({ expected_version: expectedVersion }, signal),
      method: "DELETE",
    }
  );
  if (response.status === 204) return;
  await requireJson(response, "Unable to delete the layout profile");
}
