import { Sparkles } from "@compozy/ui/icons";

import { Alert, AlertDescription, Field, FieldLabel, type EntityMode } from "@compozy/ui";

import { RunOnNotices } from "@/systems/computers/components/run-on-notices";
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

  const projectName = form.effectiveProject?.name;
  const offlineComputer = form.isOfflineTarget ? form.selectedRemote : null;
  const candidates = form.target === "auto" ? form.autoResolution?.candidates : undefined;
  const notice =
    form.upgradeNotice || candidates?.length || form.isNoEligibleAuto || offlineComputer ? (
      <div className="flex flex-col gap-3">
        {form.upgradeNotice && (
          <Alert variant="info">
            <Sparkles aria-hidden="true" className="size-4" />
            <AlertDescription>{form.upgradeNotice}</AlertDescription>
          </Alert>
        )}
        <RunOnNotices
          autoCandidates={candidates}
          autoNoEligible={form.isNoEligibleAuto}
          autoNoEligibleReason={`${projectName ?? "This project"} is only on computers that are currently offline or incompatible. Pick another project or reconnect a computer.`}
          isReconnectingOffline={form.isReconnectingTarget}
          offlineComputer={offlineComputer}
          onReconnectOffline={form.handleReconnect}
          projectName={projectName}
        />
      </div>
    ) : undefined;

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
      destinationLabel={projectName ?? "No project"}
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
