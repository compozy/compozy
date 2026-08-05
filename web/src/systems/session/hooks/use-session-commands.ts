import { useQuery } from "@tanstack/react-query";

import type { SessionCommandPayload } from "../types";
import { sessionCommandsOptions } from "../lib/query-options";

export interface SessionCommandMenuItem {
  id: string;
  token: string;
  label: string;
  description?: string;
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

function commandMenuItem(command: SessionCommandPayload): SessionCommandMenuItem {
  return {
    id: command.id,
    token: command.canonical_token,
    label: command.display_name,
    ...(command.description ? { description: command.description } : {}),
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
    const isStandalone = command.placements.includes("standalone");
    const isInlineSkill = command.lane === "skill" && command.placements.includes("inline");
    if (!isStandalone && !isInlineSkill) continue;

    const item = commandMenuItem(command);
    if (isStandalone) {
      switch (command.lane) {
        case "builtin":
          standaloneByLane.builtin.push(item);
          break;
        case "agent":
          standaloneByLane.agent.push(item);
          break;
        case "skill":
          standaloneByLane.skill.push(item);
          break;
      }
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
