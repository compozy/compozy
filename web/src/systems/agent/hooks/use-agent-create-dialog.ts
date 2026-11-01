import { useEffect } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useSelector, useStore } from "@xstate/store-react";

import { PROFILE_IDENTITY_SWATCHES, useProfileReadScope } from "@/systems/profiles";

import { useAgents, useCreateAgent, useDuplicateAgent } from "./use-agents";
import { useAgentInstances, useCreateAgentInstance } from "./use-agent-instances";
import { agentCreateDialogLogic, type AgentCreateOutcome } from "./agent-create-dialog-logic";
import { agentInstanceLimitReached } from "../lib/agent-instance";
import {
  AGENT_PLACE_LATER,
  buildCreateAgentParams,
  buildDraftFromAgentPayload,
  buildDuplicateAgentParams,
  createDefaultAgentCreateDraft,
  updateAgentCreateScope,
  validateAgentCreateDraft,
  type AgentCreateDialogDraft,
} from "../lib/agent-create-draft";
import type { AgentPayload } from "../types";
import { type RuntimeCatalogProvider, useRuntimeModelCatalog } from "@/systems/model-catalog";
import type { RuntimeModelOption, RuntimeProviderOption } from "@/systems/runtime";
import { settingsProviderToOption, useSettingsProviders } from "@/systems/settings";
import {
  useWorkspaces,
  workspaceProviderToOption,
  type SessionProviderOption,
  type WorkspacePayload,
} from "@/systems/workspace";

interface AgentCreateDialogContext {
  activeWorkspace: WorkspacePayload | undefined;
  workspaceProviders: SessionProviderOption[];
  workspaceProvidersError: string | null;
  workspaceProvidersLoading: boolean;
}

export interface AgentCreateDialogState {
  open: boolean;
  draft: AgentCreateDialogDraft;
  providerOptions: RuntimeProviderOption[];
  providersLoading: boolean;
  providersError: string | null;
  runtimeModels: RuntimeModelOption[];
  modelCatalogLoading: boolean;
  modelCatalogRefreshing: boolean;
  modelCatalogError: string | null;
  submitError: string | null;
  isSubmitting: boolean;
  hasActiveWorkspace: boolean;
  workspaceId: string | null;
  workspaceName: string | null;
  mode: "create" | "duplicate";
  duplicateSourceName: string | null;
  /** Slugs this scope already holds, so a conflict shows before submit. */
  takenNames: ReadonlySet<string>;
  /** Every project, for Where it works. */
  workspaces: readonly WorkspacePayload[];
  atInstanceLimit: boolean;
  /** The agent was created but couldn't start: the screen reports it instead of closing. */
  startFailure: string | null;
}

export interface AgentCreateOpenOptions {
  /** Opens on Stays on with this place chosen: a workspace id or `@user`. */
  place?: string;
}

export interface AgentCreateDialogApi extends AgentCreateDialogState {
  openDialog: (options?: AgentCreateOpenOptions) => void;
  openForDuplicate: (agent: AgentPayload) => void;
  onDraftChange: (draft: AgentCreateDialogDraft) => void;
  onOpenChange: (open: boolean) => void;
  onRefreshCatalog: () => void;
  onOpenProviderSettings: () => void;
  onSubmit: () => void;
  onOpenCreatedAgent: () => void;
}

const NO_NAMES: ReadonlySet<string> = new Set();

function describeError(fallback: string, error: unknown): string {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return fallback;
}

export function useAgentCreateDialog({
  activeWorkspace,
  workspaceProviders,
  workspaceProvidersError,
  workspaceProvidersLoading,
}: AgentCreateDialogContext): AgentCreateDialogApi {
  const navigate = useNavigate();
  const { destination } = useProfileReadScope();
  const { mutateAsync: createAgent } = useCreateAgent();
  const { mutateAsync: duplicateAgent } = useDuplicateAgent();
  const { mutateAsync: createInstance } = useCreateAgentInstance();
  const instances = useAgentInstances();
  const workspaces = useWorkspaces();
  const dialogStore = useStore(agentCreateDialogLogic, {
    initialDraft: createDefaultAgentCreateDraft(Boolean(activeWorkspace), activeWorkspace?.id),
  });
  const flow = useSelector(dialogStore, snapshot => snapshot.context);

  const { draft, providerOptions, providersLoading, providersError } = useAgentCreateProviders(
    flow.draft,
    { activeWorkspace, workspaceProviders, workspaceProvidersError, workspaceProvidersLoading }
  );

  const catalogProviders: RuntimeCatalogProvider[] = providerOptions.map(option => ({
    id: option.id,
    needsAuth: option.needs_auth,
  }));
  const open =
    flow.status === "editing" ||
    flow.status === "failed" ||
    flow.status === "submitting" ||
    flow.status === "start-failed";
  const catalog = useRuntimeModelCatalog(catalogProviders, { enabled: open });
  const runtimeModels = catalog.models;
  const definitions = useAgents(activeWorkspace?.id, { enabled: open });
  const takenNames = definitions.data ? new Set(definitions.data.map(a => a.name)) : NO_NAMES;
  const atInstanceLimit = agentInstanceLimitReached(instances);

  const validationContext = {
    hasActiveWorkspace: Boolean(activeWorkspace),
    providerOptions,
    providersError,
    providersLoading,
  };

  const defaultDraft = () =>
    createDefaultAgentCreateDraft(Boolean(activeWorkspace), activeWorkspace?.id);
  const navigationTarget = flow.status === "navigation-pending" ? flow : null;

  useEffect(() => {
    if (!navigationTarget) return;
    const { agentName, attempt, landing } = navigationTarget;
    dialogStore.trigger.navigationRequested({
      attempt,
      draft: createDefaultAgentCreateDraft(Boolean(activeWorkspace), activeWorkspace?.id),
      execute: async () => {
        if (landing === "sessions") await navigate({ to: "/sessions" });
        else await navigate({ to: "/agents/$name", params: { name: agentName } });
      },
    });
  }, [activeWorkspace, dialogStore, navigate, navigationTarget]);

  const openDialog = (options: AgentCreateOpenOptions = {}) => {
    const draft = defaultDraft();
    dialogStore.trigger.createOpened({
      draft: options.place ? { ...draft, kind: "stays_on", place: options.place } : draft,
    });
  };

  /** Starts the first instance, unless Decide later or a shown block says it can't start. */
  const startFirstInstance = async (agent: AgentPayload): Promise<AgentCreateOutcome> => {
    const provider = providerOptions.find(option => option.id === draft.provider);
    const skipped =
      draft.kind !== "stays_on" ||
      draft.place === AGENT_PLACE_LATER ||
      atInstanceLimit ||
      provider?.needs_auth === true;
    if (skipped) return { agent, landing: "agent" };
    try {
      await createInstance({ agent: agent.name, workspace: draft.place });
      return { agent, landing: "sessions" };
    } catch (cause) {
      return {
        agent,
        landing: "agent",
        startError: describeError("This agent couldn't start.", cause),
      };
    }
  };

  const openForDuplicate = (agent: AgentPayload) => {
    dialogStore.trigger.duplicateOpened({
      draft: buildDraftFromAgentPayload(agent, Boolean(activeWorkspace)),
      source: agent,
    });
  };

  const onOpenChange = (next: boolean) => {
    if (next) {
      if (flow.status === "closed") {
        openDialog();
      }
      return;
    }
    dialogStore.trigger.dialogDismissed({ draft: defaultDraft() });
  };

  const onDraftChange = (nextDraft: AgentCreateDialogDraft) => {
    dialogStore.trigger.draftChanged({ draft: nextDraft });
  };

  const onRefreshCatalog = catalog.refresh;

  const onOpenProviderSettings = () => {
    dialogStore.trigger.dialogDismissed({ draft: defaultDraft() });
    void navigate({ to: "/settings/providers" });
  };

  const onSubmit = () => {
    if (flow.status !== "editing" && flow.status !== "failed") {
      return;
    }
    let execute: () => Promise<AgentCreateOutcome>;
    let failureMessage: string;
    if (flow.kind === "duplicate") {
      const request = buildDuplicateAgentParams(
        flow.source,
        draft,
        activeWorkspace?.id,
        validationContext
      );
      if (!request) {
        const validation = validateAgentCreateDraft(draft, validationContext);
        const message =
          Object.values(validation.fields).find(field => field && field.length > 0) ??
          "Fix the highlighted fields before duplicating an agent.";
        dialogStore.trigger.submissionRejected({ error: message });
        return;
      }
      execute = async () =>
        startFirstInstance(
          await duplicateAgent({
            sourceName: flow.source.name,
            params: request,
            profile: destination,
          })
        );
      failureMessage = "Failed to duplicate agent.";
    } else {
      const request = buildCreateAgentParams(
        draft,
        activeWorkspace?.id,
        validationContext,
        PROFILE_IDENTITY_SWATCHES.map(swatch => swatch.value)
      );
      if (!request) {
        const validation = validateAgentCreateDraft(draft, validationContext);
        const message =
          Object.values(validation.fields).find(field => field && field.length > 0) ??
          "Fix the highlighted fields before creating an agent.";
        dialogStore.trigger.submissionRejected({ error: message });
        return;
      }
      execute = async () =>
        startFirstInstance(await createAgent({ params: request, profile: destination }));
      failureMessage = "Failed to create agent.";
    }

    dialogStore.trigger.submissionRequested({
      execute,
      failureMessage,
    });
  };

  return {
    open,
    draft,
    providerOptions,
    providersLoading,
    providersError,
    runtimeModels,
    modelCatalogLoading: catalog.loading,
    modelCatalogRefreshing: catalog.refreshing,
    modelCatalogError: catalog.error,
    submitError: flow.status === "failed" ? flow.error : null,
    isSubmitting: flow.status === "submitting",
    takenNames,
    workspaces: workspaces.data ?? [],
    atInstanceLimit,
    startFailure: flow.status === "start-failed" ? flow.error : null,
    hasActiveWorkspace: Boolean(activeWorkspace),
    workspaceId: activeWorkspace?.id ?? null,
    workspaceName: activeWorkspace?.name ?? null,
    mode: flow.status === "closed" ? "create" : flow.kind,
    duplicateSourceName:
      flow.status !== "closed" && flow.kind === "duplicate" ? flow.source.name : null,
    openDialog,
    openForDuplicate,
    onDraftChange,
    onOpenChange,
    onRefreshCatalog,
    onOpenProviderSettings,
    onSubmit,
    onOpenCreatedAgent: () => dialogStore.trigger.startFailureAcknowledged({}),
  };
}

function useAgentCreateProviders(
  storedDraft: AgentCreateDialogDraft,
  {
    activeWorkspace,
    workspaceProviders,
    workspaceProvidersError,
    workspaceProvidersLoading,
  }: AgentCreateDialogContext
) {
  const settingsProviders = useSettingsProviders();
  const globalProviderEntries = settingsProviders.data?.providers;
  const globalProviders: RuntimeProviderOption[] =
    globalProviderEntries?.map(settingsProviderToOption) ?? [];

  const workspaceProviderOptions: RuntimeProviderOption[] =
    workspaceProviders.map(workspaceProviderToOption);

  // The menubar owns an On demand agent's scope. A Stays on agent lives at the user level, so any
  // project or Global can start it.
  const scopedDraft: AgentCreateDialogDraft = updateAgentCreateScope(
    storedDraft,
    activeWorkspace && storedDraft.kind === "on_demand" ? "workspace" : "global"
  );
  const sourceProviders =
    scopedDraft.scope === "workspace" ? workspaceProviders : (globalProviderEntries ?? []);
  const draft: AgentCreateDialogDraft =
    scopedDraft.provider.length > 0 &&
    !sourceProviders.some(provider => provider.name === scopedDraft.provider)
      ? { ...scopedDraft, provider: "", model: "", reasoningEffort: "" as const }
      : scopedDraft;
  const providerOptions: RuntimeProviderOption[] =
    draft.scope === "workspace" ? workspaceProviderOptions : globalProviders;

  const providersLoading =
    draft.scope === "workspace" ? workspaceProvidersLoading : settingsProviders.isLoading;
  const providersError =
    draft.scope === "workspace"
      ? workspaceProvidersError
      : settingsProviders.error
        ? describeError("Unable to load global provider settings.", settingsProviders.error)
        : null;

  return { draft, providerOptions, providersLoading, providersError };
}
