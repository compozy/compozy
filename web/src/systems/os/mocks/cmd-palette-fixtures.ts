import { storyDefaultWorkspaceId } from "@/storybook/fintech-scenario";

import type {
  CmdPaletteAttachedClient,
  CmdPaletteCatalogResponse,
  CmdPaletteCommand,
} from "../lib/cmd-palette-types";

/**
 * A seeded catalog shaped like the daemon's: core shell/window/tab/app/settings
 * commands, one healthy source and one unhealthy extension source keeping its
 * last-known descriptors listed (BR-4). Stories and suites read this instead of
 * hand-writing rows, so no surface owns a second command list.
 */
export const cmdPaletteStoryClientId = "client-story-shell";

function coreCommand(
  command: Partial<CmdPaletteCommand> & Pick<CmdPaletteCommand, "id" | "title">
) {
  return {
    section: "Shell",
    icon: "command",
    source: "core",
    available: true,
    bindings: [],
    alias: null,
    destructive: false,
    availability_exempt: false,
    arguments: [],
    action: { kind: "client_op", op: command.id },
    execution: { retry_safe: true, single_flight: false },
    ...command,
  } satisfies CmdPaletteCommand;
}

const focusedWindowPredicate = [
  { key: "window.focused", value: true, reason: "requires a focused window" },
];

export const cmdPaletteStoryCommands: CmdPaletteCommand[] = [
  coreCommand({
    id: "palette.open",
    title: "Command palette",
    icon: "command",
    availability_exempt: true,
    bindings: ["meta+KeyK"],
  }),
  coreCommand({
    id: "shortcuts.cheatsheet",
    title: "Keyboard shortcuts",
    icon: "keyboard",
    availability_exempt: true,
    bindings: ["meta+Slash"],
  }),
  coreCommand({
    id: "shell.sessions.toggle",
    title: "Toggle sessions",
    icon: "list",
  }),
  coreCommand({
    id: "session.new",
    title: "New session",
    icon: "square-terminal",
    availability_exempt: true,
    bindings: ["meta+KeyN"],
  }),
  coreCommand({
    id: "window.close",
    title: "Close window",
    section: "Window",
    icon: "x-square",
    bindings: ["meta+KeyW"],
    execution: { retry_safe: false, single_flight: true },
    when: focusedWindowPredicate,
  }),
  coreCommand({
    id: "window.minimize",
    title: "Minimize window",
    section: "Window",
    icon: "minus-square",
    bindings: ["meta+KeyM"],
    when: focusedWindowPredicate,
  }),
  coreCommand({
    id: "window.merge_all",
    title: "Merge all windows",
    section: "Window",
    icon: "combine",
    when: [
      {
        key: "desktop.windowCount",
        operator: "greater_than_or_equal",
        value: 2,
        reason: "needs two windows on this desktop",
      },
    ],
  }),
  coreCommand({
    id: "window.tab.new",
    title: "New tab",
    section: "Tabs",
    icon: "plus",
    bindings: ["meta+KeyT"],
  }),
  coreCommand({
    id: "window.tab.detach",
    title: "Move tab to new window",
    section: "Tabs",
    icon: "panels-top-left",
    when: [{ key: "window.stacked", value: true, reason: "needs a tab in a stack" }],
  }),
  coreCommand({
    id: "window.move",
    title: "Move window",
    section: "Window",
    icon: "panel-top",
    // Structural: a browser client has no desktop shell, so this row is fully
    // irrelevant here rather than disabled (US-037.AC-2).
    when: [{ key: "shell.desktop", value: true, reason: "requires an attached shell" }],
  }),
  coreCommand({
    id: "app.open.agents",
    title: "Open Agents",
    section: "Apps",
    icon: "bot",
    action: { kind: "navigate", app: "agents" },
  }),
  coreCommand({
    id: "app.open.tasks",
    title: "Open Tasks",
    section: "Apps",
    icon: "list-checks",
    action: { kind: "navigate", app: "tasks" },
  }),
  coreCommand({
    id: "settings.appearance",
    title: "Settings → Appearance",
    section: "Settings",
    icon: "palette",
    keywords: ["wallpaper", "theme", "dock"],
    action: { kind: "navigate", app: "settings", args: { pathname: "/settings/appearance" } },
  }),
  coreCommand({
    id: "palette.view.sessions",
    title: "Sessions",
    section: "Views",
    icon: "square-terminal",
    action: { kind: "view", view: "sessions" },
  }),
  {
    ...coreCommand({ id: "ext.notes.capture", title: "Capture note" }),
    section: "Notes",
    icon: "notebook-pen",
    source: "ext.notes",
    available: false,
    reason: "extension notes is unhealthy (crash loop)",
    action: { kind: "tool", tool: "ext.notes.capture" },
    execution: { retry_safe: false, single_flight: true },
  },
];

export const cmdPaletteCatalogFixture: CmdPaletteCatalogResponse = {
  catalog_revision: "sha256:story-catalog-1",
  context_revision: "12",
  commands: cmdPaletteStoryCommands,
  sources: [
    { source: "core", status: "healthy" },
    {
      source: "ext.notes",
      status: "unhealthy",
      reason: "extension notes is unhealthy (crash loop)",
    },
  ],
};

/** 60+ rows for the capped-group / overflow-note contract (US-001.EC-2). */
export function cmdPaletteAtScaleCatalog(): CmdPaletteCatalogResponse {
  const generated = Array.from({ length: 64 }, (_, index) =>
    coreCommand({
      id: `desktop.switch.${index + 1}`,
      title: `Switch to desktop ${index + 1}`,
      section: "Desktops",
      icon: "monitor",
    })
  );
  return {
    ...cmdPaletteCatalogFixture,
    catalog_revision: "sha256:story-catalog-at-scale",
    commands: [...cmdPaletteStoryCommands, ...generated],
  };
}

/**
 * The story keymap, derived from the same catalog every story renders.
 *
 * Stories used to carry a third hand-written copy of the daemon keymap; it is
 * gone. A chord exists in a story only because the fixture catalog binds it,
 * which is the same rule production follows (BR-6).
 */
export const cmdPaletteStoryShortcuts: Record<string, readonly string[]> = Object.fromEntries(
  cmdPaletteStoryCommands.flatMap(command =>
    command.bindings.length > 0 ? [[command.id, command.bindings] as const] : []
  )
);

/** Overrides that put the table into its conflicted and rebound states. */
export const cmdPaletteStoryOverrides: Record<string, readonly string[]> = {
  "window.tab.new": ["meta+Digit3"],
  "sidebar.toggle": ["meta+KeyB"],
};

export const cmdPaletteStoryReboundOverrides: Record<string, readonly string[]> = {
  "palette.open": ["meta+shift+KeyP"],
};

export const cmdPaletteClientsFixture: CmdPaletteAttachedClient[] = [
  {
    client_id: cmdPaletteStoryClientId,
    kind: "browser",
    workspace: storyDefaultWorkspaceId,
    attached_at: "2026-04-17T18:00:00Z",
  },
];
