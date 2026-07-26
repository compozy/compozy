import { useState, type SetStateAction } from "react";

import { useSettingsPage } from "./use-settings-page";
import { useAgents, type AgentPayload } from "@/systems/agent";
import {
  SettingsApiError,
  useSettingsSkills,
  useUpdateSettingsSkills,
  type SettingsSkillsFilter,
  type SettingsSkillsSection,
  type SettingsUpdateSkillsRequest,
} from "@/systems/settings";
import { useWorkspaces, type WorkspacePayload } from "@/systems/workspace";

type SkillsConfig = SettingsSkillsSection["config"];

export type SkillsScopeSelection =
  | { scope: "global" }
  | { scope: "agent"; agentName: string; workspaceId?: string };

function cloneDisabled(config: SkillsConfig): string[] {
  return [...(config.disabled_skills ?? [])];
}

function sameDisabled(a: string[] | undefined, b: string[] | undefined): boolean {
  const left = a ?? [];
  const right = b ?? [];
  if (left.length !== right.length) return false;
  for (let i = 0; i < left.length; i += 1) {
    if (left[i] !== right[i]) return false;
  }
  return true;
}

function samePolicy(a: SkillsConfig, b: SkillsConfig): boolean {
  if (a.enabled !== b.enabled) return false;
  if (a.poll_interval !== b.poll_interval) return false;
  if (a.marketplace.registry !== b.marketplace.registry) return false;
  if ((a.marketplace.base_url ?? "") !== (b.marketplace.base_url ?? "")) return false;
  if (
    JSON.stringify(a.allowed_marketplace_hooks ?? []) !==
    JSON.stringify(b.allowed_marketplace_hooks ?? [])
  ) {
    return false;
  }
  if (
    JSON.stringify(a.allowed_marketplace_mcp ?? []) !==
    JSON.stringify(b.allowed_marketplace_mcp ?? [])
  ) {
    return false;
  }
  return true;
}

function errorMessage(error: unknown): string | null {
  if (error instanceof SettingsApiError) return error.message;
  if (error instanceof Error) return error.message;
  return null;
}

function selectionToFilter(selection: SkillsScopeSelection): SettingsSkillsFilter {
  if (selection.scope === "agent") {
    return {
      scope: "agent",
      agent_name: selection.agentName,
      workspace_id: selection.workspaceId,
    };
  }

  return { scope: "global" };
}

function envelopeScopeKey(envelope: SettingsSkillsSection): string {
  return [envelope.scope, envelope.agent_name ?? "", envelope.workspace_id ?? ""].join(":");
}

function sortAgents(agents: AgentPayload[]): AgentPayload[] {
  return [...agents].sort((left, right) => left.name.localeCompare(right.name));
}

function pickDefaultAgentName(agents: AgentPayload[]): string {
  return agents[0]?.name ?? "";
}

export function useSettingsSkillsPage() {
  const page = useSettingsPage({ currentSlug: "skills" });
  const agentsQuery = useAgents();
  const workspaceQuery = useWorkspaces();
  const disabledMutation = useUpdateSettingsSkills();
  const policyMutation = useUpdateSettingsSkills();

  const agents = sortAgents(agentsQuery.data ?? []);
  const workspaces: WorkspacePayload[] = workspaceQuery.data ?? [];

  const [storedSelection, setSelection] = useState<SkillsScopeSelection>({ scope: "global" });
  const selection: SkillsScopeSelection =
    storedSelection.scope !== "agent"
      ? storedSelection
      : agents.length === 0
        ? { scope: "global" }
        : agents.some(agent => agent.name === storedSelection.agentName)
          ? storedSelection
          : {
              scope: "agent",
              agentName: pickDefaultAgentName(agents),
              workspaceId: storedSelection.workspaceId,
            };
  const filter = selectionToFilter(selection);
  const query = useSettingsSkills(filter);
  const envelope = query.data ?? null;

  const envelopeKey = envelope ? envelopeScopeKey(envelope) : "pending";
  const [draftState, setDraftState] = useState<{ draft: SkillsConfig | null; key: string }>({
    draft: null,
    key: envelopeKey,
  });
  const draft =
    draftState.key === envelopeKey
      ? (draftState.draft ?? envelope?.config ?? null)
      : (envelope?.config ?? null);
  const setDraft = (update: SetStateAction<SkillsConfig | null>) => {
    setDraftState(current => {
      const currentDraft =
        current.key === envelopeKey
          ? (current.draft ?? envelope?.config ?? null)
          : (envelope?.config ?? null);
      return {
        draft: typeof update === "function" ? update(currentDraft) : update,
        key: envelopeKey,
      };
    });
  };
  const [lastDisabledState, setLastDisabledState] = useState({
    key: envelopeKey,
    label: null as string | null,
  });
  const [lastPolicyState, setLastPolicyState] = useState({
    key: envelopeKey,
    label: null as string | null,
  });
  const lastDisabledLabel = lastDisabledState.key === envelopeKey ? lastDisabledState.label : null;
  const lastPolicyLabel = lastPolicyState.key === envelopeKey ? lastPolicyState.label : null;
  const setLastDisabledLabel = (label: string | null) =>
    setLastDisabledState({ key: envelopeKey, label });
  const setLastPolicyLabel = (label: string | null) =>
    setLastPolicyState({ key: envelopeKey, label });

  const availableScopes = envelope?.available_scopes ?? ["global"];
  const selectedAgent =
    selection.scope === "agent"
      ? (agents.find(agent => agent.name === selection.agentName) ?? null)
      : null;
  const selectedWorkspaceContext =
    selection.scope === "agent" && selection.workspaceId
      ? (workspaces.find(workspace => workspace.id === selection.workspaceId) ?? null)
      : null;

  const isDisabledDirty =
    envelope && draft
      ? !sameDisabled(envelope.config.disabled_skills, draft.disabled_skills)
      : false;

  const isPolicyDirty =
    envelope && draft && selection.scope !== "agent" ? !samePolicy(envelope.config, draft) : false;

  const resetScopedState = () => {
    disabledMutation.reset();
    policyMutation.reset();
    setLastDisabledLabel(null);
    setLastPolicyLabel(null);
  };

  const handleResetDisabled = () => {
    if (!envelope || !draft) return;
    setDraft({ ...draft, disabled_skills: cloneDisabled(envelope.config) });
  };

  const handleResetPolicy = () => {
    if (!envelope || !draft || selection.scope === "agent") return;
    setDraft({
      ...envelope.config,
      disabled_skills: draft.disabled_skills,
    });
  };

  const handleSaveDisabled = () => {
    if (!envelope || !draft) return;
    const body: SettingsUpdateSkillsRequest = {
      config: {
        ...envelope.config,
        disabled_skills: draft.disabled_skills ?? [],
      },
    };
    disabledMutation.mutate(
      { body, filter },
      {
        onSuccess: () => {
          setLastDisabledLabel("Saved · applied immediately");
        },
      }
    );
  };

  const handleSavePolicy = () => {
    if (!envelope || !draft || selection.scope === "agent") return;
    const body: SettingsUpdateSkillsRequest = {
      config: {
        ...draft,
        disabled_skills: envelope.config.disabled_skills ?? [],
      },
    };
    policyMutation.mutate(
      { body, filter },
      {
        onSuccess: () => {
          setLastPolicyLabel("Saved · restart required to apply");
        },
      }
    );
  };

  const toggleDisabled = (name: string) => {
    if (!draft) return;
    const current = draft.disabled_skills ?? [];
    const next = current.includes(name)
      ? current.filter(entry => entry !== name)
      : [...current, name].sort();
    setDraft({ ...draft, disabled_skills: next });
  };

  const handleRetry = () => {
    void query.refetch();
  };

  const selectGlobal = () => {
    resetScopedState();
    setSelection({ scope: "global" });
  };

  const selectAgentScope = () => {
    if (agents.length === 0) {
      return;
    }
    resetScopedState();
    setSelection(current => ({
      scope: "agent",
      agentName:
        current.scope === "agent" && current.agentName.trim().length > 0
          ? current.agentName
          : pickDefaultAgentName(agents),
      workspaceId: current.scope === "agent" ? current.workspaceId : undefined,
    }));
  };

  const selectAgent = (agentName: string) => {
    if (agentName.trim().length === 0) {
      return;
    }
    resetScopedState();
    setSelection(current => ({
      scope: "agent",
      agentName,
      workspaceId: current.scope === "agent" ? current.workspaceId : undefined,
    }));
  };

  const selectWorkspaceContext = (workspaceId: string) => {
    resetScopedState();
    setSelection(current => {
      if (current.scope !== "agent") {
        return current;
      }
      return {
        ...current,
        workspaceId: workspaceId.trim().length > 0 ? workspaceId : undefined,
      };
    });
  };

  return {
    isLoading: query.isLoading || (selection.scope === "agent" && agentsQuery.isLoading),
    error: query.error,
    envelope,
    draft,
    setDraft,
    toggleDisabled,
    isDisabledDirty,
    isPolicyDirty,
    handleResetDisabled,
    handleResetPolicy,
    handleSaveDisabled,
    handleSavePolicy,
    isSavingDisabled: disabledMutation.isPending,
    isSavingPolicy: policyMutation.isPending,
    saveDisabledError: errorMessage(disabledMutation.error),
    savePolicyError: errorMessage(policyMutation.error),
    disabledWarnings: disabledMutation.data?.warnings,
    policyWarnings: policyMutation.data?.warnings,
    lastDisabledLabel,
    lastPolicyLabel,
    handleRetry,
    restart: page.restart,
    availableScopes,
    selection,
    agents,
    workspaces,
    selectedAgent,
    selectedWorkspaceContext,
    selectGlobal,
    selectAgentScope,
    selectAgent,
    selectWorkspaceContext,
  };
}
