import { Bot, CircleAlert } from "@compozy/ui/icons";

import {
  Alert,
  AlertDescription,
  AlertTitle,
  ConfirmDialog,
  Dialog,
  DialogContent,
  EntityDialogFooter,
  EntityDialogHeader,
  Eyebrow,
  dialogShellClass,
} from "@compozy/ui";

import type { RuntimeModelOption, RuntimeProviderOption } from "@/systems/runtime";
import {
  WorkspaceScopeStatement,
  destinationLabel,
  type WorkspacePayload,
} from "@/systems/workspace";

import {
  AGENT_PLACE_LATER,
  agentCreateSlug,
  validateAgentCreateDraft,
  type AgentCreateDialogDraft,
} from "../lib/agent-create-draft";
import type { AgentInstanceCapacity } from "../lib/agent-instance";
import { useAgentCreateViewState } from "../hooks/use-agent-create-view-state";
import { AgentCreateAdvanced } from "./agent-create-advanced";
import {
  AgentCreateIdentity,
  AgentCreateIdentityRead,
  AgentCreateInstructions,
  AgentCreateInstructionsRead,
} from "./agent-create-identity";
import { AgentCreatePreview } from "./agent-create-preview";
import { AgentCreateRunFacts, AgentCreateRunSettings } from "./agent-create-run-settings";

export interface AgentCreateScreenProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  draft: AgentCreateDialogDraft;
  onDraftChange: (draft: AgentCreateDialogDraft) => void;
  onSubmit: () => void;
  onRefreshCatalog: () => void;
  onOpenProviderSettings: () => void;
  /** After a start failure, the way to the agent that was created. */
  onOpenCreatedAgent: () => void;
  providerOptions: RuntimeProviderOption[];
  providersLoading: boolean;
  providersError: string | null;
  runtimeModels: RuntimeModelOption[];
  modelCatalogLoading: boolean;
  modelCatalogRefreshing: boolean;
  modelCatalogError: string | null;
  submitError: string | null;
  /** The agent was created but couldn't start; the screen turns read only and reports it. */
  startFailure?: string | null;
  isSubmitting: boolean;
  hasActiveWorkspace: boolean;
  /** Names an On demand agent's destination in the footer. */
  workspaceName: string | null;
  takenNames: ReadonlySet<string>;
  workspaces: readonly WorkspacePayload[];
  instanceCapacity: AgentInstanceCapacity;
  /** Set while duplicating: the source's label heads the screen. */
  duplicateSourceName?: string | null;
}

function outcome(
  draft: AgentCreateDialogDraft,
  blocked: boolean,
  isSubmitting: boolean,
  workspaceName: string | null
) {
  const creating = isSubmitting ? "Creating…" : "Create agent";
  if (draft.kind === "on_demand") {
    const scope = draft.scope === "workspace" ? "project" : "user";
    return {
      primary: creating,
      hint: (
        <WorkspaceScopeStatement
          destination={destinationLabel(scope, workspaceName)}
          kind="create"
          scope={scope}
          variant="note"
        />
      ),
    };
  }
  if (blocked) return { primary: creating, hint: "You can start it from its page later." };
  if (draft.place === AGENT_PLACE_LATER) {
    return { primary: creating, hint: "Nothing runs until you add it to a project." };
  }
  return {
    primary: isSubmitting ? "Starting…" : "Create and start",
    hint: "It works while CompozyOS runs on this computer.",
  };
}

type AgentCreateView = ReturnType<typeof useAgentCreateViewState>;

function createdAgentName(draft: AgentCreateDialogDraft) {
  return draft.displayName.trim() || agentCreateSlug(draft);
}

function screenTitle(screen: AgentCreateScreenProps) {
  if (screen.startFailure) return createdAgentName(screen.draft);
  return screen.duplicateSourceName ? `Duplicate ${screen.duplicateSourceName}` : "New agent";
}

function AgentCreateMain({
  screen,
  view,
}: {
  screen: AgentCreateScreenProps;
  view: AgentCreateView;
}) {
  const { draft, onDraftChange, startFailure, submitError, runtimeModels, providerOptions } =
    screen;
  const { issue, errors } = view;
  return (
    <div className="flex min-h-0 flex-col gap-7 overflow-y-auto px-8 py-7">
      {startFailure ? (
        <Alert data-testid="agent-create-start-failure" variant="destructive">
          <CircleAlert aria-hidden="true" className="size-4" />
          <AlertTitle>{createdAgentName(draft)} was created, but it couldn't start</AlertTitle>
          <AlertDescription>Nothing was lost. You can try again from its page.</AlertDescription>
        </Alert>
      ) : null}
      {submitError || errors.scope ? (
        <Alert data-testid="agent-create-submit-error" variant="destructive">
          <AlertDescription>{submitError ?? errors.scope}</AlertDescription>
        </Alert>
      ) : null}
      {startFailure ? (
        <>
          <AgentCreateIdentityRead draft={draft} />
          <AgentCreateInstructionsRead value={draft.prompt} />
        </>
      ) : (
        <>
          <AgentCreateIdentity
            draft={draft}
            issue={issue}
            onChange={next => onDraftChange({ ...draft, ...next })}
          />
          <AgentCreateInstructions
            draft={draft}
            error={errors.prompt}
            models={runtimeModels}
            onChange={prompt => onDraftChange({ ...draft, prompt })}
            providers={providerOptions}
          />
          <AgentCreateAdvanced
            draft={draft}
            errors={errors}
            onDraftChange={onDraftChange}
            onOpenChange={view.setAdvancedOpen}
            open={view.showAdvanced}
          />
        </>
      )}
    </div>
  );
}

function AgentCreateSide({
  screen,
  view,
  blocked,
}: {
  screen: AgentCreateScreenProps;
  view: AgentCreateView;
  blocked: boolean;
}) {
  const { draft, startFailure, workspaces } = screen;
  return (
    <aside className="flex min-h-0 flex-col gap-6 overflow-y-auto bg-sunken p-6">
      {startFailure ? (
        <AgentCreateRunFacts draft={draft} workspaces={workspaces} />
      ) : (
        <AgentCreateRunSettings
          instanceCapacity={screen.instanceCapacity}
          draft={draft}
          errors={view.errors}
          modelCatalogError={screen.modelCatalogError}
          modelCatalogLoading={screen.modelCatalogLoading}
          modelCatalogRefreshing={screen.modelCatalogRefreshing}
          onDraftChange={screen.onDraftChange}
          onOpenProviderSettings={screen.onOpenProviderSettings}
          onRefreshCatalog={screen.onRefreshCatalog}
          providerOptions={screen.providerOptions}
          providersLoading={screen.providersLoading}
          runtimeModels={screen.runtimeModels}
          signedOut={view.signedOut}
          workspaces={workspaces}
        />
      )}
      <div className="mt-auto flex flex-col gap-2 pt-2">
        <Eyebrow>{startFailure ? "How it looks" : "How it will look"}</Eyebrow>
        <AgentCreatePreview
          draft={draft}
          state={startFailure ? "failed" : undefined}
          stateLabel={startFailure ? "Couldn't start" : blocked ? "Not working yet" : undefined}
          workspaces={workspaces}
        />
      </div>
    </aside>
  );
}

function AgentCreateFooter({
  screen,
  view,
  ready,
  blocked,
}: {
  screen: AgentCreateScreenProps;
  view: AgentCreateView;
  ready: boolean;
  blocked: boolean;
}) {
  const { draft, startFailure, isSubmitting, workspaceName } = screen;
  const { primary, hint } = outcome(draft, blocked, isSubmitting, workspaceName);
  return (
    <EntityDialogFooter
      cancelDisabled={isSubmitting}
      cancelLabel={startFailure ? "Close" : "Cancel"}
      cancelTestId="agent-create-cancel"
      hint={startFailure ? undefined : hint}
      isSaving={isSubmitting}
      onCancel={view.close}
      primaryDisabled={!startFailure && (isSubmitting || !ready)}
      primaryLabel={startFailure ? "Open agent" : primary}
      primaryTestId="submit-agent-create"
      primaryType="submit"
    />
  );
}

/** S3: one screen for every agent, on the `xl` dialog. Agent edit stays on the agent's page. */
export function AgentCreateScreen(screen: AgentCreateScreenProps) {
  const { open, onOpenChange, draft, startFailure = null, isSubmitting } = screen;
  const { hasActiveWorkspace, providerOptions, providersError, providersLoading } = screen;
  const validation = validateAgentCreateDraft(draft, {
    hasActiveWorkspace,
    providerOptions,
    providersError,
    providersLoading,
  });
  const view = useAgentCreateViewState({
    draft,
    providerOptions,
    providersError,
    providersLoading,
    takenNames: screen.takenNames,
    validation,
    startFailure,
    isSubmitting,
    onOpenChange,
    onSubmit: screen.onSubmit,
    onOpenCreatedAgent: screen.onOpenCreatedAgent,
  });
  const { close } = view;
  const blocked = screen.instanceCapacity.reached || view.signedOut !== null;

  return (
    <>
      <Dialog onOpenChange={next => (next ? onOpenChange(true) : close())} open={open}>
        <DialogContent
          className={`grid-rows-[auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("xl", { fill: true })}`}
          data-testid="agent-create-dialog"
          showCloseButton={false}
          unframed
        >
          <EntityDialogHeader
            description={
              startFailure
                ? "See how this agent is set up."
                : "Give it a name and tell it what to do."
            }
            icon={Bot}
            onClose={isSubmitting ? undefined : close}
            title={screenTitle(screen)}
          />
          <form className="contents" onSubmit={view.handleSubmit}>
            <div className="grid min-h-0 grid-cols-[minmax(0,1fr)_23rem] max-[760px]:block max-[760px]:overflow-y-auto">
              <AgentCreateMain screen={screen} view={view} />
              <AgentCreateSide blocked={blocked} screen={screen} view={view} />
            </div>
            <AgentCreateFooter
              blocked={blocked}
              ready={validation.simpleValid && view.issue === null}
              screen={screen}
              view={view}
            />
          </form>
        </DialogContent>
      </Dialog>
      <ConfirmDialog
        cancelLabel="Keep editing"
        confirmLabel="Discard"
        description="What you entered won't be saved."
        onConfirm={() => {
          view.setDiscarding(false);
          view.dismiss();
        }}
        onOpenChange={view.setDiscarding}
        open={view.discarding}
        title="Discard this agent?"
        tone="destructive"
      />
    </>
  );
}
