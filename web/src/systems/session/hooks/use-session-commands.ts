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
  /** Daemon source scope (session/workspace/agent/global); surfaced for skills. */
  scope?: string;
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
 * The daemon's display_name is the canonical token ("/goal"); the menu shows a
 * human title instead. Built-in/agent names humanize to Title Case while skill
 * names keep their exact kebab identity.
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
  return {
    id: command.id,
    token: command.canonical_token,
    label: humanizeCommandLabel(command.display_name, lane),
    lane,
    ...(command.description ? { description: command.description } : {}),
    ...(scope ? { scope } : {}),
  };
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

export function useSessionCommands(workspaceId: string, sessionId: string) {
  const query = useQuery({
    ...sessionCommandsOptions(workspaceId, sessionId),
    select: response => sessionCommandMenuCatalog(response.commands),
  });
  return {
    ...query,
    catalog: query.data,
  };
}
