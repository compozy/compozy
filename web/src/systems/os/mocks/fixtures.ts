import type {
  CompozyApiJsonResponseFor,
  CompozyApiOkJsonResponseFor,
} from "@/storybook/openapi-msw";
import {
  storyDefaultWorkspaceId,
  storySessionIds,
  storyWorkspaceIds,
} from "@/storybook/fintech-scenario";
import type {
  OperatorNotificationEventPayload,
  SessionAttentionEventPayload,
} from "@/systems/session";

import { resolveAppForPath } from "../lib/app-registry";

export const windowManagerStoryDesktopId = "desktop-launch";
export const windowManagerStoryWindowId = "w-story-settings";

export const osSessionAttentionEventFixture: SessionAttentionEventPayload = {
  session_id: storySessionIds.cto,
  workspace_id: storyWorkspaceIds.hq,
  from: "running",
  to: "waiting-for-input",
  class: "needs-you",
  at: "2026-04-17T18:11:30Z",
};

export const osOperatorNotificationEventFixture: OperatorNotificationEventPayload = {
  notification_id: "ntf_story_deps_audit",
  session_id: storySessionIds.cto,
  workspace_id: storyWorkspaceIds.hq,
  title: "Dependency audit done",
  body: "3 findings, 1 high severity",
  at: "2026-04-17T18:12:00Z",
};

/**
 * Window IDs are opaque and generated (ADR-010); stories keep them
 * deterministic per app so MSW handlers and assertions can reference them.
 */
function storyWindowId(app: string, instanceKey: string | null): string {
  return instanceKey ? `w-story-${app}-${instanceKey}` : `w-story-${app}`;
}

type WindowManagerStorySnapshot = CompozyApiOkJsonResponseFor<
  "get",
  "/api/workspaces/{workspace_id}/window-manager"
>;

export const windowManagerSnapshotFixture: WindowManagerStorySnapshot = {
  // SnapshotVersion (internal/windowmanager/types.go:9).
  version: 3,
  workspace_id: storyDefaultWorkspaceId,
  revision: 12,
  desktops: [
    {
      id: windowManagerStoryDesktopId,
      name: "Launch",
      order: 0,
      purpose: "standard",
      groups: [
        {
          id: "group-settings",
          frame: { x: 0, y: 0, width: 1, height: 1 },
          root: {
            id: "leaf-settings",
            kind: "leaf",
            window_id: windowManagerStoryWindowId,
          },
        },
      ],
      floating: [],
      floating_stacks: [],
    },
  ],
  windows: {
    [windowManagerStoryWindowId]: {
      id: windowManagerStoryWindowId,
      app: "settings",
      route: { pathname: "/settings/layouts", search: {} },
      nav_stack: [],
      pinned: false,
      placement: "tiled",
      desktop_id: windowManagerStoryDesktopId,
      floating_rect: { x: 0.08, y: 0.08, width: 0.84, height: 0.84 },
      minimized: false,
    },
  },
  closed_entry_count: 0,
  overrides: {},
  updated_at: "2026-07-23T01:00:00Z",
};

function storyRoute(initialEntry: string): {
  pathname: string;
  search: Record<string, string | string[]>;
} {
  const url = new URL(initialEntry, "http://storybook.local");
  const search: Record<string, string | string[]> = {};
  for (const key of new Set(url.searchParams.keys())) {
    const values = url.searchParams.getAll(key);
    search[key] = values.length === 1 ? values[0] : values;
  }
  return { pathname: url.pathname, search };
}

/** Builds the daemon-owned window that hosts an app-route story. */
export function windowManagerStorySnapshot(
  initialEntry: string,
  workspaceId: string = storyDefaultWorkspaceId
): WindowManagerStorySnapshot {
  const route = storyRoute(initialEntry);
  const resolved = resolveAppForPath(route.pathname);
  const snapshot = structuredClone(windowManagerSnapshotFixture);
  snapshot.workspace_id = workspaceId;
  if (resolved === null) return snapshot;

  const windowId = storyWindowId(resolved.app.id, resolved.instanceKey);
  const desktop = snapshot.desktops[0];
  if (desktop?.groups[0]?.root.kind === "leaf") {
    desktop.groups[0].root.window_id = windowId;
  }
  const baseWindow = snapshot.windows[windowManagerStoryWindowId];
  if (baseWindow === undefined) return snapshot;
  snapshot.windows = {
    [windowId]: {
      ...baseWindow,
      id: windowId,
      app: resolved.app.id,
      ...(resolved.instanceKey === null ? {} : { instance_key: resolved.instanceKey }),
      route,
    },
  };
  return snapshot;
}

export function windowManagerClientFixture(
  clientId: string,
  workspaceId: string = storyDefaultWorkspaceId,
  focusedWindowId: string = windowManagerStoryWindowId
): CompozyApiJsonResponseFor<"post", "/api/workspaces/{workspace_id}/window-manager/clients", 201> {
  return {
    workspace_id: workspaceId,
    client_id: clientId,
    kind: "browser",
    presentation_revision: 1,
    context_revision: 1,
    active_desktop_id: windowManagerStoryDesktopId,
    focused_window_id: focusedWindowId,
    focus_order: [focusedWindowId],
    stack_active: {},
    global_shortcuts: [],
    palette_context: {
      window_focused: true,
      window_floating: false,
      window_stacked: true,
      desktop_window_count: 1,
      scope_global: false,
      shell_desktop: false,
      workspace_trusted: true,
    },
    connected_at: "2026-07-23T01:00:00Z",
  };
}
