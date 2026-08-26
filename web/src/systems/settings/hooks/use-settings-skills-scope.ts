import { useState } from "react";

import { type AgentPayload, useAgents } from "@/systems/agent";
import { useProfileReadScope } from "@/systems/profiles";
import { useActiveWorkspace, type WorkspacePayload } from "@/systems/workspace";

import type { SettingsSkillsFilter } from "../types";

const DEFAULT_AGENT_NAME = "general";
const DEFAULT_PROFILE = "default";

export type SkillsScopeValue = "user" | "workspace" | "agent";

export type SkillsScopeSelection =
  | { scope: "user" }
  | { scope: "workspace"; workspaceId: string }
  | { scope: "agent"; agentName: string; workspaceId?: string };

export interface SettingsSkillsScopeModel {
  selection: SkillsScopeSelection;
  filter: SettingsSkillsFilter;
  /** Cache/draft identity for the exact layer being edited, profile included. */
  scopeKey: string;
  agents: AgentPayload[];
  workspaces: WorkspacePayload[];
  selectedAgent: AgentPayload | null;
  selectedWorkspace: WorkspacePayload | null;
  /** Acting profile from the shell lens; "default" means the personal layer. */
  actingProfile: string;
  /** Label of the personal lane — the profile's name once one is acting. */
  personalLabel: string;
  isLoading: boolean;
  error: Error | null;
  selectUser: () => void;
  selectWorkspaceScope: () => void;
  selectAgentScope: () => void;
  selectAgent: (agentName: string) => void;
  selectWorkspace: (workspaceId: string) => void;
  refetch: () => void;
}

function sortAgents(agents: AgentPayload[]): AgentPayload[] {
  return [...agents].sort((left, right) => left.name.localeCompare(right.name));
}

function pickDefaultAgentName(agents: AgentPayload[]): string {
  return agents.find(agent => agent.name === DEFAULT_AGENT_NAME)?.name ?? agents[0]?.name ?? "";
}

/**
 * Normalizes a stored selection against what actually exists right now, so a
 * deleted agent or workspace falls back instead of querying a missing owner.
 */
function resolveSelection(
  stored: SkillsScopeSelection,
  agents: AgentPayload[],
  workspaces: WorkspacePayload[],
  fallbackWorkspaceId: string | null
): SkillsScopeSelection {
  if (stored.scope === "workspace") {
    if (workspaces.some(workspace => workspace.id === stored.workspaceId)) return stored;
    const next = fallbackWorkspaceId ?? workspaces[0]?.id ?? "";
    return next === "" ? { scope: "user" } : { scope: "workspace", workspaceId: next };
  }
  if (stored.scope === "agent") {
    if (agents.length === 0) return { scope: "user" };
    const workspaceId = workspaces.some(workspace => workspace.id === stored.workspaceId)
      ? stored.workspaceId
      : undefined;
    if (agents.some(agent => agent.name === stored.agentName)) {
      return {
        scope: "agent",
        agentName: stored.agentName,
        ...(workspaceId ? { workspaceId } : {}),
      };
    }
    return {
      scope: "agent",
      agentName: pickDefaultAgentName(agents),
      ...(workspaceId ? { workspaceId } : {}),
    };
  }
  return stored;
}

/**
 * Which layer the skills page reads and writes.
 *
 * The profile dimension is never a second control: it follows the shell lens the
 * rest of the app already uses, so switching profiles moves this page with it.
 * A named acting profile turns the personal lane into the profile layer.
 */
export function useSettingsSkillsScope(): SettingsSkillsScopeModel {
  const agentsQuery = useAgents();
  const workspace = useActiveWorkspace();
  const { destination } = useProfileReadScope();
  const [stored, setStored] = useState<SkillsScopeSelection>({ scope: "user" });

  const agents = sortAgents(agentsQuery.data ?? []);
  const workspaces: WorkspacePayload[] = workspace.workspaces ?? [];
  const selection = resolveSelection(stored, agents, workspaces, workspace.activeWorkspaceId);
  const actingProfile = destination;
  const profileScope = actingProfile !== DEFAULT_PROFILE;

  const filter: SettingsSkillsFilter =
    selection.scope === "workspace"
      ? profileScope
        ? {
            scope: "profile",
            profile: actingProfile,
            workspace_id: selection.workspaceId,
          }
        : { scope: "workspace", workspace_id: selection.workspaceId }
      : selection.scope === "agent"
        ? {
            scope: "agent",
            agent_name: selection.agentName,
            ...(selection.workspaceId ? { workspace_id: selection.workspaceId } : {}),
          }
        : profileScope
          ? { scope: "profile", profile: actingProfile }
          : { scope: "user" };

  return {
    selection,
    filter,
    scopeKey: [
      filter.scope ?? "user",
      filter.workspace_id ?? "",
      filter.profile ?? "",
      filter.agent_name ?? "",
    ].join(":"),
    agents,
    workspaces,
    selectedAgent:
      selection.scope === "agent"
        ? (agents.find(agent => agent.name === selection.agentName) ?? null)
        : null,
    selectedWorkspace:
      selection.scope === "user"
        ? null
        : (workspaces.find(candidate => candidate.id === selection.workspaceId) ?? null),
    actingProfile,
    personalLabel: profileScope ? actingProfile : "User",
    isLoading: agentsQuery.isLoading || workspace.isLoading,
    error: (agentsQuery.error ?? workspace.error ?? null) as Error | null,
    selectUser: () => setStored({ scope: "user" }),
    selectWorkspaceScope: () => {
      const next = workspace.activeWorkspaceId ?? workspaces[0]?.id ?? "";
      if (next === "") return;
      setStored({ scope: "workspace", workspaceId: next });
    },
    selectAgentScope: () => {
      if (agents.length === 0) return;
      setStored(current => ({
        scope: "agent",
        agentName:
          current.scope === "agent" && current.agentName.trim() !== ""
            ? current.agentName
            : pickDefaultAgentName(agents),
        ...(current.scope === "agent" && current.workspaceId
          ? { workspaceId: current.workspaceId }
          : {}),
      }));
    },
    selectAgent: agentName => {
      if (agentName.trim() === "") return;
      setStored(current => ({
        scope: "agent",
        agentName,
        ...(current.scope === "agent" && current.workspaceId
          ? { workspaceId: current.workspaceId }
          : {}),
      }));
    },
    selectWorkspace: workspaceId => {
      setStored(current => {
        if (current.scope === "workspace") {
          return workspaceId.trim() === "" ? current : { scope: "workspace", workspaceId };
        }
        if (current.scope !== "agent") return current;
        return {
          ...current,
          ...(workspaceId.trim() === "" ? { workspaceId: undefined } : { workspaceId }),
        };
      });
    },
    refetch: () => {
      void agentsQuery.refetch();
      void workspace.refetch();
    },
  };
}
