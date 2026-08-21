import type { OsPaletteDomainRow } from "../hooks/use-os-palette-domain-search";
import type {
  OsPaletteSessionResult,
  OsPaletteTabResult,
  OsPaletteWorktreeResult,
} from "../hooks/use-os-palette-entities";
import type { PaletteRegistry, ResolvedPaletteCommand } from "./cmd-palette-types";
import { normalizeRankingText } from "./ranking/normalize";

/**
 * What the action panel offers for one selected row (`_uiux.md` S7, US-014).
 *
 * Actions are descriptors carrying an intent, never closures: the model stays a
 * pure function of the row and the registry, and the surface that renders it
 * decides how an intent runs. That is what lets view rows and grid tiles inherit
 * this panel unchanged (ADR-004) instead of growing a second action vocabulary.
 */

export const PALETTE_META_SECTION = "Command";
export const PALETTE_PANEL_EMPTY_COPY = "No actions match";

export type PaletteRowActionIntent =
  | { readonly kind: "run-command"; readonly commandId: string }
  | { readonly kind: "pin"; readonly commandId: string; readonly pinned: boolean }
  | { readonly kind: "open-shortcut-settings"; readonly commandId: string }
  | { readonly kind: "land-session"; readonly session: OsPaletteSessionResult }
  | { readonly kind: "go-to-tab"; readonly windowId: string }
  | { readonly kind: "close-tab"; readonly windowId: string }
  | { readonly kind: "scope-worktree"; readonly entry: OsPaletteWorktreeResult }
  | { readonly kind: "remove-worktree"; readonly entry: OsPaletteWorktreeResult }
  | { readonly kind: "open-domain-row"; readonly row: OsPaletteDomainRow };

export interface PaletteRowAction {
  /** Stable within the row; also the panel's cmdk value. */
  readonly id: string;
  readonly title: string;
  /** Lucide token resolved through `cmdPaletteIconRegistry`. */
  readonly icon: string;
  readonly section: string;
  /** Marked ↩ in the panel and run by ⏎ with the panel closed (US-014.AC-2). */
  readonly primary: boolean;
  readonly destructive: boolean;
  /** Formatted chord labels from the daemon keymap; never a TS literal (BR-6). */
  readonly chords: readonly string[];
  /** The same chords unformatted, for the capture-phase matcher (US-014.EC-3). */
  readonly bindings: readonly string[];
  readonly intent: PaletteRowActionIntent;
}

export interface PaletteRowActionSection {
  readonly title: string;
  readonly actions: readonly PaletteRowAction[];
}

export interface PaletteRowActionModel {
  /** The row's cmdk value — the panel closes when it leaves the list (US-014.EC-1). */
  readonly key: string;
  readonly title: string;
  readonly available: boolean;
  /** Verbatim runtime reason; empty when available (BR-8). */
  readonly reason: string;
  readonly sections: readonly PaletteRowActionSection[];
}

export type PaletteRowSubject =
  | { readonly kind: "command"; readonly command: ResolvedPaletteCommand }
  | { readonly kind: "session"; readonly session: OsPaletteSessionResult }
  | { readonly kind: "tab"; readonly tab: OsPaletteTabResult }
  | { readonly kind: "worktree"; readonly entry: OsPaletteWorktreeResult }
  | { readonly kind: "domain"; readonly row: OsPaletteDomainRow };

export interface PaletteRowActionInput {
  readonly subject: PaletteRowSubject;
  readonly registry: PaletteRegistry;
  /** Pinned command ids from the rank-signal snapshot; drives Pin vs Unpin. */
  readonly pins: readonly string[];
}

/**
 * The settings destination that owns binding and alias editing. Resolving it
 * through the registry rather than a hardcoded route keeps the deep-link honest:
 * when the daemon cannot serve it, the action is simply absent instead of
 * pointing at a page this client may not be able to open.
 */
const SHORTCUT_SETTINGS_COMMAND_ID = "settings.layouts";

/** Display labels and raw chords always come from the same registry entry. */
function keysFor(
  registry: PaletteRegistry,
  commandId: string
): { chords: readonly string[]; bindings: readonly string[] } {
  const command = registry.byId.get(commandId);
  return { chords: command?.chords ?? [], bindings: command?.bindings ?? [] };
}

function metaActions(
  command: ResolvedPaletteCommand,
  registry: PaletteRegistry,
  pins: readonly string[]
): readonly PaletteRowAction[] {
  const pinned = pins.includes(command.id);
  const actions: PaletteRowAction[] = [
    {
      id: "meta.pin",
      title: pinned ? "Unpin" : "Pin",
      icon: pinned ? "pin-off" : "pin",
      section: PALETTE_META_SECTION,
      primary: false,
      destructive: false,
      chords: [],
      bindings: [],
      intent: { kind: "pin", commandId: command.id, pinned: !pinned },
    },
  ];
  // Alias and shortcut mutations live on the settings surface. These actions
  // deep-link there rather than mutating the keymap from the palette row.
  if (registry.byId.has(SHORTCUT_SETTINGS_COMMAND_ID)) {
    actions.push(
      {
        id: "meta.alias",
        title: "Set alias…",
        icon: "text-cursor-input",
        section: PALETTE_META_SECTION,
        primary: false,
        destructive: false,
        chords: [],
        bindings: [],
        intent: { kind: "open-shortcut-settings", commandId: command.id },
      },
      {
        id: "meta.shortcut",
        title: "Set shortcut…",
        icon: "keyboard",
        section: PALETTE_META_SECTION,
        primary: false,
        destructive: false,
        chords: [],
        bindings: [],
        intent: { kind: "open-shortcut-settings", commandId: command.id },
      }
    );
  }
  return actions;
}

function commandModel(
  command: ResolvedPaletteCommand,
  registry: PaletteRegistry,
  pins: readonly string[]
): PaletteRowActionModel {
  const sections: PaletteRowActionSection[] = [];
  // An unavailable command lists meta-actions and its reason only — nothing that
  // would fire against a target the runtime already refused (US-014.EC-2).
  if (command.available) {
    sections.push({
      title: command.section.trim() || "Command",
      actions: [
        {
          id: "primary.run",
          title: command.title,
          icon: command.icon,
          section: command.section.trim() || "Command",
          primary: true,
          destructive: command.destructive,
          chords: command.chords,
          bindings: command.bindings,
          intent: { kind: "run-command", commandId: command.id },
        },
      ],
    });
  }
  sections.push({
    title: PALETTE_META_SECTION,
    actions: metaActions(command, registry, pins),
  });
  return {
    key: command.id,
    title: command.title,
    available: command.available,
    reason: command.available ? "" : command.reason,
    sections,
  };
}

function sessionModel(session: OsPaletteSessionResult): PaletteRowActionModel {
  return {
    key: `session:${session.sessionId}`,
    title: session.title,
    available: true,
    reason: "",
    sections: [
      {
        title: "Session",
        actions: [
          {
            id: "session.land",
            title: "Land session",
            icon: "square-terminal",
            section: "Session",
            primary: true,
            destructive: false,
            chords: [],
            bindings: [],
            intent: { kind: "land-session", session },
          },
        ],
      },
    ],
  };
}

function tabModel(tab: OsPaletteTabResult, registry: PaletteRegistry): PaletteRowActionModel {
  const closeKeys = keysFor(registry, "window.close");
  return {
    key: `tab:${tab.windowId}`,
    title: tab.label,
    available: true,
    reason: "",
    sections: [
      {
        title: "Tab",
        actions: [
          {
            id: "tab.go",
            title: "Go to tab",
            icon: "arrow-right",
            section: "Tab",
            primary: true,
            destructive: false,
            chords: [],
            bindings: [],
            intent: { kind: "go-to-tab", windowId: tab.windowId },
          },
          {
            id: "tab.close",
            title: "Close tab",
            icon: "x-square",
            section: "Tab",
            primary: false,
            destructive: false,
            // Closing a tab is reopenable, so it borrows the window-close chord
            // rather than the destructive treatment reserved for real loss.
            chords: closeKeys.chords,
            bindings: closeKeys.bindings,
            intent: { kind: "close-tab", windowId: tab.windowId },
          },
        ],
      },
    ],
  };
}

function worktreeModel(entry: OsPaletteWorktreeResult): PaletteRowActionModel {
  const actions: PaletteRowAction[] = [
    {
      id: "worktree.scope",
      title: "Scope to worktree",
      icon: "folder",
      section: "Worktree",
      primary: true,
      destructive: false,
      chords: [],
      bindings: [],
      intent: { kind: "scope-worktree", entry },
    },
  ];
  // Removal exists only for an adopted worktree; a discovered row has nothing
  // to remove yet, and offering it would be a control the runtime cannot honour.
  if (entry.worktree !== null) {
    actions.push({
      id: "worktree.remove",
      title: "Remove worktree",
      icon: "trash-2",
      section: "Worktree",
      primary: false,
      destructive: true,
      chords: [],
      bindings: [],
      intent: { kind: "remove-worktree", entry },
    });
  }
  return {
    key: `worktree:${entry.key}`,
    title: entry.name,
    available: true,
    reason: "",
    sections: [{ title: "Worktree", actions }],
  };
}

function domainModel(row: OsPaletteDomainRow): PaletteRowActionModel {
  return {
    key: row.key,
    title: row.label,
    available: true,
    reason: "",
    sections: [
      {
        title: "Open",
        actions: [
          {
            id: "domain.open",
            title: `Open ${row.label}`,
            icon: "arrow-right",
            section: "Open",
            primary: true,
            destructive: false,
            chords: [],
            bindings: [],
            intent: { kind: "open-domain-row", row },
          },
        ],
      },
    ],
  };
}

/**
 * Builds the panel model for one row.
 *
 * Meta-actions (Pin/Unpin, Set alias…, Set shortcut…) belong to command rows,
 * which are the only rows carrying a registry id to pin, alias or bind
 * (US-014.AC-3, ADR-001). Entity rows carry their domain actions instead —
 * rendering the meta trio there would be three controls that cannot work.
 */
export function paletteRowActions({
  subject,
  registry,
  pins,
}: PaletteRowActionInput): PaletteRowActionModel {
  switch (subject.kind) {
    case "command":
      return commandModel(subject.command, registry, pins);
    case "session":
      return sessionModel(subject.session);
    case "tab":
      return tabModel(subject.tab, registry);
    case "worktree":
      return worktreeModel(subject.entry);
    case "domain":
      return domainModel(subject.row);
  }
}

function matches(action: PaletteRowAction, needle: string): boolean {
  return (
    normalizeRankingText(action.title).text.includes(needle) ||
    normalizeRankingText(action.section).text.includes(needle)
  );
}

/**
 * Narrows the panel as the operator types. A section whose rows all drop out
 * collapses with them, and a query matching nothing leaves the panel open and
 * empty rather than inventing a fallback action (artboard 02).
 */
export function filterRowActions(
  sections: readonly PaletteRowActionSection[],
  query: string
): readonly PaletteRowActionSection[] {
  const needle = normalizeRankingText(query).text;
  if (needle === "") return sections;
  const filtered: PaletteRowActionSection[] = [];
  for (const section of sections) {
    const actions = section.actions.filter(action => matches(action, needle));
    if (actions.length > 0) filtered.push({ title: section.title, actions });
  }
  return filtered;
}

/** Flattened reading order — what ⏎ and the capture-phase chord matcher see. */
export function flattenRowActions(
  sections: readonly PaletteRowActionSection[]
): readonly PaletteRowAction[] {
  return sections.flatMap(section => section.actions);
}

/** The action ⏎ runs with the panel closed (US-014.AC-2). */
export function primaryRowAction(model: PaletteRowActionModel): PaletteRowAction | null {
  return flattenRowActions(model.sections).find(action => action.primary) ?? null;
}

export interface PaletteRowSources {
  readonly commands: readonly ResolvedPaletteCommand[];
  readonly sessions: readonly OsPaletteSessionResult[];
  readonly tabs: readonly OsPaletteTabResult[];
  readonly worktrees: readonly OsPaletteWorktreeResult[];
  readonly domainRows: readonly OsPaletteDomainRow[];
}

/**
 * Turns the highlighted cmdk value back into the thing it stands for.
 *
 * The palette's selection is a string because that is what the list needs; the
 * action panel needs the row itself. Resolving here — against the same lists the
 * rows were rendered from — is what keeps the panel from ever describing a row
 * that is no longer on screen.
 */
export function resolvePaletteRowSubject(
  sources: PaletteRowSources,
  value: string
): PaletteRowSubject | null {
  const command = sources.commands.find(entry => entry.id === value);
  if (command !== undefined) return { kind: "command", command };
  const session = sources.sessions.find(entry => `session:${entry.sessionId}` === value);
  if (session !== undefined) return { kind: "session", session };
  const tab = sources.tabs.find(entry => `tab:${entry.windowId}` === value);
  if (tab !== undefined) return { kind: "tab", tab };
  const worktree = sources.worktrees.find(entry => `worktree:${entry.key}` === value);
  if (worktree !== undefined) return { kind: "worktree", entry: worktree };
  const row = sources.domainRows.find(entry => entry.key === value);
  if (row !== undefined) return { kind: "domain", row };
  return null;
}
