import {
  AlarmClock,
  Check,
  FolderOpen,
  Info,
  TriangleAlert,
  Zap,
} from "@compozy/ui/icons";
import { useMemo, useState } from "react";

import {
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  EntityDialogToolbar,
  Field,
  FieldDescription,
  FieldLabel,
  Input,
  NativeSelect,
  NativeSelectOption,
  Pill,
} from "@compozy/ui";

import { AutomationChoiceCards } from "@/systems/automation/components/automation-form/automation-choice-cards";
import { AutomationFormSection } from "@/systems/automation/components/automation-form/automation-form-section";
import { AutomationOptions } from "@/systems/automation/components/automation-form/automation-options";
import type { AutomationFormModel } from "@/systems/automation/hooks/use-automation-form";
import { retryDraftForStrategy } from "@/systems/automation/lib/automation-drafts";
import type { AutomationFormDraft } from "@/systems/automation/lib/automation-form-draft";
import type {
  AutomationCatchUpPolicy,
  AutomationFireLimit,
  AutomationRetry,
  AutomationSchedule,
  AutomationScheduleMode,
} from "@/systems/automation/types";
import { RunOnPicker, type RunOnTarget } from "@/systems/computers/components/run-on-picker";
import { COMPUTERS_PRO, LOCAL_COMPUTER } from "@/systems/computers/mocks/remote-computers-fixtures";
import { projectsForTarget } from "@/systems/computers/mocks/remote-computers-fixtures-sessions";

import { DraftShell } from "../always-on-agents-shared/draft-shell";
import {
  AUTOMATION_DOES_CHOICES,
  AUTOMATION_START_CHOICES,
  AutomationAgentInset,
  AutomationScheduleInset,
  buildScheduleReadout,
  describeSchedulePhrase,
  type DraftAutomationDoes,
  type DraftAutomationStart,
} from "./run-on-automation-sections";
import { automationTimeZoneForTarget, RUN_ON_AGENTS } from "./run-on-fixtures";

const FIXED_NOW = Date.parse("2026-11-22T15:00:00Z");

const WIDE_CONTENT_CLASS = `grid-rows-[auto_minmax(0,1fr)] text-fg ${dialogShellClass("lg", {
  fill: true,
})}`;

/**
 * Screen 05 Host C: Automation job editor composing the real production automation chrome
 * (`EntityDialogHeader`, `EntityDialogToolbar`, `AutomationFormSection`, `AutomationChoiceCards`,
 * `AgentCommandSelect`, `ScheduleReadout`, `AutomationOptions`, `EntityDialogFooter`), with
 * concrete `Run on` (`allowAuto={false}`) before Project, the exact ownership hint, and the
 * target computer's effective automation timezone beside the schedule time
 * (`09:00 · America/Sao_Paulo`, `_uiux.md` U4).
 */
export function RunOnAutomationEditorScene({
  initialTarget = "cmp_7f3k2q9x",
  pickerDefaultOpen = false,
}: {
  initialTarget?: RunOnTarget;
  pickerDefaultOpen?: boolean;
}) {
  const [target, setTarget] = useState<RunOnTarget>(initialTarget);
  const [start, setStart] = useState<DraftAutomationStart>("schedule");
  const [does, setDoes] = useState<DraftAutomationDoes>("agent");
  const [name, setName] = useState("morning-pg-pool-smoke");
  const [agentName, setAgentName] = useState("general");
  const [prompt, setPrompt] = useState(
    "Run go test -race ./internal/pgpool/... and report any flake."
  );
  const [schedule, setSchedule] = useState<AutomationSchedule>({
    mode: "cron",
    expr: "0 9 * * 1-5",
  });
  const [daysCleared, setDaysCleared] = useState(false);
  const [expressionOpen, setExpressionOpen] = useState(false);
  const [optionsOpen, setOptionsOpen] = useState(false);
  const [enabled, setEnabled] = useState(true);
  const [retry, setRetry] = useState<AutomationRetry>({ strategy: "none" });
  const [fireLimit, setFireLimit] = useState<AutomationFireLimit>({ max: 12, window: "1h" });

  const projects = useMemo(() => projectsForTarget(target), [target]);
  const [workspaceId, setWorkspaceId] = useState(projects[0]?.id ?? "ws_devbox_api");
  const currentProject =
    projects.find(project => project.id === workspaceId) ?? projects[0] ?? null;

  const selectedRemote = COMPUTERS_PRO.find(computer => computer.id === target) ?? null;
  const computerDisplayName =
    target === "local" ? LOCAL_COMPUTER.name : (selectedRemote?.name ?? "devbox");
  const isOffline = selectedRemote?.status === "offline";
  const targetTimeZone = automationTimeZoneForTarget(target);

  const agentLabel = RUN_ON_AGENTS.find(agent => agent.name === agentName)?.label ?? agentName;
  const readout = useMemo(
    () =>
      start === "schedule"
        ? buildScheduleReadout(schedule, FIXED_NOW, daysCleared, targetTimeZone)
        : null,
    [start, schedule, daysCleared, targetTimeZone]
  );

  const ready =
    !isOffline &&
    name.trim().length > 0 &&
    agentName.trim().length > 0 &&
    prompt.trim().length > 0 &&
    (readout?.valid ?? true);

  const schedulePhrase = describeSchedulePhrase(schedule, targetTimeZone);

  const handleScheduleMode = (mode: AutomationScheduleMode) => {
    setDaysCleared(false);
    if (mode === "cron") setSchedule({ mode: "cron", expr: "0 9 * * 1-5" });
    else if (mode === "every") setSchedule({ mode: "every", interval: "30m" });
    else setSchedule({ mode: "at", time: "2026-11-23T09:00" });
  };

  const optionsDraft = {
    start,
    schedule,
    enabled,
    retry,
    fire_limit: fireLimit,
  } as AutomationFormDraft;

  const optionsForm = {
    does,
    retry: retryDraftForStrategy(retry.strategy ?? "none", retry),
    optionsSummary: `${retry.strategy === "backoff" ? `retry ${retry.max_retries ?? 3}×` : "no retry"} · ${fireLimit.max}/${fireLimit.window} · ${enabled ? "on" : "off"}`,
    onRetryChange: next => {
      if (does !== "task") setRetry(next);
    },
    onFireLimitChange: setFireLimit,
    onEnabledChange: setEnabled,
    onCatchUpPolicyChange: (catch_up_policy: AutomationCatchUpPolicy | undefined) =>
      setSchedule(prev => ({ ...prev, catch_up_policy })),
    onMisfireGraceChange: (misfire_grace_seconds: number | undefined) =>
      setSchedule(prev => ({ ...prev, misfire_grace_seconds })),
  } as AutomationFormModel;

  const overlay = (
    <Dialog onOpenChange={() => undefined} open>
      <DialogContent
        className={WIDE_CONTENT_CLASS}
        data-testid="automation-editor-dialog"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description={
            <>
              Choose <b className="font-medium text-fg">when it starts</b> and{" "}
              <b className="font-medium text-fg">what it does</b>. You can turn it off any time.
            </>
          }
          eyebrow="Automation"
          icon={Zap}
          onClose={() => undefined}
          title="New automation"
        />

        <form
          className="flex min-h-0 flex-col"
          data-testid="automation-form"
          onSubmit={event => event.preventDefault()}
        >
          <EntityDialogToolbar
            className="flex-nowrap border-b border-line-soft bg-sunken"
            data-testid="automation-editor-sentence-bar"
            leading={
              <p
                aria-live="polite"
                className="min-w-0 flex-1 text-small-body leading-snug text-muted-foreground"
                data-testid="automation-editor-sentence"
              >
                <em className="not-italic text-fg-2">{schedulePhrase}</em>, ask{" "}
                <em className="not-italic text-fg-2">{agentLabel || "an agent"}</em> in{" "}
                <em className="not-italic text-fg-2">{currentProject?.name ?? "api"}</em> on{" "}
                <em className="not-italic text-fg-2">{computerDisplayName}</em>.
              </p>
            }
            trailing={
              <Pill data-testid="automation-editor-status" tone={ready ? "success" : "warning"}>
                {ready ? <Check aria-hidden="true" /> : <TriangleAlert aria-hidden="true" />}
                {ready ? "Ready" : "Needs a fix"}
              </Pill>
            }
          />

          <EntityDialogBody data-testid="automation-form-body">
            <AutomationFormSection
              data-testid="automation-form-name"
              description="How you'll find it in the list"
              number={1}
              title="Name"
            >
              <Field>
                <FieldLabel className="sr-only" htmlFor="automation-name">
                  Automation name
                </FieldLabel>
                <Input
                  className="font-mono"
                  data-testid="automation-name-input"
                  id="automation-name"
                  onChange={event => setName(event.target.value)}
                  placeholder="morning-digest"
                  value={name}
                />
              </Field>
            </AutomationFormSection>

            <AutomationFormSection
              data-testid="automation-form-computer"
              number={2}
              title="Where it runs"
            >
              <div className="flex flex-col gap-2.5">
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                  <Field>
                    <FieldLabel htmlFor="automation-run-on">Run on</FieldLabel>
                    <RunOnPicker
                      allowAuto={false}
                      computers={COMPUTERS_PRO}
                      defaultOpen={pickerDefaultOpen}
                      loadBalancingEnabled
                      localComputer={LOCAL_COMPUTER}
                      onAddComputer={() => undefined}
                      onChange={next => {
                        setTarget(next);
                        const nextProjects = projectsForTarget(next);
                        if (nextProjects[0]) setWorkspaceId(nextProjects[0].id);
                      }}
                      triggerId="automation-run-on"
                      value={target}
                    />
                  </Field>

                  <Field>
                    <FieldLabel htmlFor="automation-project">Project</FieldLabel>
                    <NativeSelect
                      id="automation-project"
                      onChange={event => setWorkspaceId(event.target.value)}
                      value={currentProject?.id ?? ""}
                    >
                      {projects.map(project => (
                        <NativeSelectOption key={project.id} value={project.id}>
                          {project.name}
                        </NativeSelectOption>
                      ))}
                    </NativeSelect>
                    {currentProject ? (
                      <FieldDescription className="truncate font-mono text-badge">
                        {currentProject.path}
                      </FieldDescription>
                    ) : null}
                  </Field>
                </div>

                <p
                  className="flex items-center gap-1.5 text-form-hint text-muted-foreground"
                  data-testid="automation-computer-ownership-hint"
                >
                  <Info aria-hidden="true" className="size-3.5 shrink-0 text-subtle" />
                  <span>
                    This job is saved on{" "}
                    <span className="font-medium text-fg">{computerDisplayName}</span> and runs
                    there, even when this app is closed.
                  </span>
                </p>
              </div>
            </AutomationFormSection>

            <AutomationFormSection
              data-testid="automation-form-starts"
              description="When should it run?"
              number={3}
              title="Starts"
            >
              <AutomationChoiceCards
                choices={AUTOMATION_START_CHOICES}
                label="When it starts"
                onChange={setStart}
                testIdPrefix="automation-start"
                value={start}
              />
              <AutomationScheduleInset
                daysCleared={daysCleared}
                expressionOpen={expressionOpen}
                onDaysClearedChange={setDaysCleared}
                onExpressionOpenChange={setExpressionOpen}
                onScheduleMode={handleScheduleMode}
                onSchedulePatch={patch => setSchedule(prev => ({ ...prev, ...patch }))}
                readout={readout}
                schedule={schedule}
                targetTimeZone={targetTimeZone}
              />
            </AutomationFormSection>

            <AutomationFormSection
              data-testid="automation-form-does"
              description="What should happen?"
              number={4}
              title="Does"
            >
              <AutomationChoiceCards
                choices={AUTOMATION_DOES_CHOICES}
                label="What it does"
                onChange={setDoes}
                testIdPrefix="automation-does"
                value={does}
              />
              <AutomationAgentInset
                agentName={agentName}
                agents={RUN_ON_AGENTS}
                onAgentChange={setAgentName}
                onPromptChange={setPrompt}
                prompt={prompt}
              />
            </AutomationFormSection>

            <AutomationOptions
              draft={optionsDraft}
              form={optionsForm}
              onOpenChange={setOptionsOpen}
              open={optionsOpen}
            />
          </EntityDialogBody>

          <EntityDialogFooter
            hint={
              <span className="text-form-hint text-muted-foreground">
                Creates in{" "}
                <span className="font-medium text-fg">{currentProject?.name ?? "api"}</span> on{" "}
                <span className="font-medium text-fg">{computerDisplayName}</span>.
              </span>
            }
            onCancel={() => undefined}
            primaryDisabled={!ready}
            primaryIcon={Check}
            primaryLabel="Create automation"
            primaryTestId="automation-form-submit"
            primaryType="submit"
          />
        </form>
      </DialogContent>
    </Dialog>
  );

  return (
    <DraftShell
      app="dashboard"
      glyph={<AlarmClock />}
      overlay={overlay}
      scope="All projects"
      title="Automations"
      topbar={{ crumb: "Automations" }}
    >
      <div className="flex flex-1 flex-col gap-4 bg-canvas p-6">
        <div className="flex items-center justify-between border-b border-line pb-3">
          <div>
            <h1 className="text-heading font-medium text-fg">Scheduled jobs</h1>
            <p className="text-small-body text-muted-foreground">
              Jobs live on the computer you choose and run on its schedule.
            </p>
          </div>
          <Pill form="tint" tone="neutral">
            <FolderOpen aria-hidden="true" className="size-3" />
            All computers
          </Pill>
        </div>
        <div className="flex flex-col divide-y divide-line-soft rounded-lg bg-card shadow-card">
          <div className="flex items-center justify-between px-4 py-3 text-small-body">
            <div>
              <p className="font-medium text-fg">daily-dependency-audit</p>
              <p className="text-eyebrow text-subtle">
                Every weekday at 08:00 America/Los_Angeles · compozy · studio
              </p>
            </div>
            <Pill form="plain" tone="success">
              <Pill.Dot />
              Active
            </Pill>
          </div>
          <div className="flex items-center justify-between px-4 py-3 text-small-body">
            <div>
              <p className="font-medium text-fg">weekly-billing-reconcile</p>
              <p className="text-eyebrow text-subtle">
                Mondays at 06:00 America/Sao_Paulo · billing-worker · devbox
              </p>
            </div>
            <Pill form="plain" tone="success">
              <Pill.Dot />
              Active
            </Pill>
          </div>
        </div>
      </div>
    </DraftShell>
  );
}
