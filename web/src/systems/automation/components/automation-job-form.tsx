import { Bot, CalendarCheck, Check, Clock, Eye, Repeat, Repeat2, Workflow } from "lucide-react";
import { useState } from "react";

import {
  Alert,
  AlertDescription,
  Button,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogToolbar,
  Field,
  FieldLabel,
  FormSection,
  Input,
  PillGroup,
  type PillGroupItem,
} from "@agh/ui";
import type { AgentPayload } from "@/systems/agent";
import { ScopeSelector } from "@/systems/workspace";
import { LoopTargetFields } from "@/systems/loops";

import { useAutomationJobForm, type JobTargetMode } from "../hooks/use-automation-job-form";
import { catchUpPolicyLabel } from "../lib/automation-formatters";
import type { WorkspaceOption } from "../lib/trigger-preview";
import type {
  AutomationCatchUpPolicy,
  AutomationFireLimit,
  AutomationRetry,
  AutomationScheduleMode,
  CreateAutomationJobRequest,
} from "../types";
import { AgentRunStep } from "./job-form/agent-run-step";
import { CronBuilder } from "./job-form/cron-builder";
import { JobPreview } from "./job-form/preview/job-preview";
import { ReliabilitySection } from "./job-form/reliability-section";
import { ScheduleAt } from "./job-form/schedule-at";
import { ScheduleEvery } from "./job-form/schedule-every";
import { TaskRunStep } from "./job-form/task-run-step";

interface AutomationJobFormProps {
  activeWorkspaceId?: string | null;
  draft: CreateAutomationJobRequest;
  isPending: boolean;
  mode: "create" | "edit";
  onCancel: () => void;
  onChange: (draft: CreateAutomationJobRequest) => void;
  onSubmit: () => void;
  /** Workspaces selectable for a workspace-scoped job. */
  workspaces?: ReadonlyArray<WorkspaceOption>;
  /** Agent catalog for the searchable selector. */
  agents?: AgentPayload[];
  agentsLoading?: boolean;
  agentsError?: string | null;
}

const EMPTY_AGENTS: AgentPayload[] = [];

const JOB_TARGET_ITEMS: PillGroupItem<JobTargetMode>[] = [
  {
    value: "agent",
    label: (
      <span className="flex items-center gap-1.5">
        <Bot aria-hidden="true" className="size-3" />
        Run agent
      </span>
    ),
    testId: "job-target-agent",
  },
  {
    value: "task",
    label: (
      <span className="flex items-center gap-1.5">
        <Workflow aria-hidden="true" className="size-3" />
        Run task
      </span>
    ),
    testId: "job-target-task",
  },
  {
    value: "loop",
    label: (
      <span className="flex items-center gap-1.5">
        <Repeat2 aria-hidden="true" className="size-3" />
        Run loop
      </span>
    ),
    testId: "job-target-loop",
  },
];

const SCHEDULE_MODE_ITEMS: PillGroupItem<AutomationScheduleMode>[] = [
  {
    value: "cron",
    label: (
      <span className="flex items-center gap-1.5">
        <Repeat aria-hidden="true" className="size-3" />
        Repeat on cron
      </span>
    ),
    testId: "job-schedule-mode-cron",
  },
  {
    value: "every",
    label: (
      <span className="flex items-center gap-1.5">
        <Clock aria-hidden="true" className="size-3" />
        Every interval
      </span>
    ),
    testId: "job-schedule-mode-every",
  },
  {
    value: "at",
    label: (
      <span className="flex items-center gap-1.5">
        <CalendarCheck aria-hidden="true" className="size-3" />
        Once at a time
      </span>
    ),
    testId: "job-schedule-mode-at",
  },
];

/** Job-level reliability badge mirroring the artboard's collapsible summary. */
function reliabilityBadge(
  retry: AutomationRetry,
  fireLimit: AutomationFireLimit | undefined,
  enabled: boolean,
  locked: boolean,
  catchUpPolicy: AutomationCatchUpPolicy | undefined
): string {
  const retryLabel =
    locked || retry.strategy === "none" ? "No retry" : `Backoff ×${retry.max_retries}`;
  const limit = fireLimit ?? { max: 12, window: "1h" };
  // Only recurring jobs pass a policy; the default (omitted) stays out of the summary.
  const catchUp = catchUpPolicy ? ` · ${catchUpPolicyLabel(catchUpPolicy)}` : "";
  return `${retryLabel} · ${limit.max}/${limit.window}${catchUp} · ${enabled ? "enabled" : "disabled"}`;
}

export function AutomationJobForm({
  activeWorkspaceId,
  draft,
  isPending,
  mode,
  onCancel,
  onChange,
  onSubmit,
  workspaces,
  agents = EMPTY_AGENTS,
  agentsLoading = false,
  agentsError = null,
}: AutomationJobFormProps) {
  const form = useAutomationJobForm({
    activeWorkspaceId,
    draft,
    isPending,
    mode,
    onChange,
    onSubmit,
    workspaces,
    agents,
  });
  // View state, not draft state: `AutomationEditorDialog` unmounts the content
  // on close, so the initial value is the whole reset.
  const [view, setView] = useState<"form" | "preview">("form");
  // Held above the view swap: the preview unmounts the form body, and a
  // disclosure someone opened must survive a look at the preview.
  const [reliabilityOpen, setReliabilityOpen] = useState(form.reliabilityDefaultOpen);

  return (
    <form
      className="flex min-h-0 flex-col"
      data-testid="automation-job-form"
      onSubmit={form.handleSubmit}
    >
      <EntityDialogToolbar
        trailing={
          <ScopeSelector
            ariaLabel="Job scope"
            disabled={mode === "edit"}
            onScopeChange={form.onScopeChange}
            onWorkspaceChange={form.onWorkspaceChange}
            scope={draft.scope}
            testIdPrefix="job"
            workspaceId={draft.workspace_id}
            workspaces={form.resolvedWorkspaces}
          />
        }
        trailingLabel="Scope"
      />

      <EntityDialogBody data-testid="automation-job-form-body">
        {view === "preview" ? (
          <JobPreview preview={form.preview} />
        ) : (
          <>
            {form.preview.targetIssue ? (
              // With the preview closed this is the only visible reason the
              // primary is disabled — never let it live solely in the preview.
              <Alert className="mb-4" data-testid="job-form-blocked" variant="warning">
                <AlertDescription>{form.preview.targetIssue}</AlertDescription>
              </Alert>
            ) : null}
            <Field>
              <FieldLabel htmlFor="job-name">Job name</FieldLabel>
              <Input
                className="font-mono"
                data-testid="job-name-input"
                id="job-name"
                onChange={event => form.onName(event.target.value)}
                placeholder="daily-code-review"
                value={draft.name}
              />
            </Field>

            <FormSection
              help="Prompt an agent, hand the work to a durable task, or start a Loop with typed inputs."
              icon={Bot}
              title="What fires on each tick"
            >
              <div className="space-y-4">
                <PillGroup
                  aria-label="Target"
                  className="max-w-full flex-wrap"
                  items={JOB_TARGET_ITEMS.map(item => ({ ...item, disabled: mode === "edit" }))}
                  onChange={form.onTargetChange}
                  size="sm"
                  value={form.targetMode}
                />
                {form.targetMode === "loop" ? (
                  <LoopTargetFields
                    catalog={form.loopCatalog}
                    identityDisabled={mode === "edit"}
                    mode={mode}
                    value={form.loopTarget}
                    onChange={form.onLoopTargetChange}
                  />
                ) : form.targetMode === "task" && draft.task ? (
                  <TaskRunStep
                    disabled={isPending}
                    jobName={draft.name}
                    onOwnerKind={form.onOwnerKind}
                    onOwnerRef={form.onOwnerRef}
                    onNetworkParticipationChange={form.onTaskNetworkParticipation}
                    onTaskDescription={form.onTaskDescription}
                    onTaskTitle={form.onTaskTitle}
                    task={draft.task}
                  />
                ) : (
                  <AgentRunStep
                    agent={draft.agent_name}
                    agentDisabled={mode === "edit"}
                    agents={form.agents}
                    agentsError={agentsError}
                    agentsLoading={agentsLoading}
                    onAgentChange={form.onAgentChange}
                    onPromptChange={form.onPromptChange}
                    prompt={draft.prompt}
                  />
                )}
              </div>
            </FormSection>

            <FormSection
              description="All times evaluate in UTC, the runtime's automation timezone."
              icon={Clock}
              title="On this schedule"
            >
              <div className="space-y-4">
                <PillGroup
                  aria-label="Schedule mode"
                  className="max-w-full flex-wrap"
                  items={SCHEDULE_MODE_ITEMS}
                  onChange={form.onScheduleMode}
                  value={form.scheduleMode}
                />
                {form.scheduleMode === "cron" ? (
                  <CronBuilder
                    expr={draft.schedule.expr ?? ""}
                    model={form.cronModel}
                    onDailyTime={form.onDailyTime}
                    onEveryMinutes={form.onEveryMinutes}
                    onExpr={form.onCronExpr}
                    onFrequency={form.onCronFrequency}
                    onHourlyMinute={form.onHourlyMinute}
                    onMonthDay={form.onMonthDay}
                    onMonthlyTime={form.onMonthlyTime}
                    onPreset={form.onCronPreset}
                    onToggleWeekday={form.onToggleWeekday}
                    onWeeklyTime={form.onWeeklyTime}
                    onWeekdayPreset={form.onWeekdayPreset}
                    readout={form.preview.scheduleReadout}
                    valid={form.preview.scheduleValid}
                  />
                ) : null}
                {form.scheduleMode === "every" ? (
                  <ScheduleEvery
                    interval={draft.schedule.interval ?? ""}
                    onInterval={form.onEveryInterval}
                    onPreset={form.onEveryPreset}
                    readout={form.preview.scheduleReadout}
                    valid={form.preview.scheduleValid}
                  />
                ) : null}
                {form.scheduleMode === "at" ? (
                  <ScheduleAt
                    onTime={form.onAtTime}
                    readout={form.preview.scheduleReadout}
                    time={draft.schedule.time ?? ""}
                    valid={form.preview.scheduleValid}
                  />
                ) : null}
              </div>
            </FormSection>

            <ReliabilitySection
              badge={reliabilityBadge(
                form.retry,
                draft.fire_limit ?? undefined,
                draft.enabled ?? true,
                form.output === "task",
                form.recurring ? form.catchUpPolicy : undefined
              )}
              catchUpPolicy={form.catchUpPolicy}
              defaultOpen={form.reliabilityDefaultOpen}
              onOpenChange={setReliabilityOpen}
              open={reliabilityOpen}
              enabled={draft.enabled ?? true}
              fireLimit={draft.fire_limit ?? undefined}
              locked={form.output === "task"}
              misfireGraceSeconds={form.misfireGraceSeconds}
              mode={mode}
              onCatchUpPolicyChange={form.onCatchUpPolicyChange}
              onEnabledChange={form.onEnabledChange}
              onFireLimitChange={form.onFireLimitChange}
              onMisfireGraceChange={form.onMisfireGraceChange}
              onRetryChange={form.onRetryChange}
              recurring={form.recurring}
              retry={form.retry}
            />
          </>
        )}
      </EntityDialogBody>

      <EntityDialogFooter
        hint={
          <>
            Created as a <b className="font-medium text-muted">dynamic</b> job: editable,
            disable-able, and deletable anytime.
          </>
        }
        isSaving={isPending}
        leading={
          <Button
            aria-pressed={view === "preview"}
            data-testid="job-preview-toggle"
            onClick={() => setView(current => (current === "form" ? "preview" : "form"))}
            size="sm"
            type="button"
            variant="ghost"
          >
            <Eye aria-hidden="true" className="size-3.5" />
            {view === "preview" ? "Back to form" : "Show live preview"}
          </Button>
        }
        onCancel={onCancel}
        primaryDisabled={!form.canSubmit}
        primaryIcon={mode === "create" ? Check : undefined}
        primaryLabel={isPending ? "Saving..." : mode === "create" ? "Create job" : "Save changes"}
        primaryTestId="submit-job-form"
        primaryType="submit"
      />
    </form>
  );
}
