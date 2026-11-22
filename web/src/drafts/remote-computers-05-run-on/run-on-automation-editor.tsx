import { AlarmClock, Check, Clock, FolderOpen, Info, TriangleAlert } from "@compozy/ui/icons";
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
  Textarea,
} from "@compozy/ui";

import { AutomationFormSection } from "@/systems/automation/components/automation-form/automation-form-section";
import { RunOnPicker, type RunOnTarget } from "@/systems/computers/components/run-on-picker";
import { COMPUTERS_PRO, LOCAL_COMPUTER } from "@/systems/computers/mocks/remote-computers-fixtures";
import { projectsForTarget } from "@/systems/computers/mocks/remote-computers-fixtures-sessions";

import { DraftShell } from "../always-on-agents-shared/draft-shell";

/**
 * Screen 05 Host C — Automation Job Editor with concrete `Run on` picker (`allowAuto={false}`)
 * immediately before Project and the exact persistence hint (`_uiux.md` U4, US-009 AC-4):
 * "This job is saved on <name> and runs there, even when this app is closed."
 */
export function RunOnAutomationEditorScene({
  initialTarget = "cmp_7f3k2q9x",
  pickerDefaultOpen = false,
}: {
  initialTarget?: RunOnTarget;
  pickerDefaultOpen?: boolean;
}) {
  const [target, setTarget] = useState<RunOnTarget>(initialTarget);
  const [name, setName] = useState("nightly-pg-pool-smoke");
  const [scheduleTime, setScheduleTime] = useState("02:00");
  const [prompt, setPrompt] = useState(
    "Run go test -race -count=10 ./internal/pgpool/... and summarize any flake or regression."
  );

  const projects = useMemo(() => projectsForTarget(target), [target]);
  const [projectName, setProjectName] = useState(projects[0]?.name ?? "api");
  const currentProject = projects.find(p => p.name === projectName) ?? projects[0] ?? null;

  const selectedRemote = COMPUTERS_PRO.find(c => c.id === target) ?? null;
  const computerDisplayName =
    target === "local" ? LOCAL_COMPUTER.name : (selectedRemote?.name ?? "devbox");
  const isOffline = selectedRemote?.status === "offline";

  const isReady = !isOffline && Boolean(name.trim());

  const overlay = (
    <Dialog onOpenChange={() => undefined} open>
      <DialogContent
        className={`grid-rows-[auto_auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("md")}`}
        data-testid="automation-job-editor-dialog"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          eyebrow="Automations"
          icon={AlarmClock}
          onClose={() => undefined}
          title="New scheduled job"
        />

        <EntityDialogToolbar
          className="flex-nowrap border-b border-line-soft bg-sunken"
          data-testid="automation-editor-sentence-bar"
          leading={
            <p
              aria-live="polite"
              className="min-w-0 flex-1 text-small-body leading-snug text-muted-foreground"
            >
              Every day at <span className="font-medium text-fg">{scheduleTime}</span>, ask{" "}
              <span className="font-medium text-fg">general</span> in{" "}
              <span className="font-medium text-fg">{currentProject?.name ?? "api"}</span> on{" "}
              <span className="font-medium text-fg">{computerDisplayName}</span>.
            </p>
          }
          trailing={
            <Pill tone={isReady ? "success" : "warning"}>
              {isReady ? <Check aria-hidden="true" /> : <TriangleAlert aria-hidden="true" />}
              {isReady ? "Ready" : "Needs a fix"}
            </Pill>
          }
        />

        <EntityDialogBody className="flex flex-col gap-5">
          <AutomationFormSection
            description="How you'll find it in the list"
            number={1}
            title="Name"
          >
            <Field>
              <FieldLabel className="sr-only" htmlFor="auto-job-name">
                Job name
              </FieldLabel>
              <Input
                className="font-mono"
                id="auto-job-name"
                onChange={event => setName(event.target.value)}
                value={name}
              />
            </Field>
          </AutomationFormSection>

          <AutomationFormSection
            description="Jobs belong to one concrete computer and always run there."
            number={2}
            title="Where it runs"
          >
            <div className="flex flex-col gap-3">
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <Field>
                  <FieldLabel>Run on</FieldLabel>
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
                      if (nextProjects[0]) setProjectName(nextProjects[0].name);
                    }}
                    value={target}
                  />
                </Field>

                <Field>
                  <FieldLabel htmlFor="auto-job-project">Project</FieldLabel>
                  <NativeSelect
                    id="auto-job-project"
                    onChange={event => setProjectName(event.target.value)}
                    value={currentProject?.name ?? ""}
                  >
                    {projects.map(project => (
                      <NativeSelectOption key={project.id} value={project.name}>
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
                className="flex items-start gap-2 rounded-md border border-line-soft bg-sunken px-3 py-2 text-eyebrow text-muted-foreground"
                data-testid="automation-computer-ownership-hint"
              >
                <Info aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-subtle" />
                <span>
                  This job is saved on{" "}
                  <span className="font-medium text-fg">{computerDisplayName}</span> and runs there,
                  even when this app is closed.
                </span>
              </p>
            </div>
          </AutomationFormSection>

          <AutomationFormSection
            description="When this computer fires the job"
            number={3}
            title="Starts"
          >
            <div className="flex flex-wrap items-center gap-2.5">
              <Pill form="tint" tone="neutral">
                <Clock aria-hidden="true" className="size-3" />
                Every day
              </Pill>
              <Input
                aria-label="Run time"
                className="w-36 font-mono tabular-nums"
                onChange={event => setScheduleTime(event.target.value)}
                type="time"
                value={scheduleTime}
              />
              <span className="text-eyebrow text-subtle">
                {computerDisplayName} local time (UTC)
              </span>
            </div>
          </AutomationFormSection>

          <AutomationFormSection
            description="What the agent receives when the job fires"
            number={4}
            title="Does"
          >
            <Field>
              <FieldLabel htmlFor="auto-job-prompt">Prompt for general</FieldLabel>
              <Textarea
                id="auto-job-prompt"
                onChange={event => setPrompt(event.target.value)}
                rows={3}
                value={prompt}
              />
            </Field>
          </AutomationFormSection>
        </EntityDialogBody>

        <EntityDialogFooter
          onCancel={() => undefined}
          primaryDisabled={isOffline || !name.trim()}
          primaryIcon={Check}
          primaryLabel="Create automation"
        />
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
              <p className="text-eyebrow text-subtle">Every weekday at 08:00 · compozy · studio</p>
            </div>
            <Pill form="plain" tone="success">
              <Pill.Dot />
              Active
            </Pill>
          </div>
          <div className="flex items-center justify-between px-4 py-3 text-small-body">
            <div>
              <p className="font-medium text-fg">weekly-billing-reconcile</p>
              <p className="text-eyebrow text-subtle">Mondays at 06:00 · billing-worker · devbox</p>
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
