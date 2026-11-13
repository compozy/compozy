import { z } from "zod";

import type { CompozyApiOkJsonResponseFor } from "@/storybook/openapi-msw";
import { storyDefaultWorkspaceId } from "@/storybook/fintech-scenario";
import { globalShortcutRegistrationSchema } from "@/systems/os";
import { cmdPaletteStoryCommands } from "@/systems/os/mocks";
import type { SettingsWindowManagerSection } from "@/systems/settings";

export function settingsGlobalShortcutRegistrations(
  entries: z.input<typeof globalShortcutRegistrationSchema>[]
): z.infer<typeof globalShortcutRegistrationSchema>[] {
  return z.array(globalShortcutRegistrationSchema).parse(entries);
}

type WindowManagerSnapshotWire = CompozyApiOkJsonResponseFor<
  "get",
  "/api/workspaces/{workspace_id}/window-manager"
>;

type WindowManagerResourceRecord = CompozyApiOkJsonResponseFor<
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

type WindowManagerLayoutDocumentWire = CompozyApiOkJsonResponseFor<
  "get",
  "/api/workspaces/{workspace_id}/window-manager/layout"
>;

/**
 * The registry the settings section serves, taken from the one story catalog so
 * the shortcut table, the palette and the cheatsheet cannot disagree about a
 * command's id, title or source.
 */
const storyRegistryCommands = cmdPaletteStoryCommands.map(command => ({
  id: command.id,
  title: command.title,
  section: command.section,
  source: command.source,
}));

const storyEffectiveShortcuts: Record<string, string[]> = Object.fromEntries(
  cmdPaletteStoryCommands.map(command => [
    command.id,
    command.id === "window.tab.new" ? ["meta+Digit3"] : [...command.bindings],
  ])
);

const storyAliases: Record<string, string> = Object.fromEntries(
  cmdPaletteStoryCommands.flatMap(command =>
    command.alias === null ? [] : [[command.id, command.alias] as const]
  )
);

export const settingsWindowManagerSectionFixture: SettingsWindowManagerSection = {
  section: "window-manager",
  scope: "workspace",
  workspace_id: storyDefaultWorkspaceId,
  available_scopes: ["user", "workspace"],
  commands: storyRegistryCommands,
  aliases: storyAliases,
  extension_defaults: [],
  diagnostics: [],
  config: {
    new_window_policy: "beside_focus",
    small_viewport_policy: "stack",
    focus_policy: "click_directional",
    focus_wrap: true,
    focus_follows_pointer: false,
    // Retired keys: the generated `SettingsWindowManagerConfigPayload` still
    // requires them, so this daemon payload carries them; the client parser
    // drops them and no domain config or save holds them (remove in v0.8.0).
    raise_on_focus: true,
    drag_away_policy: "window",
    group_move_modifier: "alt",
    swap_modifier: "shift",
    history_limit: 100,
    nav_stack_limit: 50,
    closed_entry_limit: 20,
    desktop_transition: "slide",
    gaps: { inner: 0, top: 0, right: 0, bottom: 0, left: 0 },
    snap: {
      edge_band: 32,
      corner_reach: 150,
      exit_slack: 16,
      repeat_ratios: [0.5, 0.666667, 0.333333],
    },
    bindings: { top_center: "zoom", bottom_center: "zoom" },
    shortcuts: { "window.tab.new": ["meta+Digit3"] },
    global_shortcuts: { "palette.summon.global": "meta+shift+Space" },
  },
  defaults: Object.fromEntries(
    cmdPaletteStoryCommands.map(command => [command.id, [...command.bindings]])
  ),
  effective_shortcuts: storyEffectiveShortcuts,
  global_shortcuts: settingsGlobalShortcutRegistrations([
    {
      command_id: "palette.summon.global",
      intended_chord: "meta+shift+Space",
      status: "unsupported",
    },
  ]),
};

export const settingsWindowManagerDesktopIds = {
  build: "desktop-build",
  review: "desktop-review",
  focus: "desktop-focus",
} as const;

/**
 * A workspace dense enough to exercise the layout stage: three desktops, a
 * pinned tab, a nested split, and single-window tabs beside them.
 */
export const settingsWindowManagerSnapshotFixture: WindowManagerSnapshotWire = {
  version: 6,
  workspace_id: storyDefaultWorkspaceId,
  revision: 41,
  desktops: [
    {
      id: settingsWindowManagerDesktopIds.build,
      name: "Build",
      order: 0,
      active_tab_id: "tab-main",
      tabs: [
        {
          id: "tab-dashboard",
          pinned: true,
          root: { id: "leaf-dashboard", kind: "leaf", window_id: "app:dashboard" },
        },
        {
          id: "tab-main",
          pinned: false,
          root: {
            id: "split-main",
            kind: "split",
            axis: "horizontal",
            weights: [0.62, 0.38],
            children: [
              { id: "leaf-tasks", kind: "leaf", window_id: "app:tasks" },
              {
                id: "split-main-side",
                kind: "split",
                axis: "vertical",
                weights: [0.5, 0.5],
                children: [
                  { id: "leaf-session-a", kind: "leaf", window_id: "session:ses_8f2a" },
                  { id: "leaf-skills", kind: "leaf", window_id: "app:skills" },
                ],
              },
            ],
          },
        },
        {
          id: "tab-session-b",
          pinned: false,
          root: { id: "leaf-session-b", kind: "leaf", window_id: "session:ses_51c7" },
        },
        {
          id: "tab-vault",
          pinned: false,
          root: { id: "leaf-vault", kind: "leaf", window_id: "app:vault" },
        },
      ],
    },
    {
      id: settingsWindowManagerDesktopIds.review,
      name: "Review",
      order: 1,
      active_tab_id: "tab-review",
      tabs: [
        {
          id: "tab-review",
          pinned: false,
          root: {
            id: "split-review",
            kind: "split",
            axis: "vertical",
            weights: [0.56, 0.44],
            children: [
              {
                id: "split-review-top",
                kind: "split",
                axis: "horizontal",
                weights: [0.5, 0.5],
                children: [
                  { id: "leaf-loops", kind: "leaf", window_id: "app:loops" },
                  { id: "leaf-marketplace", kind: "leaf", window_id: "app:marketplace" },
                ],
              },
              { id: "leaf-automations", kind: "leaf", window_id: "app:automations" },
            ],
          },
        },
        {
          id: "tab-agents",
          pinned: false,
          root: { id: "leaf-agents", kind: "leaf", window_id: "app:agents" },
        },
      ],
    },
    {
      id: settingsWindowManagerDesktopIds.focus,
      name: "Deep work",
      order: 2,
      active_tab_id: "tab-terminal",
      tabs: [
        {
          id: "tab-terminal",
          pinned: false,
          root: { id: "leaf-terminal", kind: "leaf", window_id: "app:terminal" },
        },
      ],
    },
  ],
  windows: {
    "app:dashboard": shown("app:dashboard", "dashboard", "/", "build", "tab-dashboard"),
    "app:tasks": shown("app:tasks", "tasks", "/tasks", "build", "tab-main"),
    "session:ses_8f2a": {
      ...shown("session:ses_8f2a", "session", "/sessions/ses_8f2a", "build", "tab-main"),
      instance_key: "ses_8f2a",
    },
    "app:skills": shown("app:skills", "skills", "/skills", "build", "tab-main"),
    "session:ses_51c7": {
      ...shown("session:ses_51c7", "session", "/sessions/ses_51c7", "build", "tab-session-b"),
      instance_key: "ses_51c7",
    },
    "app:vault": shown("app:vault", "vault", "/vault", "build", "tab-vault"),
    "app:loops": shown("app:loops", "loops", "/loops", "review", "tab-review"),
    "app:marketplace": shown(
      "app:marketplace",
      "marketplace",
      "/marketplace",
      "review",
      "tab-review"
    ),
    "app:automations": shown(
      "app:automations",
      "automations",
      "/automations",
      "review",
      "tab-review"
    ),
    "app:agents": shown("app:agents", "agents", "/agents", "review", "tab-agents"),
    "app:terminal": shown("app:terminal", "terminal", "/terminal", "focus", "tab-terminal"),
  },
  closed_entry_count: 0,
  overrides: {},
  updated_at: "2026-07-23T01:00:00Z",
};

function shown(
  id: string,
  app: string,
  pathname: string,
  desktop: keyof typeof settingsWindowManagerDesktopIds,
  tabId: string
): WindowManagerSnapshotWire["windows"][string] {
  return {
    id,
    app,
    route: { pathname, search: {} },
    nav_stack: [],
    desktop_id: settingsWindowManagerDesktopIds[desktop],
    tab_id: tabId,
  };
}

export const windowManagerLayoutDocumentFixture: WindowManagerLayoutDocumentWire = {
  version: 6,
  workspace_id: storyDefaultWorkspaceId,
  desktops: settingsWindowManagerSnapshotFixture.desktops,
  windows: settingsWindowManagerSnapshotFixture.windows,
  overrides: {},
};

export const windowManagerLayoutResourceFixture: WindowManagerLayoutResourceFixture = {
  kind: "window_layout",
  id: "launch-console",
  version: 4,
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

/**
 * A profile saved before tabs and splits: the record keeps the version 5
 * document it was stored with, and the daemon converts it when it is opened.
 */
export const windowManagerEarlierLayoutResourceFixture: WindowManagerResourceRecord = {
  kind: "window_layout",
  id: "classic-grid",
  version: 2,
  scope: { kind: "user" },
  owner: { kind: "daemon", id: "storybook" },
  source: { kind: "operator", id: "storybook" },
  spec: {
    version: 1,
    id: "classic-grid",
    display_name: "Classic grid",
    aspect_variant: "any",
    participant_slots: ["app:tasks", "app:terminal"],
    overflow_policy: "stack",
    document: {
      version: 5,
      workspace_id: "",
      desktops: [
        {
          id: "desktop-main",
          name: "Main",
          order: 0,
          groups: [
            {
              id: "group-main",
              frame: { x: 0, y: 0, width: 1, height: 1 },
              root: {
                id: "split-main",
                kind: "split",
                axis: "horizontal",
                weights: [0.5, 0.5],
                children: [
                  { id: "leaf-tasks", kind: "leaf", window_id: "app:tasks" },
                  { id: "leaf-terminal", kind: "leaf", window_id: "app:terminal" },
                ],
              },
            },
          ],
          floating: [],
          floating_stacks: [],
        },
      ],
      windows: {},
      overrides: {},
    },
  },
  created_at: "2026-06-01T10:00:00Z",
  updated_at: "2026-06-01T10:00:00Z",
};
