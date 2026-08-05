import type { Unstable_TriggerItem } from "@assistant-ui/react";
import type { LucideIcon } from "lucide-react";
import {
  Blocks,
  Bot,
  Brain,
  CircleHelp,
  Eraser,
  GitFork,
  Info,
  ListTodo,
  Minimize2,
  Play,
  SearchCode,
  Settings2,
  SquareSlash,
  Target,
  Zap,
} from "lucide-react";

export type SessionCommandLane = "builtin" | "agent" | "skill";

export interface SessionComposerCommand {
  /** Stable catalog identity; source qualification stays intact. */
  id: string;
  /** Exact text written into the prompt and preserved in the transcript. */
  token: string;
  /** Human-readable command name for the menu. */
  label: string;
  description?: string;
  lane: SessionCommandLane;
  /** Daemon source scope (session/workspace/agent/global); shown for skills. */
  scope?: string;
}

export interface SessionComposerCommandSection {
  id: string;
  label: string;
  commands: readonly SessionComposerCommand[];
}

/** Presentation-only projection of the daemon-owned session command catalog. */
export interface SessionComposerCommandCatalog {
  standaloneSections: readonly SessionComposerCommandSection[];
  inlineSkills: readonly SessionComposerCommand[];
}

export interface SessionCommandItemPresentation {
  token: string;
  lane: SessionCommandLane;
  scope?: string;
  sectionId: string;
  sectionLabel: string;
  /** Row title in the menu; skill rows keep their exact kebab identity. */
  menuTitle: string;
}

export interface SessionCommandMenuGroupEntry {
  item: Unstable_TriggerItem;
  /** Index into the flat navigable list assistant-ui keyboard state runs over. */
  flatIndex: number;
}

export interface SessionCommandMenuGroup {
  sectionId: string;
  sectionLabel: string;
  entries: SessionCommandMenuGroupEntry[];
}

const LANES: readonly SessionCommandLane[] = ["builtin", "agent", "skill"];

function isSessionCommandLane(value: unknown): value is SessionCommandLane {
  return typeof value === "string" && (LANES as readonly string[]).includes(value);
}

/** kebab/underscore token → Title Case chip label ("/frontend-qa" → "Frontend Qa"). */
export function formatCommandChipLabel(token: string): string {
  const bare = token.startsWith("/") ? token.slice(1) : token;
  return bare
    .split(/[-_]/u)
    .map(part => (part.length > 0 ? part.charAt(0).toLocaleUpperCase() + part.slice(1) : part))
    .join(" ");
}

/**
 * The trigger item carries chip-facing identity (`id` = canonical token,
 * `label` = chip label) because the Lexical DirectivePlugin builds the inline
 * chip straight from this item; the menu row reads its kebab title from
 * `metadata.menuTitle` instead.
 */
export function asTriggerItem(
  section: Pick<SessionComposerCommandSection, "id" | "label">,
  command: SessionComposerCommand
): Unstable_TriggerItem {
  return {
    id: command.token,
    type: "session-command",
    label: command.lane === "skill" ? formatCommandChipLabel(command.token) : command.label,
    ...(command.description ? { description: command.description } : {}),
    metadata: {
      token: command.token,
      menuTitle: command.label,
      lane: command.lane,
      ...(command.scope ? { scope: command.scope } : {}),
      sectionId: section.id,
      sectionLabel: section.label,
    },
  };
}

export function commandItemPresentation(
  item: Unstable_TriggerItem
): SessionCommandItemPresentation {
  const metadata = item.metadata ?? {};
  const lane = isSessionCommandLane(metadata.lane) ? metadata.lane : "skill";
  return {
    token: typeof metadata.token === "string" ? metadata.token : item.label,
    lane,
    ...(typeof metadata.scope === "string" ? { scope: metadata.scope } : {}),
    sectionId: typeof metadata.sectionId === "string" ? metadata.sectionId : lane,
    sectionLabel: typeof metadata.sectionLabel === "string" ? metadata.sectionLabel : item.label,
    menuTitle: typeof metadata.menuTitle === "string" ? metadata.menuTitle : item.label,
  };
}

function rankCommand(command: SessionComposerCommand, normalizedQuery: string): number | null {
  if (normalizedQuery.length === 0) return 0;
  const token = command.token.toLocaleLowerCase();
  const bareToken = token.startsWith("/") ? token.slice(1) : token;
  const label = command.label.toLocaleLowerCase();
  if (bareToken.startsWith(normalizedQuery) || label.startsWith(normalizedQuery)) return 0;
  if (
    bareToken.includes(normalizedQuery) ||
    label.includes(normalizedQuery) ||
    command.id.toLocaleLowerCase().includes(normalizedQuery)
  ) {
    return 1;
  }
  if (command.description?.toLocaleLowerCase().includes(normalizedQuery) === true) return 2;
  return null;
}

/**
 * Flat search over ordered sections: prefix matches beat substring matches
 * inside a section, and sections never interleave, so the rendered grouping
 * stays contiguous while assistant-ui navigates one flat list.
 */
export function searchCommandSections(
  sections: readonly SessionComposerCommandSection[],
  query: string
): Unstable_TriggerItem[] {
  const normalizedQuery = query.trim().toLocaleLowerCase();
  const results: Unstable_TriggerItem[] = [];
  for (const section of sections) {
    const ranked: { command: SessionComposerCommand; rank: number }[] = [];
    for (const command of section.commands) {
      const rank = rankCommand(command, normalizedQuery);
      if (rank !== null) ranked.push({ command, rank });
    }
    ranked.sort((a, b) => a.rank - b.rank);
    for (const entry of ranked) results.push(asTriggerItem(section, entry.command));
  }
  return results;
}

/** Splits the flat navigable list into contiguous section runs for rendering. */
export function groupCommandItems(
  items: readonly Unstable_TriggerItem[]
): SessionCommandMenuGroup[] {
  const groups: SessionCommandMenuGroup[] = [];
  items.forEach((item, flatIndex) => {
    const presentation = commandItemPresentation(item);
    const current = groups.at(-1);
    if (current && current.sectionId === presentation.sectionId) {
      current.entries.push({ item, flatIndex });
      return;
    }
    groups.push({
      sectionId: presentation.sectionId,
      sectionLabel: presentation.sectionLabel,
      entries: [{ item, flatIndex }],
    });
  });
  return groups;
}

const COMMAND_ICONS: Record<string, LucideIcon> = {
  goal: Target,
  clear: Eraser,
  compact: Minimize2,
  model: Brain,
  plan: ListTodo,
  review: SearchCode,
  status: Info,
  help: CircleHelp,
  resume: Play,
  fork: GitFork,
  fast: Zap,
  config: Settings2,
};

const LANE_ICONS: Record<SessionCommandLane, LucideIcon> = {
  builtin: SquareSlash,
  agent: Bot,
  skill: Blocks,
};

export function commandIcon(token: string, lane: SessionCommandLane): LucideIcon {
  const name = token.startsWith("/") ? token.slice(1) : token;
  return COMMAND_ICONS[name] ?? LANE_ICONS[lane];
}

export function formatScopeLabel(scope: string | undefined): string {
  const normalized = scope?.trim() ?? "";
  if (normalized.length === 0) return "Skill";
  return normalized.charAt(0).toLocaleUpperCase() + normalized.slice(1);
}

export interface SessionCommandTrailing {
  text: string;
  mono: boolean;
}

/** Built-in/agent rows show the canonical token; skill rows show their daemon scope. */
export function commandTrailing(
  presentation: SessionCommandItemPresentation
): SessionCommandTrailing {
  if (presentation.lane === "skill") {
    return { text: formatScopeLabel(presentation.scope), mono: false };
  }
  return { text: presentation.token, mono: true };
}
