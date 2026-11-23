import { RefreshCw, Scale, Sparkles, TriangleAlert, WifiOff } from "@compozy/ui/icons";

import {
  Alert,
  AlertActions,
  AlertDescription,
  Button,
  cn,
  Field,
  FieldLabel,
  Pill,
  Spinner,
  type EntityMode,
} from "@compozy/ui";

import { ComputerLoadMeter } from "@/systems/computers/components/computer-load-meter";
import { RunOnPicker, type RunOnTarget } from "@/systems/computers/components/run-on-picker";
import {
  COMPUTERS_PRO,
  LOCAL_COMPUTER,
  type ComputerFixture,
  type LocalComputerFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import {
  DEVBOX_TRANSCRIPT,
  REMOTE_SESSION_ENTRIES,
} from "@/systems/computers/mocks/remote-computers-fixtures-sessions";
import { SessionCreateDialog } from "@/systems/session/components/session-create-dialog";
import {
  ROOT_ENVIRONMENT_TARGET,
  type SessionEnvironmentTarget,
} from "@/systems/session/lib/session-environment-target";

import { RemoteSessionsWindow } from "../remote-computers-shared-sessions/remote-sessions-window";
import { useRunOnSessionCreate } from "./hooks/use-run-on-session-create";
import { RUN_ON_AGENTS } from "./run-on-fixtures";

export interface RunOnSessionCreateDialogProps {
  localComputer?: LocalComputerFixture;
  computers?: readonly ComputerFixture[];
  initialTarget: RunOnTarget;
  initialProject?: string;
  initialAgent?: string;
  initialSessionName?: string;
  initialEnvironment?: SessionEnvironmentTarget;
  loadBalancingEnabled?: boolean;
  pickerDefaultOpen?: boolean;
  highlightedReconnectId?: string;
  initialReconnectingIds?: readonly string[];
  initialMode?: EntityMode;
}

/**
 * Composes the real production `SessionCreateDialog` (`SessionCreateSimpleSection` with
 * `AgentCommandSelect`, `SessionCreateAdvancedSection` with `SessionEnvironmentField`, and
 * `SessionDestinationRow` project picker in the footer), adding only the `Run on` field
 * before the session controls (`_uiux.md` U4).
 */
export function RunOnSessionCreateDialog({
  localComputer = LOCAL_COMPUTER,
  computers = COMPUTERS_PRO,
  initialTarget,
  initialProject,
  initialAgent = "general",
  initialSessionName = "",
  initialEnvironment = ROOT_ENVIRONMENT_TARGET,
  loadBalancingEnabled = true,
  pickerDefaultOpen = false,
  highlightedReconnectId,
  initialReconnectingIds = [],
  initialMode = "simple",
}: RunOnSessionCreateDialogProps) {
  const form = useRunOnSessionCreate({
    localComputer,
    computers,
    initialTarget,
    initialProject,
    initialAgent,
    initialSessionName,
    initialEnvironment,
    initialReconnectingIds,
    initialMode,
  });

  const runOnField = (
    <Field>
      <FieldLabel htmlFor="session-create-run-on">Run on</FieldLabel>
      <RunOnPicker
        allowAuto
        autoResolution={form.autoResolution}
        computers={computers}
        defaultOpen={pickerDefaultOpen}
        highlightedReconnectId={highlightedReconnectId}
        loadBalancingEnabled={loadBalancingEnabled}
        localComputer={localComputer}
        onAddComputer={() => undefined}
        onChange={form.handleTargetChange}
        onReconnect={form.handleReconnect}
        onUpgrade={form.handleUpgrade}
        reconnectingIds={form.reconnectingIds}
        triggerId="session-create-run-on"
        value={form.target}
      />
    </Field>
  );

  const notice =
    form.upgradeNotice ||
    (form.target === "auto" && form.autoResolution) ||
    form.isNoEligibleAuto ||
    (form.isOfflineTarget && form.selectedRemote) ? (
      <div className="flex flex-col gap-3">
        {form.upgradeNotice ? (
          <Alert variant="info">
            <Sparkles aria-hidden="true" className="size-4" />
            <AlertDescription>{form.upgradeNotice}</AlertDescription>
          </Alert>
        ) : null}

        {form.target === "auto" && form.autoResolution ? (
          <div
            className="flex flex-col gap-2 rounded-md border border-line-soft bg-sunken px-3 py-2.5"
            data-testid="auto-resolution-panel"
          >
            <div className="flex items-center gap-1.5 text-eyebrow font-medium text-fg">
              <Scale aria-hidden="true" className="size-3.5 text-subtle" />
              <span>Eligible computers for {form.effectiveProject?.name ?? "this project"}</span>
            </div>
            <div className="flex flex-col gap-1.5 border-t border-line-soft pt-2">
              {form.autoResolution.candidates.map(candidate => (
                <div
                  className={cn(
                    "flex items-center justify-between gap-2 text-eyebrow",
                    candidate.winner ? "text-fg" : "text-muted-foreground"
                  )}
                  key={candidate.id}
                >
                  <span className="flex min-w-0 items-center gap-1.5 truncate">
                    <span className={cn("truncate", candidate.winner && "font-medium")}>
                      {candidate.name}
                    </span>
                    {candidate.winner ? (
                      <Pill form="tint" size="sm" tone="success">
                        Chosen
                      </Pill>
                    ) : null}
                    <span className="shrink-0 text-badge text-faint">{candidate.weightLabel}</span>
                  </span>
                  <ComputerLoadMeter computerName={candidate.name} load={candidate.load} />
                </div>
              ))}
            </div>
          </div>
        ) : null}

        {form.isNoEligibleAuto ? (
          <Alert data-testid="auto-no-eligible-alert" variant="warning">
            <TriangleAlert aria-hidden="true" className="size-4" />
            <AlertDescription>
              <span className="font-medium text-fg">No computer can run this project.</span>{" "}
              {form.effectiveProject?.name ?? "This project"} is only on computers that are
              currently offline or incompatible. Pick another project or reconnect a computer.
            </AlertDescription>
          </Alert>
        ) : null}

        {form.isOfflineTarget && form.selectedRemote ? (
          <Alert data-testid="offline-draft-alert" variant="warning">
            <WifiOff aria-hidden="true" className="size-4" />
            <AlertDescription>
              <span className="font-medium text-fg">{form.selectedRemote.name} is offline.</span>{" "}
              Your draft is kept here; reconnect to start the session.
            </AlertDescription>
            <AlertActions>
              <Button
                disabled={form.isReconnectingTarget}
                onClick={() => form.handleReconnect(form.selectedRemote!.id)}
                size="xs"
                type="button"
                variant="secondary"
              >
                {form.isReconnectingTarget ? (
                  <Spinner aria-hidden="true" data-icon="inline-start" />
                ) : (
                  <RefreshCw aria-hidden="true" data-icon="inline-start" />
                )}
                {form.isReconnectingTarget ? "Reconnecting…" : "Reconnect"}
              </Button>
            </AlertActions>
          </Alert>
        ) : null}
      </div>
    ) : null;

  return (
    <SessionCreateDialog
      agents={RUN_ON_AGENTS}
      destinationAxes={
        form.effectiveProject
          ? {
              project: {
                options: form.availableProjects.map(project => ({
                  value: project.id,
                  label: project.name,
                })),
                value: form.effectiveProject.id,
                onChange: form.handleProjectChange,
              },
            }
          : undefined
      }
      destinationLabel={form.effectiveProject?.name ?? "No project"}
      destinationReady={Boolean(form.effectiveProject)}
      environment={
        form.effectiveProject
          ? {
              value: form.environmentTarget,
              worktrees: form.worktrees,
              onChange: form.setEnvironmentTarget,
            }
          : undefined
      }
      environmentListingState={form.effectiveProject ? "ready" : "unsupported"}
      isAwaitingEnvironment={false}
      isSubmitting={false}
      mode={form.mode}
      notice={notice}
      onAgentChange={form.setSelectedAgentName}
      onCancelEnvironment={() => undefined}
      onModeChange={form.setMode}
      onOpenChange={() => undefined}
      onSessionNameChange={form.setSessionName}
      onSubmit={() => undefined}
      open
      restoreFocusOnClose={false}
      runOn={runOnField}
      scope="project"
      selectedAgentName={form.selectedAgentName}
      sessionName={form.sessionName}
      sessionRoot={form.effectiveProject?.path ?? ""}
      submitDisabled={form.submitBlocked}
      submitError={null}
    />
  );
}

export type RunOnSessionCreateSceneProps = RunOnSessionCreateDialogProps;

export function RunOnSessionCreateScene(props: RunOnSessionCreateSceneProps) {
  const backdropSession = REMOTE_SESSION_ENTRIES[2]!;
  const backdropComputer = COMPUTERS_PRO[2]!;

  return (
    <RemoteSessionsWindow
      activeComputer={backdropComputer}
      activeSession={backdropSession}
      isWorking
      overlay={<RunOnSessionCreateDialog {...props} />}
      sessions={REMOTE_SESSION_ENTRIES}
      transcript={DEVBOX_TRANSCRIPT}
    />
  );
}
