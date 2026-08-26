import { HttpResponse, type HttpHandler } from "msw";
import { compozyApiMock } from "@/storybook/openapi-msw";

import type {
  SettingsMutationResult,
  SettingsNotificationPresetCollection,
  SettingsUpdateApplyRequest,
} from "@/systems/settings";

import {
  settingsAppliedMutationFixture,
  settingsApplyRecordsFixture,
  settingsAutomationSectionFixture,
  settingsAttentionSectionFixture,
  settingsCmdPaletteSectionFixture,
  settingsSandboxesCollectionFixture,
  settingsSandboxFixtures,
  settingsGeneralSectionFixture,
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
  settingsHooksCollectionFixtureFor,
  settingsPersonaSectionFixtureFor,
} from "./layered-fixtures";
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
  const scope = url.searchParams.get("scope") === "workspace" ? "workspace" : "user";
  const workspaceId = url.searchParams.get("workspace_id")?.trim();
  return {
    server_name: serverName,
    scope,
    ...(scope === "workspace" && workspaceId ? { workspace_id: workspaceId } : {}),
  };
}

function settingsHooksTarget(request: Request) {
  const url = new URL(request.url);
  const scope = url.searchParams.get("scope");
  const profile = url.searchParams.get("profile")?.trim();
  const workspaceId = url.searchParams.get("workspace_id")?.trim();

  if (scope === "profile" && profile) {
    return settingsHooksCollectionFixtureFor({
      scope: "profile",
      profile,
      ...(workspaceId ? { workspace_id: workspaceId } : {}),
    });
  }
  if (scope === "workspace" && workspaceId) {
    return settingsHooksCollectionFixtureFor({ scope: "workspace", workspace_id: workspaceId });
  }
  return settingsHooksCollectionFixtureFor({ scope: "user" });
}

function settingsPersonaTarget(request: Request) {
  const url = new URL(request.url);
  const scope = url.searchParams.get("scope");
  const profile = url.searchParams.get("profile")?.trim();
  const workspaceId = url.searchParams.get("workspace_id")?.trim();

  if (scope === "profile" && profile) {
    return settingsPersonaSectionFixtureFor({
      scope: "profile",
      profile,
      ...(workspaceId ? { workspace_id: workspaceId } : {}),
    });
  }
  if (scope === "workspace" && workspaceId) {
    return settingsPersonaSectionFixtureFor({ scope: "workspace", workspace_id: workspaceId });
  }
  return settingsPersonaSectionFixtureFor({ scope: "user" });
}

export const handlers: HttpHandler[] = [
  compozyApiMock.get("/api/settings/general", () =>
    HttpResponse.json(settingsGeneralSectionFixture)
  ),
  compozyApiMock.patch("/api/settings/general", () =>
    HttpResponse.json(mutationResult("general", true))
  ),
  compozyApiMock.get("/api/settings/persona", ({ request }) =>
    HttpResponse.json(settingsPersonaTarget(request))
  ),
  compozyApiMock.get("/api/settings/update", () => HttpResponse.json(settingsUpdateStatusFixture)),
  // Apply acknowledges acquisition only; the GET above stays the terminal truth.
  compozyApiMock.post("/api/settings/update/apply", async ({ request }) => {
    const body = (await request.json()) as SettingsUpdateApplyRequest;
    return HttpResponse.json({
      targets: body.targets,
      status: "accepted",
      operation_id: "op-storybook",
      message: "Started the requested updates.",
      holder: null,
    });
  }),
  compozyApiMock.post("/api/settings/update/cancel", () =>
    HttpResponse.json({
      status: "canceled",
      operation_id: "op-storybook",
      message: "Canceled dormant update operation; the update channel is free.",
      holder: null,
    })
  ),

  compozyApiMock.get("/api/settings/memory", () => HttpResponse.json(settingsMemorySectionFixture)),
  compozyApiMock.patch("/api/settings/memory", () => HttpResponse.json(mutationResult("memory"))),

  compozyApiMock.get("/api/roles", () => HttpResponse.json(rolesStatusFixture)),
  compozyApiMock.get("/api/settings/roles", () => HttpResponse.json(settingsRolesSectionFixture)),
  compozyApiMock.patch("/api/settings/roles", () => HttpResponse.json(mutationResult("roles"))),

  compozyApiMock.get("/api/settings/skills", () => HttpResponse.json(settingsSkillsSectionFixture)),
  compozyApiMock.patch("/api/settings/skills", () =>
    HttpResponse.json({
      ...settingsSkillsSectionFixture,
      ...settingsRestartRequiredMutationFixture,
      section: "skills",
    })
  ),

  compozyApiMock.get("/api/settings/automation", () =>
    HttpResponse.json(settingsAutomationSectionFixture)
  ),
  compozyApiMock.patch("/api/settings/automation", () =>
    HttpResponse.json(mutationResult("automation", true))
  ),

  compozyApiMock.get("/api/settings/network", () =>
    HttpResponse.json(settingsNetworkSectionFixture)
  ),
  compozyApiMock.patch("/api/settings/network", () =>
    HttpResponse.json(mutationResult("network", true))
  ),

  compozyApiMock.get("/api/settings/attention", () =>
    HttpResponse.json(settingsAttentionSectionFixture)
  ),
  compozyApiMock.patch("/api/settings/attention", () =>
    HttpResponse.json(mutationResult("attention"))
  ),

  // Both scopes answer from one fixture: which scope was asked for is the
  // caller's contract, and the stories exercise the switch, not the daemon's
  // per-scope storage.
  compozyApiMock.get("/api/settings/cmd-palette", () =>
    HttpResponse.json(settingsCmdPaletteSectionFixture)
  ),
  compozyApiMock.patch("/api/settings/cmd-palette", () =>
    HttpResponse.json(settingsCmdPaletteSectionFixture)
  ),

  compozyApiMock.get("/api/settings/window-manager", () =>
    HttpResponse.json(settingsWindowManagerSectionFixture)
  ),
  // Bindings echo the section the daemon produced, never a write receipt: the
  // caller has to see the keymap that actually resulted (ADR-006).
  compozyApiMock.patch("/api/settings/window-manager", () =>
    HttpResponse.json(settingsWindowManagerSectionFixture)
  ),

  compozyApiMock.get("/api/workspaces/{workspace_id}/window-manager/layout", ({ params }) =>
    HttpResponse.json(layoutDocumentForWorkspace(String(params.workspace_id)))
  ),
  compozyApiMock.put("/api/workspaces/{workspace_id}/window-manager/layout", ({ params }) =>
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
  compozyApiMock.post(
    "/api/workspaces/{workspace_id}/window-manager/layout/validate",
    ({ params }) =>
      HttpResponse.json({
        workspace_id: String(params.workspace_id),
        valid: true,
        diagnostics: [],
      })
  ),
  compozyApiMock.post("/api/workspaces/{workspace_id}/window-manager/preview", ({ params }) =>
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
  compozyApiMock.get(
    "/api/workspaces/{workspace_id}/window-manager/layout-profiles",
    ({ params }) => {
      const workspaceId = String(params.workspace_id);
      return HttpResponse.json({
        records: [layoutResourceFor(workspaceId, windowManagerLayoutResourceFixture.id)],
      });
    }
  ),
  compozyApiMock.put(
    "/api/workspaces/{workspace_id}/window-manager/layout-profiles/{profile_id}",
    ({ params }) =>
      HttpResponse.json({
        record: layoutResourceFor(String(params.workspace_id), String(params.profile_id)),
      })
  ),
  compozyApiMock.delete(
    "/api/workspaces/{workspace_id}/window-manager/layout-profiles/{profile_id}",
    () => new HttpResponse(null, { status: 204 })
  ),

  compozyApiMock.get("/api/settings/observability", () =>
    HttpResponse.json(settingsObservabilitySectionFixture)
  ),
  compozyApiMock.patch("/api/settings/observability", () =>
    HttpResponse.json(mutationResult("observability"))
  ),

  compozyApiMock.get("/api/settings/hooks-extensions", () =>
    HttpResponse.json(settingsHooksExtensionsSectionFixture)
  ),
  compozyApiMock.patch("/api/settings/hooks-extensions", () =>
    HttpResponse.json(mutationResult("hooks-extensions", true))
  ),

  compozyApiMock.get("/api/notifications/presets", () =>
    HttpResponse.json(settingsNotificationPresetCollectionFixture)
  ),
  compozyApiMock.post("/api/notifications/presets", async ({ request }) => {
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
          profile: "default",
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
  compozyApiMock.put("/api/notifications/presets/{name}", async ({ params, request }) => {
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
  compozyApiMock.delete(
    "/api/notifications/presets/{name}",
    () => new HttpResponse(null, { status: 204 })
  ),

  compozyApiMock.get("/api/settings/providers", () =>
    HttpResponse.json(settingsProvidersCollectionFixture)
  ),
  compozyApiMock.get("/api/settings/providers/{name}", ({ params }) => {
    const name = String(params.name);
    const provider = settingsProviderFixtures.find(entry => entry.name === name);

    if (!provider) {
      return HttpResponse.json({ error: `Provider not found: ${name}` }, { status: 404 });
    }

    return HttpResponse.json({ provider });
  }),
  compozyApiMock.put("/api/settings/providers/{name}", () =>
    HttpResponse.json(mutationResult("providers", true))
  ),
  compozyApiMock.delete("/api/settings/providers/{name}", () =>
    HttpResponse.json(mutationResult("providers", true))
  ),

  compozyApiMock.get("/api/settings/sandboxes", () =>
    HttpResponse.json(settingsSandboxesCollectionFixture)
  ),
  compozyApiMock.get("/api/settings/sandboxes/{name}", ({ params }) => {
    const name = String(params.name);
    const sandbox = settingsSandboxFixtures.find(entry => entry.name === name);

    if (!sandbox) {
      return HttpResponse.json({ error: `Sandbox not found: ${name}` }, { status: 404 });
    }

    return HttpResponse.json({ sandbox });
  }),
  compozyApiMock.put("/api/settings/sandboxes/{name}", () =>
    HttpResponse.json(mutationResult("sandboxes", true))
  ),
  compozyApiMock.delete("/api/settings/sandboxes/{name}", () =>
    HttpResponse.json(mutationResult("sandboxes", true))
  ),

  compozyApiMock.get("/api/settings/hooks", ({ request }) =>
    HttpResponse.json(settingsHooksTarget(request))
  ),
  compozyApiMock.put("/api/settings/hooks/{name}", () =>
    HttpResponse.json(mutationResult("hooks-extensions", true))
  ),
  compozyApiMock.delete("/api/settings/hooks/{name}", () =>
    HttpResponse.json(mutationResult("hooks-extensions", true))
  ),

  compozyApiMock.get("/api/settings/mcp-servers", ({ request }) => {
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
  compozyApiMock.put("/api/settings/mcp-servers/{name}", () =>
    HttpResponse.json(mutationResult("mcp-servers", true))
  ),
  compozyApiMock.delete("/api/settings/mcp-servers/{name}", () =>
    HttpResponse.json(mutationResult("mcp-servers", true))
  ),
  compozyApiMock.post("/api/settings/mcp-servers/{name}/auth/begin", () =>
    HttpResponse.json(mcpAuthBeginFixture)
  ),
  compozyApiMock.post("/api/settings/mcp-servers/{name}/auth/exchange", ({ request, params }) =>
    HttpResponse.json({
      ...mcpAuthStatusAuthenticatedFixture,
      ...mcpAuthTarget(request, String(params.name)),
    })
  ),
  compozyApiMock.post("/api/settings/mcp-servers/{name}/auth/logout", ({ request, params }) =>
    HttpResponse.json({
      ...mcpAuthStatusAuthenticatedFixture,
      ...mcpAuthTarget(request, String(params.name)),
      status: "needs_login",
      token_present: false,
      diagnostic: "login required",
    })
  ),

  compozyApiMock.get("/api/settings/apply", ({ request }) =>
    HttpResponse.json(applyRecordsForUrl(request))
  ),
  compozyApiMock.post("/api/settings/reload", () =>
    HttpResponse.json(settingsReloadBlockedFixture)
  ),

  compozyApiMock.post("/api/settings/actions/restart", () =>
    HttpResponse.json(settingsRestartResponseFixture, { status: 202 })
  ),
  compozyApiMock.get("/api/settings/actions/restart/{operation_id}", () =>
    HttpResponse.json(settingsRestartStatusFixture)
  ),
];
