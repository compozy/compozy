import { HttpResponse, type HttpHandler } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";

import type {
  SettingsMutationResult,
  SettingsNotificationPresetCollection,
} from "@/systems/settings";

import {
  settingsAppliedMutationFixture,
  settingsApplyRecordsFixture,
  settingsAutomationSectionFixture,
  settingsSandboxesCollectionFixture,
  settingsSandboxFixtures,
  settingsGeneralSectionFixture,
  settingsHooksCollectionFixture,
  settingsHooksExtensionsSectionFixture,
  mcpAuthBeginFixture,
  mcpAuthStatusAuthenticatedFixture,
  settingsMCPServersCollectionFixture,
  settingsMemorySectionFixture,
  settingsNetworkSectionFixture,
  settingsNotificationPresetCollectionFixture,
  settingsObservabilitySectionFixture,
  settingsProvidersCollectionFixture,
  settingsProviderFixtures,
  settingsReloadBlockedFixture,
  settingsRestartRequiredMutationFixture,
  settingsRestartResponseFixture,
  settingsRestartStatusFixture,
  settingsSkillsSectionFixture,
} from "./fixtures";
import {
  settingsWindowManagerDesktopIds,
  settingsWindowManagerSectionFixture,
  settingsWindowManagerSnapshotFixture,
  windowManagerLayoutDocumentFixture,
  windowManagerLayoutResourceFixture,
} from "./window-manager-fixtures";
import { rolesStatusFixture, settingsRolesSectionFixture } from "./roles-fixtures";
import { settingsUpdateStatusFixture } from "./settings-update-fixture";

function layoutDocumentForWorkspace(workspaceId: string) {
  return structuredClone({ ...windowManagerLayoutDocumentFixture, workspace_id: workspaceId });
}

function layoutResourceFor(workspaceId: string, id: string) {
  const record = structuredClone(windowManagerLayoutResourceFixture);
  record.id = id;
  record.scope = { kind: "workspace", id: workspaceId };
  record.spec.id = id;
  record.spec.document = layoutDocumentForWorkspace(workspaceId);
  return record;
}

function mutationResult(section: SettingsMutationResult["section"], restartRequired = false) {
  return {
    ...(restartRequired ? settingsRestartRequiredMutationFixture : settingsAppliedMutationFixture),
    section,
  };
}

function applyRecordsForUrl(request: Request) {
  const url = new URL(request.url);
  const status = url.searchParams.get("status");
  const actor = url.searchParams.get("actor");
  const limit = Number.parseInt(url.searchParams.get("limit") ?? "", 10);
  let entries = settingsApplyRecordsFixture.entries;

  if (status) {
    entries = entries.filter(record => record.status === status);
  }

  if (actor) {
    entries = entries.filter(record => record.actor === actor);
  }

  if (Number.isFinite(limit) && limit >= 0) {
    entries = entries.slice(0, limit);
  }

  return { entries };
}

function mcpAuthTarget(request: Request, serverName: string) {
  const url = new URL(request.url);
  const scope = url.searchParams.get("scope") === "workspace" ? "workspace" : "global";
  const workspaceId = url.searchParams.get("workspace_id")?.trim();
  return {
    server_name: serverName,
    scope,
    ...(scope === "workspace" && workspaceId ? { workspace_id: workspaceId } : {}),
  };
}

export const handlers: HttpHandler[] = [
  aghApiMock.get("/api/settings/general", () => HttpResponse.json(settingsGeneralSectionFixture)),
  aghApiMock.patch("/api/settings/general", () =>
    HttpResponse.json(mutationResult("general", true))
  ),
  aghApiMock.get("/api/settings/update", () => HttpResponse.json(settingsUpdateStatusFixture)),

  aghApiMock.get("/api/settings/memory", () => HttpResponse.json(settingsMemorySectionFixture)),
  aghApiMock.patch("/api/settings/memory", () => HttpResponse.json(mutationResult("memory"))),

  aghApiMock.get("/api/roles", () => HttpResponse.json(rolesStatusFixture)),
  aghApiMock.get("/api/settings/roles", () => HttpResponse.json(settingsRolesSectionFixture)),
  aghApiMock.patch("/api/settings/roles", () => HttpResponse.json(mutationResult("roles"))),

  aghApiMock.get("/api/settings/skills", () => HttpResponse.json(settingsSkillsSectionFixture)),
  aghApiMock.patch("/api/settings/skills", () => HttpResponse.json(mutationResult("skills", true))),

  aghApiMock.get("/api/settings/automation", () =>
    HttpResponse.json(settingsAutomationSectionFixture)
  ),
  aghApiMock.patch("/api/settings/automation", () =>
    HttpResponse.json(mutationResult("automation", true))
  ),

  aghApiMock.get("/api/settings/network", () => HttpResponse.json(settingsNetworkSectionFixture)),
  aghApiMock.patch("/api/settings/network", () =>
    HttpResponse.json(mutationResult("network", true))
  ),

  aghApiMock.get("/api/settings/window-manager", () =>
    HttpResponse.json(settingsWindowManagerSectionFixture)
  ),
  aghApiMock.patch("/api/settings/window-manager", () =>
    HttpResponse.json(mutationResult("window-manager"))
  ),

  aghApiMock.get("/api/workspaces/{workspace_id}/window-manager/layout", ({ params }) =>
    HttpResponse.json(layoutDocumentForWorkspace(String(params.workspace_id)))
  ),
  aghApiMock.put("/api/workspaces/{workspace_id}/window-manager/layout", ({ params }) =>
    HttpResponse.json({
      snapshot: {
        ...settingsWindowManagerSnapshotFixture,
        workspace_id: String(params.workspace_id),
        revision: settingsWindowManagerSnapshotFixture.revision + 1,
      },
      applied: true,
      changes: {},
      diagnostics: [],
    })
  ),
  aghApiMock.post("/api/workspaces/{workspace_id}/window-manager/layout/validate", ({ params }) =>
    HttpResponse.json({
      workspace_id: String(params.workspace_id),
      valid: true,
      diagnostics: [],
    })
  ),
  aghApiMock.post("/api/workspaces/{workspace_id}/window-manager/preview", ({ params }) =>
    HttpResponse.json({
      snapshot: {
        ...settingsWindowManagerSnapshotFixture,
        workspace_id: String(params.workspace_id),
      },
      changed: true,
      changes: {
        desktop_ids: [settingsWindowManagerDesktopIds.build],
        group_ids: ["group-main"],
        node_ids: ["split-main"],
        window_ids: [],
      },
      diagnostics: [],
    })
  ),
  aghApiMock.get("/api/workspaces/{workspace_id}/window-manager/layout-profiles", ({ params }) => {
    const workspaceId = String(params.workspace_id);
    return HttpResponse.json({
      records: [layoutResourceFor(workspaceId, windowManagerLayoutResourceFixture.id)],
    });
  }),
  aghApiMock.put(
    "/api/workspaces/{workspace_id}/window-manager/layout-profiles/{profile_id}",
    ({ params }) =>
      HttpResponse.json({
        record: layoutResourceFor(String(params.workspace_id), String(params.profile_id)),
      })
  ),
  aghApiMock.delete(
    "/api/workspaces/{workspace_id}/window-manager/layout-profiles/{profile_id}",
    () => new HttpResponse(null, { status: 204 })
  ),

  aghApiMock.get("/api/settings/observability", () =>
    HttpResponse.json(settingsObservabilitySectionFixture)
  ),
  aghApiMock.patch("/api/settings/observability", () =>
    HttpResponse.json(mutationResult("observability"))
  ),

  aghApiMock.get("/api/settings/hooks-extensions", () =>
    HttpResponse.json(settingsHooksExtensionsSectionFixture)
  ),
  aghApiMock.patch("/api/settings/hooks-extensions", () =>
    HttpResponse.json(mutationResult("hooks-extensions", true))
  ),

  aghApiMock.get("/api/notifications/presets", () =>
    HttpResponse.json(settingsNotificationPresetCollectionFixture)
  ),
  aghApiMock.post("/api/notifications/presets", async ({ request }) => {
    const body = (await request.json()) as {
      name?: string;
      events?: string[];
      targets?: SettingsNotificationPresetCollection["presets"][number]["targets"];
      filter?: string;
      enabled?: boolean;
    };
    return HttpResponse.json(
      {
        preset: {
          name: body.name ?? "custom",
          events: body.events ?? [],
          targets: body.targets ?? [],
          filter: body.filter ?? "",
          enabled: body.enabled ?? false,
          built_in: false,
          default_version: "",
          default_hash: "",
          user_modified: false,
          default_update_available: false,
          created_at: "2026-04-17T11:30:00Z",
          updated_at: "2026-04-17T11:30:00Z",
        },
      },
      { status: 201 }
    );
  }),
  aghApiMock.put("/api/notifications/presets/{name}", async ({ params, request }) => {
    const name = String(params.name);
    const body = (await request.json()) as { enabled?: boolean };
    const existing = settingsNotificationPresetCollectionFixture.presets.find(
      preset => preset.name === name
    );
    return HttpResponse.json({
      preset: {
        ...(existing ?? settingsNotificationPresetCollectionFixture.presets[0]),
        name,
        enabled: body.enabled ?? existing?.enabled ?? true,
        updated_at: "2026-04-17T11:45:00Z",
      },
    });
  }),
  aghApiMock.delete(
    "/api/notifications/presets/{name}",
    () => new HttpResponse(null, { status: 204 })
  ),

  aghApiMock.get("/api/settings/providers", () =>
    HttpResponse.json(settingsProvidersCollectionFixture)
  ),
  aghApiMock.get("/api/settings/providers/{name}", ({ params }) => {
    const name = String(params.name);
    const provider = settingsProviderFixtures.find(entry => entry.name === name);

    if (!provider) {
      return HttpResponse.json({ error: `Provider not found: ${name}` }, { status: 404 });
    }

    return HttpResponse.json({ provider });
  }),
  aghApiMock.put("/api/settings/providers/{name}", () =>
    HttpResponse.json(mutationResult("providers", true))
  ),
  aghApiMock.delete("/api/settings/providers/{name}", () =>
    HttpResponse.json(mutationResult("providers", true))
  ),

  aghApiMock.get("/api/settings/sandboxes", () =>
    HttpResponse.json(settingsSandboxesCollectionFixture)
  ),
  aghApiMock.get("/api/settings/sandboxes/{name}", ({ params }) => {
    const name = String(params.name);
    const sandbox = settingsSandboxFixtures.find(entry => entry.name === name);

    if (!sandbox) {
      return HttpResponse.json({ error: `Sandbox not found: ${name}` }, { status: 404 });
    }

    return HttpResponse.json({ sandbox });
  }),
  aghApiMock.put("/api/settings/sandboxes/{name}", () =>
    HttpResponse.json(mutationResult("sandboxes", true))
  ),
  aghApiMock.delete("/api/settings/sandboxes/{name}", () =>
    HttpResponse.json(mutationResult("sandboxes", true))
  ),

  aghApiMock.get("/api/settings/hooks", () => HttpResponse.json(settingsHooksCollectionFixture)),
  aghApiMock.put("/api/settings/hooks/{name}", () =>
    HttpResponse.json(mutationResult("hooks-extensions", true))
  ),
  aghApiMock.delete("/api/settings/hooks/{name}", () =>
    HttpResponse.json(mutationResult("hooks-extensions", true))
  ),

  aghApiMock.get("/api/settings/mcp-servers", ({ request }) => {
    const url = new URL(request.url);
    const scope = url.searchParams.get("scope");
    const workspaceId = url.searchParams.get("workspace_id");

    if (scope === "workspace" && workspaceId) {
      return HttpResponse.json({
        ...settingsMCPServersCollectionFixture,
        mcp_servers: [],
        scope: "workspace",
        workspace_id: workspaceId,
      });
    }

    return HttpResponse.json(settingsMCPServersCollectionFixture);
  }),
  aghApiMock.put("/api/settings/mcp-servers/{name}", () =>
    HttpResponse.json(mutationResult("mcp-servers", true))
  ),
  aghApiMock.delete("/api/settings/mcp-servers/{name}", () =>
    HttpResponse.json(mutationResult("mcp-servers", true))
  ),
  aghApiMock.post("/api/settings/mcp-servers/{name}/auth/begin", () =>
    HttpResponse.json(mcpAuthBeginFixture)
  ),
  aghApiMock.post("/api/settings/mcp-servers/{name}/auth/exchange", ({ request, params }) =>
    HttpResponse.json({
      ...mcpAuthStatusAuthenticatedFixture,
      ...mcpAuthTarget(request, String(params.name)),
    })
  ),
  aghApiMock.post("/api/settings/mcp-servers/{name}/auth/logout", ({ request, params }) =>
    HttpResponse.json({
      ...mcpAuthStatusAuthenticatedFixture,
      ...mcpAuthTarget(request, String(params.name)),
      status: "needs_login",
      token_present: false,
      diagnostic: "login required",
    })
  ),

  aghApiMock.get("/api/settings/apply", ({ request }) =>
    HttpResponse.json(applyRecordsForUrl(request))
  ),
  aghApiMock.post("/api/settings/reload", () => HttpResponse.json(settingsReloadBlockedFixture)),

  aghApiMock.post("/api/settings/actions/restart", () =>
    HttpResponse.json(settingsRestartResponseFixture, { status: 202 })
  ),
  aghApiMock.get("/api/settings/actions/restart/{operation_id}", () =>
    HttpResponse.json(settingsRestartStatusFixture)
  ),
];
