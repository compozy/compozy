import { storyDefaultWorkspaceId } from "@/storybook/fintech-scenario";

import { buildPaletteRegistry } from "../lib/cmd-palette-registry";
import type {
  CmdPaletteAttachedClient,
  CmdPaletteCatalogResponse,
  CmdPaletteCommand,
  PaletteRegistry,
  ResolvedPaletteCommand,
} from "../lib/cmd-palette-types";

/**
 * A seeded catalog shaped like the daemon's: core shell/window/tab/app/settings
 * commands, one healthy source and one unhealthy extension source keeping its
 * last-known descriptors listed (BR-4). Stories and suites read this instead of
 * hand-writing rows, so no surface owns a second command list.
 */
export const cmdPaletteStoryClientId = "client-story-shell";

function baseCommand(
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

export function resolvedPaletteCommand(
  command: Partial<ResolvedPaletteCommand> & Pick<ResolvedPaletteCommand, "id" | "title">
): ResolvedPaletteCommand {
  const base = baseCommand(command);
  return {
    ...base,
    visible: true,
    reason: base.reason ?? "",
    chords: [],
    ...command,
  };
}

export function paletteRegistryFixture(
  commands: readonly ResolvedPaletteCommand[]
): PaletteRegistry {
  return {
    commands,
    byId: new Map(commands.map(entry => [entry.id, entry])),
    sources: [{ source: "core", status: "healthy" }],
    catalogRevision: "sha256:test",
    stale: false,
    daemonReachable: true,
  };
}

const focusedWindowPredicate = [
  { key: "window.focused", value: true, reason: "requires a focused window" },
];

export const cmdPaletteStoryCommands: CmdPaletteCommand[] = [
  baseCommand({
    id: "palette.open",
    title: "Command palette",
    icon: "command",
    availability_exempt: true,
    bindings: ["meta+KeyK"],
  }),
  baseCommand({
    id: "shortcuts.cheatsheet",
    title: "Keyboard shortcuts",
    icon: "keyboard",
    availability_exempt: true,
    bindings: ["meta+Slash"],
  }),
  baseCommand({
    id: "shell.sessions.toggle",
    title: "Toggle sessions",
    icon: "list",
  }),
  baseCommand({
    id: "session.new",
    title: "New session",
    icon: "square-terminal",
    availability_exempt: true,
    bindings: ["meta+KeyN"],
  }),
  baseCommand({
    id: "workspace.picker",
    title: "Workspace picker",
    icon: "folders",
    availability_exempt: true,
  }),
  baseCommand({
    id: "window.close",
    title: "Close window",
    section: "Window",
    icon: "x-square",
    bindings: ["meta+KeyW"],
    execution: { retry_safe: false, single_flight: true },
    when: focusedWindowPredicate,
  }),
  baseCommand({
    id: "window.minimize",
    title: "Minimize window",
    section: "Window",
    icon: "minus-square",
    bindings: ["meta+KeyM"],
    when: focusedWindowPredicate,
  }),
  baseCommand({
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
  baseCommand({
    id: "window.tab.new",
    title: "New tab",
    section: "Tabs",
    icon: "plus",
    bindings: ["meta+KeyT"],
  }),
  baseCommand({
    id: "window.tab.detach",
    title: "Move tab to new window",
    section: "Tabs",
    icon: "panels-top-left",
    when: [{ key: "window.stacked", value: true, reason: "needs a tab in a stack" }],
  }),
  baseCommand({
    id: "window.move",
    title: "Move window",
    section: "Window",
    icon: "panel-top",
    // Structural: a browser client has no desktop shell, so this row is fully
    // irrelevant here rather than disabled (US-037.AC-2).
    when: [{ key: "shell.desktop", value: true, reason: "requires an attached shell" }],
  }),
  baseCommand({
    id: "app.open.agents",
    title: "Open Agents",
    section: "Apps",
    icon: "bot",
    action: { kind: "navigate", app: "agents" },
  }),
  baseCommand({
    id: "app.open.tasks",
    title: "Open Tasks",
    section: "Apps",
    icon: "list-checks",
    action: { kind: "navigate", app: "tasks" },
  }),
  baseCommand({
    id: "settings.appearance",
    title: "Settings → Appearance",
    section: "Settings",
    icon: "palette",
    keywords: ["wallpaper", "theme", "dock"],
    action: { kind: "navigate", app: "settings", args: { pathname: "/settings/appearance" } },
  }),
  baseCommand({
    id: "palette.view.sessions",
    title: "Sessions",
    section: "Views",
    icon: "square-terminal",
    action: { kind: "view", view: "sessions" },
  }),
  {
    ...baseCommand({ id: "ext.notes.capture", title: "Capture note" }),
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
  profile_lens: { profile_lens_id: "00000000000000000000000000", profile_name: "default" },
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

export const cmdPaletteStoryRegistry = buildPaletteRegistry({
  catalog: {
    commands: cmdPaletteCatalogFixture.commands,
    sources: cmdPaletteCatalogFixture.sources,
    catalogRevision: cmdPaletteCatalogFixture.catalog_revision,
  },
  context: null,
  daemonReachable: true,
  stale: false,
  platform: "MacIntel",
});

/**
 * The execution surfaces need what the base catalog deliberately lacks: a
 * healthy extension command that declares arguments, and a destructive one that
 * declares its confirmation copy. They are added rather than substituted so the
 * base catalog keeps serving the unhealthy-source and disabled-row contracts
 * (`ext.notes.capture` stays unavailable there on purpose).
 */
export const cmdPaletteArgumentsCommand: CmdPaletteCommand = {
  ...baseCommand({ id: "ext.notes.capture_note", title: "Capture note" }),
  section: "Notes",
  icon: "notebook-pen",
  source: "ext.notes",
  bindings: ["alt+shift+KeyN"],
  alias: "cap",
  action: { kind: "tool", tool: "ext__notes__capture" },
  execution: { retry_safe: false, single_flight: true },
  arguments: [
    { name: "title", type: "text", required: true, placeholder: "Note title" },
    { name: "tag", type: "dropdown", required: false, options: ["inbox", "idea"] },
  ],
};

export const cmdPaletteDestructiveCommand: CmdPaletteCommand = {
  ...baseCommand({ id: "ext.notes.purge", title: "Purge archived notes" }),
  section: "Notes",
  icon: "trash-2",
  source: "ext.notes",
  destructive: true,
  action: { kind: "tool", tool: "ext__notes__purge" },
  execution: { retry_safe: false, single_flight: true },
  confirmation: {
    title: "Purge archived notes?",
    body: "Permanently deletes every archived note in this workspace.",
    confirm: "Purge",
  },
};

export function cmdPaletteExecutionCatalog(): CmdPaletteCatalogResponse {
  return {
    ...cmdPaletteCatalogFixture,
    catalog_revision: "sha256:story-catalog-execution",
    commands: [
      ...cmdPaletteStoryCommands,
      cmdPaletteArgumentsCommand,
      cmdPaletteDestructiveCommand,
    ],
  };
}

/** 60+ rows for the capped-group / overflow-note contract (US-001.EC-2). */
export function cmdPaletteAtScaleCatalog(): CmdPaletteCatalogResponse {
  const generated = Array.from({ length: 64 }, (_, index) =>
    baseCommand({
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
