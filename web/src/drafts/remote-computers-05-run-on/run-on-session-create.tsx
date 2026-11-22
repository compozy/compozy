import { RefreshCw, Scale, Sparkles, TriangleAlert, WifiOff } from "@compozy/ui/icons";
import { useMemo, useState } from "react";

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
  projectsForTarget,
  REMOTE_SESSION_ENTRIES,
  resolveAutoComputer,
} from "@/systems/computers/mocks/remote-computers-fixtures-sessions";
import { SessionCreateDialog } from "@/systems/session/components/session-create-dialog";
import {
  ROOT_ENVIRONMENT_TARGET,
  type SessionEnvironmentTarget,
} from "@/systems/session/lib/session-environment-target";

import { RemoteSessionsWindow } from "../remote-computers-shared-sessions";
import { RUN_ON_AGENTS, worktreesForProject } from "./run-on-fixtures";

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
  const [target, setTarget] = useState<RunOnTarget>(initialTarget);
  const [mode, setMode] = useState<EntityMode>(initialMode);
  const [selectedAgentName, setSelectedAgentName] = useState(initialAgent);
  const [sessionName, setSessionName] = useState(initialSessionName);
  const [environmentTarget, setEnvironmentTarget] =
    useState<SessionEnvironmentTarget>(initialEnvironment);
  const [reconnectingIds, setReconnectingIds] = useState<string[]>([...initialReconnectingIds]);
  const [upgradeNotice, setUpgradeNotice] = useState<string | null>(null);

  const availableProjects = useMemo(() => projectsForTarget(target), [target]);
  const [selectedProjectName, setSelectedProjectName] = useState<string>(
    initialProject ?? availableProjects[0]?.name ?? "api"
  );

  const effectiveProject = useMemo(() => {
    const match = availableProjects.find(project => project.name === selectedProjectName);
    return match ?? availableProjects[0] ?? null;
  }, [availableProjects, selectedProjectName]);

  const worktrees = useMemo(
    () => worktreesForProject(effectiveProject?.id),
    [effectiveProject?.id]
  );

  const autoResolution = useMemo(() => {
    if (!effectiveProject) return null;
    return resolveAutoComputer(effectiveProject.name, localComputer, computers);
  }, [effectiveProject, localComputer, computers]);

  const selectedRemote = useMemo(
    () => computers.find(computer => computer.id === target) ?? null,
    [computers, target]
  );

  const isOfflineTarget = selectedRemote?.status === "offline";
  const isNoEligibleAuto = target === "auto" && !autoResolution;
  const isLocalUnavailable =
    !localComputer.localRunAgents &&
    computers.filter(c => c.enabled && c.status === "connected").length === 0;
  const isReconnectingTarget =
    selectedRemote !== null && reconnectingIds.includes(selectedRemote.id);

  const submitBlocked = isOfflineTarget || isNoEligibleAuto || isLocalUnavailable;

  const handleTargetChange = (next: RunOnTarget) => {
    setTarget(next);
    setUpgradeNotice(null);
    const nextProjects = projectsForTarget(next);
    if (nextProjects.length > 0 && !nextProjects.some(p => p.name === selectedProjectName)) {
      setSelectedProjectName(nextProjects[0]!.name);
    }
    // Worktrees belong to one computer and project: switching computer resets to root.
    setEnvironmentTarget(ROOT_ENVIRONMENT_TARGET);
  };

  const handleProjectChange = (nextProjectId: string) => {
    const picked = availableProjects.find(project => project.id === nextProjectId);
    if (!picked) return;
    setSelectedProjectName(picked.name);
    setEnvironmentTarget(ROOT_ENVIRONMENT_TARGET);
  };

  const handleReconnect = (computerId: string) => {
    setReconnectingIds(prev => (prev.includes(computerId) ? prev : [...prev, computerId]));
  };

  const runOnField = (
    <Field>
      <FieldLabel htmlFor="session-create-run-on">Run on</FieldLabel>
      <RunOnPicker
        allowAuto
        autoResolution={autoResolution}
        computers={computers}
        defaultOpen={pickerDefaultOpen}
        highlightedReconnectId={highlightedReconnectId}
        loadBalancingEnabled={loadBalancingEnabled}
        localComputer={localComputer}
        onAddComputer={() => undefined}
        onChange={handleTargetChange}
        onReconnect={handleReconnect}
        onUpgrade={id => {
          const found = computers.find(computer => computer.id === id);
          setUpgradeNotice(
            `${found?.name ?? "This computer"} needs Compozy Pro. Opening the Upgrade sheet…`
          );
        }}
        reconnectingIds={reconnectingIds}
        triggerId="session-create-run-on"
        value={target}
      />
    </Field>
  );

  const notice =
    upgradeNotice ||
    (target === "auto" && autoResolution) ||
    isNoEligibleAuto ||
    (isOfflineTarget && selectedRemote) ? (
      <div className="flex flex-col gap-3">
        {upgradeNotice ? (
          <Alert variant="info">
            <Sparkles aria-hidden="true" className="size-4" />
            <AlertDescription>{upgradeNotice}</AlertDescription>
          </Alert>
        ) : null}

        {target === "auto" && autoResolution ? (
          <div
            className="flex flex-col gap-2 rounded-md border border-line-soft bg-sunken px-3 py-2.5"
            data-testid="auto-resolution-panel"
          >
            <div className="flex items-center gap-1.5 text-eyebrow font-medium text-fg">
              <Scale aria-hidden="true" className="size-3.5 text-subtle" />
              <span>Eligible computers for {effectiveProject?.name ?? "this project"}</span>
            </div>
            <div className="flex flex-col gap-1.5 border-t border-line-soft pt-2">
              {autoResolution.candidates.map(candidate => (
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

        {isNoEligibleAuto ? (
          <Alert data-testid="auto-no-eligible-alert" variant="warning">
            <TriangleAlert aria-hidden="true" className="size-4" />
            <AlertDescription>
              <span className="font-medium text-fg">No computer can run this project.</span>{" "}
              {effectiveProject?.name ?? "This project"} is only on computers that are currently
              offline or incompatible. Pick another project or reconnect a computer.
            </AlertDescription>
          </Alert>
        ) : null}

        {isOfflineTarget && selectedRemote ? (
          <Alert data-testid="offline-draft-alert" variant="warning">
            <WifiOff aria-hidden="true" className="size-4" />
            <AlertDescription>
              <span className="font-medium text-fg">{selectedRemote.name} is offline.</span> Your
              draft is kept here; reconnect to start the session.
            </AlertDescription>
            <AlertActions>
              <Button
                disabled={isReconnectingTarget}
                onClick={() => handleReconnect(selectedRemote.id)}
                size="xs"
                type="button"
                variant="secondary"
              >
                {isReconnectingTarget ? (
                  <Spinner aria-hidden="true" data-icon="inline-start" />
                ) : (
                  <RefreshCw aria-hidden="true" data-icon="inline-start" />
                )}
                {isReconnectingTarget ? "Reconnecting…" : "Reconnect"}
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
        effectiveProject
          ? {
              project: {
                options: availableProjects.map(project => ({
                  value: project.id,
                  label: project.name,
                })),
                value: effectiveProject.id,
                onChange: handleProjectChange,
              },
            }
          : undefined
      }
      destinationLabel={effectiveProject?.name ?? "No project"}
      destinationReady={Boolean(effectiveProject)}
      environment={
        effectiveProject
          ? {
              value: environmentTarget,
              worktrees,
              onChange: setEnvironmentTarget,
            }
          : undefined
      }
      environmentListingState={effectiveProject ? "ready" : "unsupported"}
      isAwaitingEnvironment={false}
      isSubmitting={false}
      mode={mode}
      notice={notice}
      onAgentChange={setSelectedAgentName}
      onCancelEnvironment={() => undefined}
      onModeChange={setMode}
      onOpenChange={() => undefined}
      onSessionNameChange={setSessionName}
      onSubmit={() => undefined}
      open
      restoreFocusOnClose={false}
      runOn={runOnField}
      scope="project"
      selectedAgentName={selectedAgentName}
      sessionName={sessionName}
      sessionRoot={effectiveProject?.path ?? ""}
      submitDisabled={submitBlocked}
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
