import type { AghApiOkJsonResponseFor } from "@/storybook/openapi-msw";
import { storyDefaultWorkspaceId } from "@/storybook/fintech-scenario";
import type { SettingsWindowManagerSection } from "@/systems/settings";

type WindowManagerSnapshotWire = AghApiOkJsonResponseFor<
  "get",
  "/api/workspaces/{workspace_id}/window-manager"
>;

type WindowManagerResourceRecord = AghApiOkJsonResponseFor<
  "get",
  "/api/workspaces/{workspace_id}/window-manager/layout-profiles"
>["records"][number];

/**
 * The resource envelope types `spec` as an open record, so the profile shape is
 * restated here. The unions are the daemon's, not invented: `aspect_variant` and
 * `overflow_policy` come from `internal/windowmanager/resource_codec.go`, and the
 * document version is `SnapshotVersion` (`internal/windowmanager/types.go:9`).
 */
type WindowManagerLayoutProfileWire = {
  version: 1;
  id: string;
  display_name: string;
  aspect_variant: "any" | "landscape" | "portrait";
  participant_slots: string[];
  overflow_policy: "reject" | "stack";
  document: WindowManagerLayoutDocumentWire;
};

type WindowManagerLayoutResourceFixture = Omit<WindowManagerResourceRecord, "spec"> & {
  spec: WindowManagerLayoutProfileWire;
};

type WindowManagerLayoutDocumentWire = AghApiOkJsonResponseFor<
  "get",
  "/api/workspaces/{workspace_id}/window-manager/layout"
>;

export const settingsWindowManagerSectionFixture: SettingsWindowManagerSection = {
  section: "window-manager",
  scope: "global",
  available_scopes: ["global"],
  config: {
    new_window_policy: "floating",
    small_viewport_policy: "stack",
    focus_policy: "click_directional",
    focus_wrap: true,
    focus_follows_pointer: false,
    raise_on_focus: true,
    drag_away_policy: "window",
    group_move_modifier: "alt",
    swap_modifier: "shift",
    history_limit: 100,
    desktop_transition: "slide",
    gaps: { inner: 8, top: 8, right: 10, bottom: 8, left: 10 },
    snap: {
      edge_band: 32,
      corner_reach: 150,
      exit_slack: 16,
      repeat_ratios: [0.5, 0.666667, 0.333333],
    },
    bindings: { top_center: "zoom", bottom_center: "reserved" },
    shortcuts: { "window.focus.left": "meta+alt+ArrowLeft" },
  },
};

export const settingsWindowManagerDesktopIds = {
  build: "desktop-build",
  review: "desktop-review",
  focus: "desktop-focus",
} as const;

/**
 * A workspace dense enough to exercise the layout stage: three desktops, four
 * groups, a nested split, a stack, a floating window and a minimized one. The
 * previous single-leaf fixture is why the density problem never showed up in a
 * review capture.
 */
export const settingsWindowManagerSnapshotFixture: WindowManagerSnapshotWire = {
  version: 2,
  workspace_id: storyDefaultWorkspaceId,
  revision: 41,
  desktops: [
    {
      id: settingsWindowManagerDesktopIds.build,
      name: "Build",
      order: 0,
      purpose: "standard",
      groups: [
        {
          id: "group-main",
          frame: { x: 0, y: 0, width: 0.63, height: 1 },
          root: {
            id: "split-main",
            kind: "split",
            axis: "vertical",
            weights: [0.62, 0.38],
            children: [
              { id: "leaf-tasks", kind: "leaf", window_id: "app:tasks" },
              {
                id: "split-main-lower",
                kind: "split",
                axis: "horizontal",
                weights: [0.5, 0.5],
                children: [
                  {
                    id: "stack-sessions",
                    kind: "stack",
                    window_ids: ["session:ses_8f2a", "session:ses_51c7"],
                    active_id: "session:ses_8f2a",
                  },
                  { id: "leaf-dashboard", kind: "leaf", window_id: "app:dashboard" },
                ],
              },
            ],
          },
        },
        {
          id: "group-side",
          frame: { x: 0.63, y: 0, width: 0.37, height: 1 },
          root: { id: "leaf-network", kind: "leaf", window_id: "app:network" },
        },
      ],
      floating: ["app:vault"],
    },
    {
      id: settingsWindowManagerDesktopIds.review,
      name: "Review",
      order: 1,
      purpose: "standard",
      groups: [
        {
          id: "group-top",
          frame: { x: 0, y: 0, width: 1, height: 0.56 },
          root: {
            id: "split-review",
            kind: "split",
            axis: "horizontal",
            weights: [0.5, 0.5],
            children: [
              { id: "leaf-loops", kind: "leaf", window_id: "app:loops" },
              { id: "leaf-marketplace", kind: "leaf", window_id: "app:marketplace" },
            ],
          },
        },
        {
          id: "group-bottom",
          frame: { x: 0, y: 0.56, width: 1, height: 0.44 },
          root: { id: "leaf-jobs", kind: "leaf", window_id: "app:jobs" },
        },
      ],
      floating: ["app:agents"],
    },
    {
      id: settingsWindowManagerDesktopIds.focus,
      name: "Deep work",
      order: 2,
      purpose: "focus",
      focus_owner: "app:knowledge",
      groups: [
        {
          id: "group-focus",
          frame: { x: 0, y: 0, width: 1, height: 1 },
          root: { id: "leaf-knowledge", kind: "leaf", window_id: "app:knowledge" },
        },
      ],
      floating: [],
    },
  ],
  windows: {
    "app:tasks": tiled("app:tasks", "tasks", "/tasks", settingsWindowManagerDesktopIds.build),
    "session:ses_8f2a": {
      ...tiled(
        "session:ses_8f2a",
        "session",
        "/sessions/ses_8f2a",
        settingsWindowManagerDesktopIds.build
      ),
      instance_key: "ses_8f2a",
      placement: "stacked",
    },
    "session:ses_51c7": {
      ...tiled(
        "session:ses_51c7",
        "session",
        "/sessions/ses_51c7",
        settingsWindowManagerDesktopIds.build
      ),
      instance_key: "ses_51c7",
      placement: "stacked",
    },
    "app:dashboard": tiled(
      "app:dashboard",
      "dashboard",
      "/",
      settingsWindowManagerDesktopIds.build
    ),
    "app:network": tiled(
      "app:network",
      "network",
      "/network",
      settingsWindowManagerDesktopIds.build
    ),
    "app:vault": {
      ...tiled("app:vault", "vault", "/vault", settingsWindowManagerDesktopIds.build),
      placement: "floating",
      floating_rect: { x: 0.52, y: 0.55, width: 0.3, height: 0.34 },
    },
    "app:loops": tiled("app:loops", "loops", "/loops", settingsWindowManagerDesktopIds.review),
    "app:marketplace": tiled(
      "app:marketplace",
      "marketplace",
      "/marketplace",
      settingsWindowManagerDesktopIds.review
    ),
    "app:jobs": tiled("app:jobs", "jobs", "/jobs", settingsWindowManagerDesktopIds.review),
    "app:agents": {
      ...tiled("app:agents", "agents", "/agents", settingsWindowManagerDesktopIds.review),
      placement: "floating",
      minimized: true,
      floating_rect: { x: 0.58, y: 0.62, width: 0.28, height: 0.3 },
    },
    "app:knowledge": tiled(
      "app:knowledge",
      "knowledge",
      "/knowledge",
      settingsWindowManagerDesktopIds.focus
    ),
  },
  history: { undo: [], redo: [] },
  overrides: {},
  updated_at: "2026-07-23T01:00:00Z",
};

function tiled(
  id: string,
  app: string,
  pathname: string,
  desktopId: string
): WindowManagerSnapshotWire["windows"][string] {
  return {
    id,
    app,
    route: { pathname, search: {} },
    placement: "tiled",
    desktop_id: desktopId,
    floating_rect: { x: 0.2, y: 0.2, width: 0.4, height: 0.4 },
    minimized: false,
  };
}

export const windowManagerLayoutDocumentFixture: WindowManagerLayoutDocumentWire = {
  version: 2,
  workspace_id: storyDefaultWorkspaceId,
  desktops: settingsWindowManagerSnapshotFixture.desktops,
  windows: settingsWindowManagerSnapshotFixture.windows,
  overrides: {},
};

export const windowManagerLayoutResourceFixture: WindowManagerLayoutResourceFixture = {
  kind: "window_layout",
  id: "launch-console",
  version: 3,
  scope: { kind: "workspace", id: storyDefaultWorkspaceId },
  owner: { kind: "daemon", id: "storybook" },
  source: { kind: "operator", id: "storybook" },
  spec: {
    version: 1,
    id: "launch-console",
    display_name: "Launch console",
    aspect_variant: "landscape",
    participant_slots: Object.keys(settingsWindowManagerSnapshotFixture.windows),
    overflow_policy: "stack",
    document: windowManagerLayoutDocumentFixture,
  },
  created_at: "2026-07-22T22:00:00Z",
  updated_at: "2026-07-23T01:00:00Z",
};
