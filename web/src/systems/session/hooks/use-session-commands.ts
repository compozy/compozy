import { useQuery } from "@tanstack/react-query";

import type { SessionCommandPayload } from "../types";
import { sessionCommandsOptions } from "../lib/query-options";

export type SessionCommandMenuLane = "builtin" | "agent" | "skill";

export interface SessionCommandMenuItem {
  id: string;
  token: string;
  label: string;
  description?: string;
  lane: SessionCommandMenuLane;
  /** Daemon-reported source tier, surfaced for skill rows. */
  scope?: string;
  /**
   * Which folder convention contributed this skill, straight from the daemon
   * catalog. Empty for CompozyOS-native skills, which carry no label.
   */
  origin?: string;
  /**
   * Daemon availability for this session's current state. An unavailable command
   * stays listed and inert — hiding it would erase the reason it cannot run.
   */
  available: boolean;
  /** Verbatim daemon explanation for `available: false`. Never reworded locally. */
  unavailableReason?: string;
}

export interface SessionCommandMenuSection {
  id: string;
  label: string;
  commands: readonly SessionCommandMenuItem[];
}

export interface SessionCommandMenuCatalog {
  standaloneSections: readonly SessionCommandMenuSection[];
  inlineSkills: readonly SessionCommandMenuItem[];
}

const COMMAND_SECTIONS = [
  { id: "builtin", label: "Built-in" },
  { id: "agent", label: "Agent" },
  { id: "skill", label: "Skills" },
] as const;

/**
 * Built-in and agent display names humanize to Title Case. Skill rows use their
 * canonical token so a namespaced collision stays distinguishable from its bare winner.
 */
function humanizeCommandLabel(displayName: string, lane: SessionCommandMenuLane): string {
  const bare = displayName.startsWith("/") ? displayName.slice(1) : displayName;
  if (lane === "skill") return bare;
  return bare
    .split(/[-_]/u)
    .map(part => (part.length > 0 ? part.charAt(0).toLocaleUpperCase() + part.slice(1) : part))
    .join(" ");
}

function commandMenuLane(lane: string): SessionCommandMenuLane | null {
  return lane === "builtin" || lane === "agent" || lane === "skill" ? lane : null;
}

function commandMenuItem(
  command: SessionCommandPayload,
  lane: SessionCommandMenuLane
): SessionCommandMenuItem {
  const scope = command.source?.scope;
  const origin = command.source?.origin?.trim();
  const reason = command.unavailable_reason?.trim();
  const labelSource = lane === "skill" ? command.canonical_token : command.display_name;
  return {
    id: command.id,
    token: command.canonical_token,
    label: humanizeCommandLabel(labelSource, lane),
    lane,
    available: command.available,
    ...(command.description ? { description: command.description } : {}),
    ...(scope ? { scope } : {}),
    ...(origin ? { origin } : {}),
    ...(reason ? { unavailableReason: reason } : {}),
  };
}

/** Finds one command by its canonical token ("/worktree") across every lane. */
export function findSessionCommand(
  catalog: SessionCommandMenuCatalog | undefined,
  token: string
): SessionCommandMenuItem | undefined {
  if (!catalog) return undefined;
  for (const section of catalog.standaloneSections) {
    const found = section.commands.find(command => command.token === token);
    if (found) return found;
  }
  return catalog.inlineSkills.find(command => command.token === token);
}

/** Projects the public daemon catalog into the assistant-ui menu contract without local policy. */
export function sessionCommandMenuCatalog(
  commands: readonly SessionCommandPayload[]
): SessionCommandMenuCatalog {
  const standaloneByLane: Record<
    (typeof COMMAND_SECTIONS)[number]["id"],
    SessionCommandMenuItem[]
  > = {
    builtin: [],
    agent: [],
    skill: [],
  };
  const inlineSkills: SessionCommandMenuItem[] = [];

  for (const command of commands) {
    const lane = commandMenuLane(command.lane);
    if (lane === null) continue;
    const isStandalone = command.placements.includes("standalone");
    const isInlineSkill = lane === "skill" && command.placements.includes("inline");
    if (!isStandalone && !isInlineSkill) continue;

    const item = commandMenuItem(command, lane);
    if (isStandalone) {
      standaloneByLane[lane].push(item);
    }
    if (isInlineSkill) {
      inlineSkills.push(item);
    }
  }

  const standaloneSections: SessionCommandMenuSection[] = [];
  for (const section of COMMAND_SECTIONS) {
    const sectionCommands = standaloneByLane[section.id];
    if (sectionCommands.length > 0) {
      standaloneSections.push({ ...section, commands: sectionCommands });
    }
  }
  return { standaloneSections, inlineSkills };
}

export function useSessionCommands(
  workspaceId: string,
  sessionId: string,
  options: { enabled?: boolean } = {}
) {
  const query = useQuery({
    ...sessionCommandsOptions(workspaceId, sessionId, options.enabled),
    select: response => sessionCommandMenuCatalog(response.commands),
  });
  return {
    ...query,
    catalog: query.data,
  };
}
