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
  profile_id: "00000000000000000000000000",
  profile_name: "default",
  session_id: storySessionIds.cto,
  workspace_id: storyWorkspaceIds.hq,
  from: "running",
  to: "waiting-for-input",
  class: "needs-you",
  at: "2026-04-17T18:11:30Z",
};

export const osOperatorNotificationEventFixture: OperatorNotificationEventPayload = {
  profile_id: "00000000000000000000000000",
  profile_name: "default",
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

export const windowManagerStoryTabId = "tab-story-settings";

type WindowManagerStoryTab = WindowManagerStorySnapshot["desktops"][number]["tabs"][number];

const settingsStoryTab: WindowManagerStoryTab = {
  id: windowManagerStoryTabId,
  pinned: false,
  root: { id: "leaf-settings", kind: "leaf", window_id: windowManagerStoryWindowId },
};

export const windowManagerSnapshotFixture: WindowManagerStorySnapshot = {
  // SnapshotVersion (internal/windowmanager/types.go).
  version: 6,
  workspace_id: storyDefaultWorkspaceId,
  revision: 12,
  desktops: [
    {
      id: windowManagerStoryDesktopId,
      name: "Launch",
      order: 0,
      active_tab_id: windowManagerStoryTabId,
      tabs: [settingsStoryTab],
    },
  ],
  windows: {
    [windowManagerStoryWindowId]: {
      id: windowManagerStoryWindowId,
      app: "settings",
      route: { pathname: "/settings/layouts", search: {} },
      nav_stack: [],
      desktop_id: windowManagerStoryDesktopId,
      tab_id: windowManagerStoryTabId,
    },
  },
  closed_entry_count: 0,
  overrides: {},
  updated_at: "2026-07-23T01:00:00Z",
};

function storyTabWindow(
  id: string,
  app: string,
  pathname: string,
  tabId: string,
  instanceKey?: string
): WindowManagerStorySnapshot["windows"][string] {
  return {
    id,
    app,
    ...(instanceKey === undefined ? {} : { instance_key: instanceKey }),
    route: { pathname, search: {} },
    nav_stack: [],
    desktop_id: windowManagerStoryDesktopId,
    tab_id: tabId,
  };
}

/**
 * A busy bar: a pinned Dashboard tab, a Tasks tab, a split tab holding a session beside Files,
 * and an active Settings tab. Stories and tests that exercise the tab bar start from it.
 */
export const windowManagerTabsSnapshotFixture: WindowManagerStorySnapshot = {
  ...windowManagerSnapshotFixture,
  desktops: [
    {
      id: windowManagerStoryDesktopId,
      name: "Launch",
      order: 0,
      active_tab_id: windowManagerStoryTabId,
      tabs: [
        {
          id: "tab-story-dashboard",
          pinned: true,
          root: { id: "leaf-dashboard", kind: "leaf", window_id: "w-story-dashboard" },
        },
        {
          id: "tab-story-tasks",
          pinned: false,
          root: { id: "leaf-tasks", kind: "leaf", window_id: "w-story-tasks" },
        },
        {
          id: "tab-story-split",
          pinned: false,
          root: {
            id: "split-story",
            kind: "split",
            axis: "horizontal",
            weights: [0.6, 0.4],
            children: [
              { id: "leaf-session", kind: "leaf", window_id: "w-story-session" },
              { id: "leaf-files", kind: "leaf", window_id: "w-story-files" },
            ],
          },
        },
        settingsStoryTab,
      ],
    },
  ],
  windows: {
    "w-story-dashboard": storyTabWindow(
      "w-story-dashboard",
      "dashboard",
      "/",
      "tab-story-dashboard"
    ),
    "w-story-tasks": storyTabWindow("w-story-tasks", "tasks", "/tasks", "tab-story-tasks"),
    "w-story-session": storyTabWindow(
      "w-story-session",
      "session",
      `/sessions/${storySessionIds.cto}`,
      "tab-story-split",
      storySessionIds.cto
    ),
    "w-story-files": storyTabWindow("w-story-files", "files", "/files", "tab-story-split"),
    ...windowManagerSnapshotFixture.windows,
  },
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
  const root = snapshot.desktops[0]?.tabs[0]?.root;
  if (root?.kind === "leaf") root.window_id = windowId;
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
    active_tab: { [windowManagerStoryDesktopId]: windowManagerStoryTabId },
    // Retired through v0.7; the daemon still sends it empty.
    stack_active: {},
    global_shortcuts: [],
    palette_context: {
      window_focused: true,
      window_in_split: false,
      // Retired through v0.7; the daemon still sends them false.
      window_floating: false,
      window_stacked: false,
      desktop_window_count: 1,
      scope_global: false,
      shell_desktop: false,
      workspace_trusted: true,
    },
    connected_at: "2026-07-23T01:00:00Z",
  };
}
