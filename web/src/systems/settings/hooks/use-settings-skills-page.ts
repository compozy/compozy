import { useState, type SetStateAction } from "react";
import { useSelector } from "@xstate/store-react";

import { useStoreBinding } from "@/hooks/use-store-binding";

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
import { settingsSkillsDraftLogic, shouldRebindSkillsDraft } from "./settings-skills-draft-logic";

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
  const baseline = envelope?.config ?? null;
  const { store: draftLogic } = useStoreBinding(
    envelopeKey,
    () =>
      settingsSkillsDraftLogic.createStore({
        baseline,
        key: envelopeKey,
      }),
    previous =>
      settingsSkillsDraftLogic.createStore({
        baseline,
        key: envelopeKey,
        previous: previous.getSnapshot().context,
      }),
    current => shouldRebindSkillsDraft(current.store.getSnapshot().context, baseline, envelopeKey)
  );
  const draftFlow = useSelector(draftLogic, snapshot => snapshot.context);
  const draft = draftFlow.draft;
  const setDraft = (update: SetStateAction<SkillsConfig | null>) => {
    draftLogic.trigger.draftChanged({
      draft: typeof update === "function" ? update(draft) : update,
    });
  };
  const lastDisabledLabel = draftFlow.key === envelopeKey ? draftFlow.labels.disabled : null;
  const lastPolicyLabel = draftFlow.key === envelopeKey ? draftFlow.labels.policy : null;

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
  };

  const handleResetDisabled = () => {
    if (!envelope || !draft) return;
    draftLogic.trigger.draftChanged({
      draft: { ...draft, disabled_skills: cloneDisabled(envelope.config) },
    });
  };

  const handleResetPolicy = () => {
    if (!envelope || !draft || selection.scope === "agent") return;
    draftLogic.trigger.draftChanged({
      draft: {
        ...envelope.config,
        disabled_skills: draft.disabled_skills,
      },
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
    draftLogic.trigger.saveRequested({
      execute: () => disabledMutation.mutateAsync({ body, filter }),
      kind: "disabled",
      label: "Saved · applied immediately",
    });
  };

  const handleSavePolicy = () => {
    if (!envelope || !draft || selection.scope === "agent") return;
    const body: SettingsUpdateSkillsRequest = {
      config: {
        ...draft,
        disabled_skills: envelope.config.disabled_skills ?? [],
      },
    };
    draftLogic.trigger.saveRequested({
      execute: () => policyMutation.mutateAsync({ body, filter }),
      kind: "policy",
      label: "Saved · restart required to apply",
    });
  };

  const toggleDisabled = (name: string) => {
    if (!draft) return;
    const current = draft.disabled_skills ?? [];
    const next = current.includes(name)
      ? current.filter(entry => entry !== name)
      : [...current, name].sort();
    draftLogic.trigger.draftChanged({ draft: { ...draft, disabled_skills: next } });
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
    isSavingDisabled: draftFlow.pending.disabled !== null,
    isSavingPolicy: draftFlow.pending.policy !== null,
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
