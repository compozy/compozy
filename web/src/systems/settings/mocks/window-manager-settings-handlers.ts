import { HttpResponse, type HttpHandler } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";
import type { SettingsWindowManagerSection } from "../types";
import { settingsReloadAppliedFixture } from "./fixtures";
import { settingsWindowManagerSectionFixture } from "./window-manager-fixtures";

const sections = new Map<string, SettingsWindowManagerSection>();

/** Starts each story or test with an independent window-manager settings fixture. */
export function resetWindowManagerSettingsMockState(): void {
  sections.clear();
}

/** Persisted settings belong to the profile and workspace, not the requesting shell client. */
function scopedSection(request: Request) {
  const query = new URL(request.url).searchParams;
  const scope = query.get("scope") === "workspace" ? "workspace" : "user";
  const workspaceId = scope === "workspace" ? (query.get("workspace_id") ?? "") : "";
  const key = JSON.stringify([query.get("profile") ?? "default", scope, workspaceId]);
  const section = sections.get(key) ?? {
    ...structuredClone(settingsWindowManagerSectionFixture),
    scope,
    workspace_id: workspaceId,
  };
  return { key, section };
}

/** Echoes writes and subsequent reads from the same scoped state for interactive stories. */
export const windowManagerSettingsHandlers: HttpHandler[] = [
  compozyApiMock.get("/api/settings/window-manager", ({ request }) =>
    HttpResponse.json(scopedSection(request).section)
  ),
  compozyApiMock.patch("/api/settings/window-manager", async ({ request }) => {
    const { key, section } = scopedSection(request);
    const body = await request.json();
    const config = { ...section.config, ...body.config };
    if (body.preserve_shortcuts) {
      config.shortcuts = section.config.shortcuts;
      config.global_shortcuts = section.config.global_shortcuts;
    }
    if (body.shortcuts != null) config.shortcuts = body.shortcuts;
    if (body.global_shortcuts != null) config.global_shortcuts = body.global_shortcuts;
    const saved: SettingsWindowManagerSection = {
      ...section,
      config,
      aliases: body.aliases ?? section.aliases,
      effective_shortcuts: { ...section.defaults, ...config.shortcuts },
    };
    sections.set(key, saved);
    return HttpResponse.json({ ...saved, apply: settingsReloadAppliedFixture });
  }),
];
