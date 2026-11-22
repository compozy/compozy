import {
  Check,
  ChevronDown,
  FolderOpen,
  Play,
  RefreshCw,
  Scale,
  Sparkles,
  TriangleAlert,
  WifiOff,
} from "@compozy/ui/icons";
import { useMemo, useState } from "react";

import {
  Alert,
  AlertActions,
  AlertDescription,
  Button,
  cn,
  CommandEmpty,
  CommandItem,
  CommandList,
  CommandSelect,
  CommandSelectGroup,
  CommandSelectShell,
  CommandSelectTrigger,
  CommandSeparator,
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  EntityModeToolbar,
  Field,
  FieldDescription,
  FieldLabel,
  Input,
  Pill,
  Spinner,
  Textarea,
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
import { RuntimeSelector } from "@/systems/runtime/components/runtime-selector";
import type {
  RuntimeModelOption,
  RuntimeProviderOption,
  RuntimeSelectorValue,
} from "@/systems/runtime/components/runtime-selector";

import { RUNTIME_MODELS, RUNTIME_PROVIDERS } from "../always-on-agents-shared/sessions-fixtures";
import { RemoteSessionsWindow } from "../remote-computers-shared-sessions";

export interface RunOnSessionCreateSceneProps {
  localComputer?: LocalComputerFixture;
  computers?: readonly ComputerFixture[];
  initialTarget: RunOnTarget;
  initialProject?: string;
  loadBalancingEnabled?: boolean;
  pickerDefaultOpen?: boolean;
  highlightedReconnectId?: string;
  initialReconnectingIds?: readonly string[];
  initialPrompt?: string;
  initialMode?: EntityMode;
}

export function RunOnSessionCreateScene({
  localComputer = LOCAL_COMPUTER,
  computers = COMPUTERS_PRO,
  initialTarget,
  initialProject,
  loadBalancingEnabled = true,
  pickerDefaultOpen = false,
  highlightedReconnectId,
  initialReconnectingIds = [],
  initialPrompt = "",
  initialMode = "simple",
}: RunOnSessionCreateSceneProps) {
  const [target, setTarget] = useState<RunOnTarget>(initialTarget);
  const [mode, setMode] = useState<EntityMode>(initialMode);
  const [prompt, setPrompt] = useState(initialPrompt);
  const [sessionName, setSessionName] = useState("");
  const [projectOpen, setProjectOpen] = useState(false);
  const [reconnectingIds, setReconnectingIds] = useState<string[]>([...initialReconnectingIds]);
  const [upgradeNotice, setUpgradeNotice] = useState<string | null>(null);
  const [runtime, setRuntime] = useState<RuntimeSelectorValue>({
    provider: "claude",
    model: "claude-opus-5-5",
    reasoning_effort: "",
  });

  const availableProjects = useMemo(() => projectsForTarget(target), [target]);
  const [selectedProjectName, setSelectedProjectName] = useState<string>(
    initialProject ?? availableProjects[0]?.name ?? "api"
  );

  const effectiveProject = useMemo(() => {
    const match = availableProjects.find(p => p.name === selectedProjectName);
    return match ?? availableProjects[0] ?? null;
  }, [availableProjects, selectedProjectName]);

  const autoResolution = useMemo(() => {
    if (!effectiveProject) return null;
    return resolveAutoComputer(effectiveProject.name, localComputer, computers);
  }, [effectiveProject, localComputer, computers]);

  const selectedRemote = useMemo(
    () => computers.find(c => c.id === target) ?? null,
    [computers, target]
  );

  const targetName =
    target === "local"
      ? "This computer"
      : target === "auto"
        ? autoResolution
          ? `Auto → ${autoResolution.computerName}`
          : "Auto"
        : (selectedRemote?.name ?? "computer");

  const isOfflineTarget = selectedRemote?.status === "offline";
  const isNoEligibleAuto = target === "auto" && !autoResolution;
  const isLocalUnavailable =
    !localComputer.localRunAgents &&
    computers.filter(c => c.enabled && c.status === "connected").length === 0;
  const isReconnectingTarget =
    selectedRemote !== null && reconnectingIds.includes(selectedRemote.id);

  const canSubmit =
    !isOfflineTarget && !isNoEligibleAuto && !isLocalUnavailable && Boolean(effectiveProject);

  const handleTargetChange = (next: RunOnTarget) => {
    setTarget(next);
    setUpgradeNotice(null);
    const nextProjects = projectsForTarget(next);
    if (nextProjects.length > 0 && !nextProjects.some(p => p.name === selectedProjectName)) {
      setSelectedProjectName(nextProjects[0]!.name);
    }
  };

  const handleReconnect = (computerId: string) => {
    setReconnectingIds(prev => (prev.includes(computerId) ? prev : [...prev, computerId]));
  };

  const backdropSession = REMOTE_SESSION_ENTRIES[2]!;
  const backdropComputer = COMPUTERS_PRO[2]!;

  const dialog = (
    <Dialog onOpenChange={() => undefined} open>
      <DialogContent
        className={`grid-rows-[auto_auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("sm")}`}
        data-testid="run-on-session-create-dialog"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          eyebrow="New session"
          icon={Play}
          onClose={() => undefined}
          title="Start session"
        />

        <EntityModeToolbar mode={mode} onModeChange={setMode} testIdPrefix="run-on-create" />

        <EntityDialogBody className="flex flex-col gap-4">
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <Field>
              <FieldLabel>Run on</FieldLabel>
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
                  const found = computers.find(c => c.id === id);
                  setUpgradeNotice(
                    `${found?.name ?? "This computer"} needs Compozy Pro. Opening the Upgrade sheet…`
                  );
                }}
                reconnectingIds={reconnectingIds}
                value={target}
              />
            </Field>

            <Field>
              <FieldLabel>Project</FieldLabel>
              <CommandSelect onOpenChange={setProjectOpen} open={projectOpen}>
                <CommandSelectTrigger
                  aria-label={`Project: ${effectiveProject?.name ?? "Choose project"}`}
                  className="[&>svg:last-child]:hidden"
                  selected={Boolean(effectiveProject)}
                >
                  <span className="flex min-w-0 flex-1 items-center gap-1.5 text-left">
                    <FolderOpen aria-hidden="true" className="size-3.5 shrink-0 text-subtle" />
                    <span className="truncate font-medium text-fg">
                      {effectiveProject?.name ?? "Choose project"}
                    </span>
                  </span>
                  <ChevronDown aria-hidden="true" className="size-3.5 shrink-0 text-subtle" />
                </CommandSelectTrigger>
                <CommandSelectShell
                  className="w-72 p-0"
                  inputPlaceholder={
                    target === "local"
                      ? "Search projects…"
                      : `Search projects on ${selectedRemote?.name ?? "computers"}…`
                  }
                >
                  <CommandList>
                    <CommandEmpty>No projects on {targetName}.</CommandEmpty>
                    <CommandSelectGroup
                      heading={
                        target === "auto"
                          ? "Shared projects"
                          : `Projects on ${selectedRemote?.name ?? "this computer"}`
                      }
                    >
                      {availableProjects.map(project => (
                        <CommandItem
                          key={project.id}
                          onSelect={() => {
                            setSelectedProjectName(project.name);
                            setProjectOpen(false);
                          }}
                          value={`${project.name} ${project.path}`}
                        >
                          <span className="flex min-w-0 flex-1 flex-col">
                            <span className="truncate text-small-body font-medium text-fg">
                              {project.name}
                            </span>
                            <span className="truncate font-mono text-badge text-faint">
                              {project.path}
                            </span>
                          </span>
                          {effectiveProject?.name === project.name ? (
                            <Check aria-hidden="true" className="size-3.5 shrink-0 text-fg" />
                          ) : null}
                        </CommandItem>
                      ))}
                    </CommandSelectGroup>
                    <CommandSeparator />
                    <CommandSelectGroup>
                      <CommandItem
                        onSelect={() => setProjectOpen(false)}
                        value="add project browse"
                      >
                        <FolderOpen aria-hidden="true" className="size-3.5 text-subtle" />
                        <span className="text-small-body text-muted-foreground">
                          {target === "local" || target === "auto"
                            ? "Add project…"
                            : `Browse files on ${selectedRemote?.name ?? "computer"}…`}
                        </span>
                      </CommandItem>
                    </CommandSelectGroup>
                  </CommandList>
                </CommandSelectShell>
              </CommandSelect>
              {effectiveProject ? (
                <FieldDescription className="truncate font-mono text-badge">
                  {effectiveProject.path}
                </FieldDescription>
              ) : null}
            </Field>
          </div>

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
                      <span className="shrink-0 text-badge text-faint">
                        {candidate.weightLabel}
                      </span>
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

          <Field>
            <FieldLabel>Agent and model</FieldLabel>
            <div className="flex items-center gap-2">
              <RuntimeSelector
                models={RUNTIME_MODELS as unknown as RuntimeModelOption[]}
                onChange={setRuntime}
                providers={RUNTIME_PROVIDERS as RuntimeProviderOption[]}
                value={runtime}
                variant="composer"
              />
              <span className="text-eyebrow text-subtle">general · default agent</span>
            </div>
          </Field>

          <Field>
            <FieldLabel htmlFor="run-on-first-prompt">First message</FieldLabel>
            <Textarea
              id="run-on-first-prompt"
              onChange={event => setPrompt(event.target.value)}
              placeholder={
                isOfflineTarget && selectedRemote
                  ? `Draft your prompt for ${selectedRemote.name}…`
                  : `What should the agent work on in ${effectiveProject?.name ?? "this project"}?`
              }
              rows={3}
              value={prompt}
            />
          </Field>

          {mode === "advanced" ? (
            <Field>
              <FieldLabel htmlFor="run-on-session-name">Session title (optional)</FieldLabel>
              <Input
                id="run-on-session-name"
                onChange={event => setSessionName(event.target.value)}
                placeholder="Generated from your first message"
                value={sessionName}
              />
            </Field>
          ) : null}
        </EntityDialogBody>

        <EntityDialogFooter
          hint={
            isOfflineTarget && selectedRemote ? (
              <span className="text-form-hint text-warning">{selectedRemote.name} is offline</span>
            ) : isNoEligibleAuto ? (
              <span className="text-form-hint text-warning">No computer can run this project</span>
            ) : target === "auto" ? undefined : (
              <span className="text-form-hint text-muted-foreground">
                Runs on <span className="font-medium text-fg">{targetName}</span> in{" "}
                <span className="font-medium text-fg">{effectiveProject?.name ?? "project"}</span>
              </span>
            )
          }
          onCancel={() => undefined}
          primaryDisabled={!canSubmit}
          primaryLabel="Start session"
          primaryTestId="run-on-start-session"
        />
      </DialogContent>
    </Dialog>
  );

  return (
    <RemoteSessionsWindow
      activeComputer={backdropComputer}
      activeSession={backdropSession}
      isWorking
      overlay={dialog}
      sessions={REMOTE_SESSION_ENTRIES}
      transcript={DEVBOX_TRANSCRIPT}
    />
  );
}
