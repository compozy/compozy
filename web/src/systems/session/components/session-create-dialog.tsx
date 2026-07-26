import { Play } from "lucide-react";
import { useRef, type FormEvent } from "react";

import {
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogHeader,
  EntityModeToolbar,
  FieldError,
  type EntityMode,
} from "@agh/ui";

import type { AgentPayload } from "@/systems/agent";
import {
  isNetworkParticipationDraftValid,
  type NetworkParticipationDraft,
} from "@/systems/network";
import {
  RuntimeSelector,
  type RuntimeModelOption,
  type RuntimeProviderOption,
  type RuntimeSelectorValue,
} from "@/systems/runtime";
import type { WorkspaceCommandSelectOption, WorkspacePayload } from "@/systems/workspace";

import { validateSessionModelSelection } from "../lib/session-model-selection";
import { SessionCreateAdvancedSection } from "./session-create-advanced-section";
import { SessionCreatePromptComposer } from "./session-create-prompt-composer";
import { SessionCreateSimpleSection } from "./session-create-simple-section";

export interface SessionCreateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  mode: EntityMode;
  onModeChange: (mode: EntityMode) => void;
  agents: AgentPayload[];
  workspace: WorkspacePayload | undefined;
  workspaces: WorkspaceCommandSelectOption[];
  workspaceId: string | null;
  onWorkspaceChange: (workspaceId: string) => void;
  sessionName: string;
  onSessionNameChange: (next: string) => void;
  workspacePath: string;
  onWorkspacePathChange: (next: string) => void;
  selectedAgentName: string;
  runtimeValue: RuntimeSelectorValue;
  runtimeProviders: RuntimeProviderOption[];
  runtimeModels: RuntimeModelOption[];
  catalogStale: boolean;
  catalogLoading: boolean;
  catalogLoaded: boolean;
  catalogError: string | null;
  catalogRefreshing: boolean;
  catalogRefreshError: string | null;
  providersLoading: boolean;
  providersError: string | null;
  hasProviderOptions: boolean;
  networkParticipation: NetworkParticipationDraft;
  promptValue: string;
  onPromptChange: (next: string) => void;
  onAgentChange: (agentName: string) => void;
  onRuntimeChange: (next: RuntimeSelectorValue) => void;
  onNetworkParticipationChange: (next: NetworkParticipationDraft) => void;
  onCatalogRefresh: () => void;
  onOpenProviderSettings: () => void;
  onSubmit: () => void;
  isSubmitting: boolean;
  submitError: string | null;
}

function SessionCreateDialog({
  open,
  onOpenChange,
  mode,
  onModeChange,
  agents,
  workspace,
  workspaces,
  workspaceId,
  onWorkspaceChange,
  sessionName,
  onSessionNameChange,
  workspacePath,
  onWorkspacePathChange,
  selectedAgentName,
  runtimeValue,
  runtimeProviders,
  runtimeModels,
  catalogStale,
  catalogLoading,
  catalogLoaded,
  catalogError,
  catalogRefreshing,
  catalogRefreshError,
  providersLoading,
  providersError,
  hasProviderOptions,
  networkParticipation,
  promptValue,
  onPromptChange,
  onAgentChange,
  onRuntimeChange,
  onNetworkParticipationChange,
  onCatalogRefresh,
  onOpenProviderSettings,
  onSubmit,
  isSubmitting,
  submitError,
}: SessionCreateDialogProps) {
  const promptRef = useRef<HTMLTextAreaElement>(null);
  const trimmedSelectedAgentName = selectedAgentName.trim();
  const workspaceSelected = workspace !== undefined;
  const hasAgents = agents.length > 0;
  const hasSelectedAgent = agents.some(agent => agent.name === trimmedSelectedAgentName);
  const hasSelectedProvider = runtimeProviders.some(option => option.id === runtimeValue.provider);
  const modelSelection = validateSessionModelSelection({
    provider: runtimeValue.provider,
    model: runtimeValue.model,
    models: runtimeModels,
    catalogLoading,
    catalogLoaded,
    catalogError,
  });
  const canSubmit =
    !isSubmitting &&
    !providersLoading &&
    workspaceSelected &&
    hasAgents &&
    hasSelectedAgent &&
    hasProviderOptions &&
    hasSelectedProvider &&
    modelSelection.valid &&
    promptValue.trim().length > 0 &&
    isNetworkParticipationDraftValid(networkParticipation, ["named"]);

  const submitIfAllowed = () => {
    if (!canSubmit) return;
    onSubmit();
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    submitIfAllowed();
  };

  const handleOpenChange = (nextOpen: boolean) => {
    if (isSubmitting && !nextOpen) {
      return;
    }
    onOpenChange(nextOpen);
  };

  return (
    <Dialog onOpenChange={handleOpenChange} open={open}>
      <DialogContent
        className={`grid-rows-[auto_auto_minmax(0,1fr)] text-fg ${dialogShellClass("sm")}`}
        data-testid="session-create-dialog"
        initialFocus={promptRef}
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description={
            workspaceSelected ? (
              <>
                Launch an agent in a workspace.{" "}
                <b className="font-medium text-muted">This starts a run</b> — it does not create or
                edit a definition.
              </>
            ) : (
              "Choose an active workspace before starting a session."
            )
          }
          eyebrow="Operate · Session"
          icon={Play}
          onClose={isSubmitting ? undefined : () => handleOpenChange(false)}
          title="Start session"
        />

        <EntityModeToolbar mode={mode} onModeChange={onModeChange} testIdPrefix="session-create" />

        <form className="contents" onSubmit={handleSubmit}>
          <EntityDialogBody className="flex flex-col">
            <SessionCreateSimpleSection
              agents={agents}
              isSubmitting={isSubmitting}
              onAgentChange={onAgentChange}
              onSessionNameChange={onSessionNameChange}
              onWorkspaceChange={onWorkspaceChange}
              selectedAgentName={selectedAgentName}
              sessionName={sessionName}
              workspaceId={workspaceId}
              workspaceSelected={workspaceSelected}
              workspaces={workspaces}
            />

            {mode === "advanced" ? (
              <SessionCreateAdvancedSection
                isSubmitting={isSubmitting}
                networkParticipation={networkParticipation}
                onNetworkParticipationChange={onNetworkParticipationChange}
                onWorkspacePathChange={onWorkspacePathChange}
                workspacePath={workspacePath}
              />
            ) : null}

            <SessionCreatePromptComposer
              canSubmit={canSubmit}
              disabled={!workspaceSelected || !hasAgents}
              errorMessageId={submitError ? "session-create-submit-error" : undefined}
              inputRef={promptRef}
              isSubmitting={isSubmitting}
              onChange={onPromptChange}
              onSubmitDraft={submitIfAllowed}
              runtimeControl={
                <RuntimeSelector
                  catalogLoaded={catalogLoaded}
                  catalogStatus={
                    <CatalogStatusLine
                      error={catalogError}
                      loading={catalogLoading}
                      optionCount={runtimeModels.length}
                      refreshError={catalogRefreshError}
                      refreshing={catalogRefreshing}
                      stale={catalogStale}
                    />
                  }
                  disabled={
                    !workspaceSelected || providersLoading || !hasProviderOptions || isSubmitting
                  }
                  loading={catalogLoading}
                  models={runtimeModels}
                  onChange={onRuntimeChange}
                  onOpenProviderSettings={onOpenProviderSettings}
                  onRefreshCatalog={onCatalogRefresh}
                  providers={runtimeProviders}
                  refreshing={catalogRefreshing}
                  triggerId="session-create-runtime"
                  triggerTestId="session-create-runtime-select"
                  value={runtimeValue}
                  variant="composer"
                />
              }
              value={promptValue}
            />

            {providersError ? (
              <p
                className="text-form-hint text-danger"
                data-testid="session-create-providers-error"
                role="alert"
              >
                {providersError}
              </p>
            ) : null}
            {workspaceSelected && !providersLoading && !providersError && !hasProviderOptions ? (
              <p
                className="text-form-hint text-warning"
                data-testid="session-create-providers-empty"
              >
                No providers are configured for this workspace.
              </p>
            ) : null}
            {modelSelection.error ? (
              <FieldError
                className="text-form-hint text-danger"
                data-testid="session-create-model-error"
              >
                {modelSelection.error}
              </FieldError>
            ) : null}

            {isSubmitting ? (
              <p
                aria-live="polite"
                className="text-form-hint text-subtle"
                data-testid="session-create-pending-status"
                role="status"
              >
                Starting the session. It opens as soon as AGH durably accepts it; the first message
                is sent when the runtime starts.
              </p>
            ) : null}

            {submitError ? (
              <p
                className="text-form-hint text-danger"
                data-testid="session-create-submit-error"
                id="session-create-submit-error"
                role="alert"
              >
                {submitError}
              </p>
            ) : null}
          </EntityDialogBody>
        </form>
      </DialogContent>
    </Dialog>
  );
}

interface CatalogStatusLineProps {
  loading: boolean;
  refreshing: boolean;
  stale: boolean;
  error: string | null;
  refreshError: string | null;
  optionCount: number;
}

function CatalogStatusLine({
  loading,
  refreshing,
  stale,
  error,
  refreshError,
  optionCount,
}: CatalogStatusLineProps) {
  if (refreshError) {
    return (
      <span className="text-danger" data-testid="session-create-catalog-refresh-error" role="alert">
        {refreshError}
      </span>
    );
  }
  if (error) {
    return (
      <span className="text-danger" data-testid="session-create-catalog-error" role="alert">
        {error}. Refresh the catalog or leave Model blank to use the provider default.
      </span>
    );
  }
  if (refreshing) {
    return (
      <span className="text-subtle" data-testid="session-create-catalog-refreshing">
        Refreshing model catalog…
      </span>
    );
  }
  if (loading) {
    return (
      <span className="text-subtle" data-testid="session-create-catalog-loading">
        Loading provider models…
      </span>
    );
  }
  if (stale) {
    return (
      <span className="text-warning" data-testid="session-create-catalog-stale">
        Some models are stale — refresh to confirm availability.
      </span>
    );
  }
  if (optionCount === 0) {
    return (
      <span className="text-subtle" data-testid="session-create-catalog-empty">
        No catalog models. Leave Model blank to use the provider default.
      </span>
    );
  }
  return null;
}

export { SessionCreateDialog };
