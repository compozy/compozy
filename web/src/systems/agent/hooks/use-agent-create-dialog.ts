import { useEffect } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useSelector, useStore } from "@xstate/store-react";

import { useAgents } from "./use-agents";
import { useAgentInstanceCapacity } from "./use-agent-instances";
import {
  agentCreateDialogLogic,
  errorMessage,
  type AgentCreateDialogStore,
  type AgentCreateFlow,
} from "./agent-create-dialog-logic";
import { useAgentCreateSubmit } from "./use-agent-create-submit";
import type { AgentInstanceCapacity } from "../lib/agent-instance";
import {
  buildDraftFromAgentPayload,
  createDefaultAgentCreateDraft,
  updateAgentCreateScope,
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
  /** The draft a duplicate opened with; closing asks only once it changes. */
  entrySeed: AgentCreateDialogDraft | null;
  /** Slugs this scope already holds, so a conflict shows before submit. */
  takenNames: ReadonlySet<string>;
  /** Every project, for Where it works. */
  workspaces: readonly WorkspacePayload[];
  instanceCapacity: AgentInstanceCapacity;
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

/** What the flow's status says about the screen, whatever the draft holds. */
function describeAgentCreateFlow(flow: AgentCreateFlow) {
  const open =
    flow.status === "editing" ||
    flow.status === "failed" ||
    flow.status === "submitting" ||
    flow.status === "start-failed";
  const duplicateSource =
    flow.status !== "closed" && flow.kind === "duplicate" ? flow.source : null;
  return {
    open,
    duplicateSource,
    mode: duplicateSource ? ("duplicate" as const) : ("create" as const),
    submitError: flow.status === "failed" ? flow.error : null,
    startFailure: flow.status === "start-failed" ? flow.error : null,
    isSubmitting: flow.status === "submitting",
  };
}

/** Leaves the dialog for the created agent's landing once the flow asks for it. */
function useAgentCreateNavigation(
  dialogStore: AgentCreateDialogStore,
  flow: AgentCreateFlow,
  activeWorkspace: WorkspacePayload | undefined
) {
  const navigate = useNavigate();
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
}

export function useAgentCreateDialog({
  activeWorkspace,
  workspaceProviders,
  workspaceProvidersError,
  workspaceProvidersLoading,
}: AgentCreateDialogContext): AgentCreateDialogApi {
  const navigate = useNavigate();
  const instanceCapacity = useAgentInstanceCapacity();
  const workspaces = useWorkspaces();
  const dialogStore = useStore(agentCreateDialogLogic, {
    initialDraft: createDefaultAgentCreateDraft(Boolean(activeWorkspace), activeWorkspace?.id),
  });
  const flow = useSelector(dialogStore, snapshot => snapshot.context);
  const { open, duplicateSource, ...flowView } = describeAgentCreateFlow(flow);

  const { resolveDraft, providerOptions, providersLoading, providersError } =
    useAgentCreateProviders(flow.draft, {
      activeWorkspace,
      workspaceProviders,
      workspaceProvidersError,
      workspaceProvidersLoading,
    });
  const draft = resolveDraft(flow.draft);

  const catalogProviders: RuntimeCatalogProvider[] = providerOptions.map(option => ({
    id: option.id,
    needsAuth: option.needs_auth,
  }));
  const catalog = useRuntimeModelCatalog(catalogProviders, { enabled: open });
  const definitions = useAgents(activeWorkspace?.id, { enabled: open });
  const takenNames = definitions.data ? new Set(definitions.data.map(a => a.name)) : NO_NAMES;

  const defaultDraft = () =>
    createDefaultAgentCreateDraft(Boolean(activeWorkspace), activeWorkspace?.id);

  useAgentCreateNavigation(dialogStore, flow, activeWorkspace);
  const onSubmit = useAgentCreateSubmit({
    activeWorkspaceId: activeWorkspace?.id,
    dialogStore,
    draft,
    flow,
    instanceCapacity,
    providerOptions,
    providersError,
    providersLoading,
  });

  const openDialog = (options: AgentCreateOpenOptions = {}) => {
    const draft = defaultDraft();
    dialogStore.trigger.createOpened({
      draft: options.place ? { ...draft, kind: "stays_on", place: options.place } : draft,
    });
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

  const onOpenProviderSettings = () => {
    dialogStore.trigger.dialogDismissed({ draft: defaultDraft() });
    void navigate({ to: "/settings/providers" });
  };

  return {
    ...flowView,
    open,
    draft,
    providerOptions,
    providersLoading,
    providersError,
    runtimeModels: catalog.models,
    modelCatalogLoading: catalog.loading,
    modelCatalogRefreshing: catalog.refreshing,
    modelCatalogError: catalog.error,
    takenNames,
    workspaces: workspaces.data ?? [],
    instanceCapacity,
    hasActiveWorkspace: Boolean(activeWorkspace),
    workspaceId: activeWorkspace?.id ?? null,
    workspaceName: activeWorkspace?.name ?? null,
    duplicateSourceName: duplicateSource?.name ?? null,
    // Resolved like the draft, so a provider this scope lacks doesn't read as an edit.
    entrySeed: duplicateSource
      ? resolveDraft(buildDraftFromAgentPayload(duplicateSource, Boolean(activeWorkspace)))
      : null,
    openDialog,
    openForDuplicate,
    onDraftChange,
    onOpenChange,
    onRefreshCatalog: catalog.refresh,
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

  /** The menubar owns an On demand agent's scope; a Stays on agent lives at the user level. */
  const resolveDraft = (stored: AgentCreateDialogDraft): AgentCreateDialogDraft => {
    const scoped = updateAgentCreateScope(
      stored,
      activeWorkspace && stored.kind === "on_demand" ? "workspace" : "global"
    );
    const sourceProviders =
      scoped.scope === "workspace" ? workspaceProviders : (globalProviderEntries ?? []);
    const providerKnown = sourceProviders.some(provider => provider.name === scoped.provider);
    return scoped.provider.length > 0 && !providerKnown
      ? { ...scoped, provider: "", model: "", reasoningEffort: "" as const }
      : scoped;
  };
  const { scope } = resolveDraft(storedDraft);
  const providerOptions: RuntimeProviderOption[] =
    scope === "workspace" ? workspaceProviderOptions : globalProviders;

  const providersLoading =
    scope === "workspace" ? workspaceProvidersLoading : settingsProviders.isLoading;
  const providersError =
    scope === "workspace"
      ? workspaceProvidersError
      : settingsProviders.error
        ? errorMessage(settingsProviders.error, "Unable to load global provider settings.")
        : null;

  return { resolveDraft, providerOptions, providersLoading, providersError };
}
