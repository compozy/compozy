import { ChevronRight } from "lucide-react";
import type { ReactNode, Ref } from "react";

import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Input,
  NativeSelect,
  NativeSelectOption,
  PillGroup,
  Switch,
  cn,
} from "@compozy/ui";

import type { AutomationFormModel } from "../../hooks/use-automation-form";
import { retryDraftForStrategy } from "../../lib/automation-drafts";
import type { AutomationFormDraft } from "../../lib/automation-form-draft";
import type { AutomationCatchUpPolicy } from "../../types";

const LOOP_DEFAULT = "loop-default";
type MissedChoice = AutomationCatchUpPolicy | typeof LOOP_DEFAULT;

interface AutomationOptionsProps {
  draft: AutomationFormDraft;
  form: AutomationFormModel;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  ref?: Ref<HTMLDivElement>;
}

/**
 * The Options fold: retries, run limit, missed runs (schedules only) and
 * turn-on, closed by default behind a mono summary of the current values.
 */
export function AutomationOptions({
  draft,
  form,
  open,
  onOpenChange,
  ref,
}: AutomationOptionsProps) {
  const retry = form.retry;
  const taskTarget = form.does === "task";
  const recurring = draft.start === "schedule" && draft.schedule.mode !== "at";

  return (
    <Collapsible
      className="mt-7 border-t border-line pt-3"
      data-testid="automation-form-options"
      onOpenChange={onOpenChange}
      open={open}
      ref={ref}
    >
      <CollapsibleTrigger
        className="flex w-full items-center gap-2 py-2 text-left outline-none focus-visible:shadow-focus-ring"
        data-testid="automation-options-toggle"
        type="button"
      >
        <ChevronRight
          aria-hidden="true"
          className={cn("size-4 text-muted transition-transform", open && "rotate-90")}
        />
        <span className="text-small-body font-semibold text-fg-strong">Options</span>
        <span
          className="ml-auto truncate font-mono text-form-hint text-subtle"
          data-testid="automation-options-summary"
        >
          {form.optionsSummary}
        </span>
      </CollapsibleTrigger>
      <CollapsibleContent className="flex flex-col divide-y divide-line-soft pt-1">
        <OptionRow
          description={
            taskTarget
              ? "The task handles its own retries."
              : retry.strategy === "backoff"
                ? `Up to ${retry.max_retries} times, first after ${retry.base_delay}, then longer.`
                : "Try again, waiting longer each time."
          }
          label="Retry if it fails"
        >
          <PillGroup
            aria-label="Retry if it fails"
            items={[
              { value: "none", label: "No", disabled: taskTarget, testId: "automation-retry-none" },
              {
                value: "backoff",
                label: `Up to ${retryDraftForStrategy("backoff", retry).max_retries} times`,
                disabled: taskTarget,
                testId: "automation-retry-backoff",
              },
            ]}
            onChange={next => form.onRetryChange(retryDraftForStrategy(next, retry))}
            size="sm"
            value={taskTarget ? "none" : retry.strategy}
          />
        </OptionRow>
        {retry.strategy === "backoff" && !taskTarget ? (
          <>
            <OptionRow label="Times" description="How many retries before it gives up.">
              <Input
                aria-label="Max retries"
                className="w-20 font-mono"
                data-testid="automation-retry-max"
                min={1}
                onChange={event =>
                  form.onRetryChange({ ...retry, max_retries: Number(event.target.value || "0") })
                }
                type="number"
                value={retry.max_retries}
              />
            </OptionRow>
            <OptionRow label="First wait" description="How long before the first retry.">
              <Input
                aria-label="Base delay"
                className="w-20 font-mono"
                data-testid="automation-retry-delay"
                onChange={event => form.onRetryChange({ ...retry, base_delay: event.target.value })}
                placeholder="2s"
                value={retry.base_delay}
              />
            </OptionRow>
          </>
        ) : null}
        <OptionRow label="Run limit" description="Stops a runaway automation.">
          <Input
            aria-label="Max runs"
            className="w-20 font-mono"
            data-testid="automation-fire-limit-max"
            min={1}
            onChange={event =>
              form.onFireLimitChange({
                ...fireLimit(draft),
                max: Number(event.target.value || "1"),
              })
            }
            type="number"
            value={fireLimit(draft).max}
          />
          <span className="font-mono text-form-hint text-subtle">per</span>
          <Input
            aria-label="Window"
            className="w-20 font-mono"
            data-testid="automation-fire-limit-window"
            onChange={event =>
              form.onFireLimitChange({ ...fireLimit(draft), window: event.target.value })
            }
            placeholder="1h"
            value={fireLimit(draft).window}
          />
        </OptionRow>
        {recurring ? <MissedRunsRow draft={draft} form={form} /> : null}
        <OptionRow label="Turn on after saving" description="Off keeps it saved without running.">
          <Switch
            aria-label="Turn on after saving"
            checked={draft.enabled ?? true}
            data-testid="automation-enabled-toggle"
            onCheckedChange={form.onEnabledChange}
          />
        </OptionRow>
      </CollapsibleContent>
    </Collapsible>
  );
}

function fireLimit(draft: AutomationFormDraft) {
  return draft.fire_limit ?? { max: 12, window: "1h" };
}

function MissedRunsRow({ draft, form }: { draft: AutomationFormDraft; form: AutomationFormModel }) {
  const policy = draft.schedule.catch_up_policy;
  const loopTarget = form.does === "loop";
  const value: MissedChoice = policy ?? (loopTarget ? LOOP_DEFAULT : "skip_missed");
  const skipping = value === "skip_missed";

  return (
    <OptionRow
      description="What to do with runs it missed."
      label="If CompozyOS was off at the start time"
    >
      <NativeSelect
        aria-label="Missed runs"
        className="w-52"
        data-testid="automation-missed-runs"
        onChange={event => {
          const next = event.target.value as MissedChoice;
          form.onCatchUpPolicyChange(next === LOOP_DEFAULT ? undefined : next);
        }}
        value={value}
      >
        {loopTarget ? (
          <NativeSelectOption value={LOOP_DEFAULT}>Use the Loop&apos;s default</NativeSelectOption>
        ) : null}
        <NativeSelectOption value="skip_missed">Skip them</NativeSelectOption>
        <NativeSelectOption value="coalesce">Run once to catch up</NativeSelectOption>
        <NativeSelectOption value="replay">Run every missed time</NativeSelectOption>
        {policy === "run_once_on_catchup" ? (
          <NativeSelectOption value="run_once_on_catchup">
            Run once, then continue
          </NativeSelectOption>
        ) : null}
      </NativeSelect>
      {skipping ? (
        <>
          <span className="font-mono text-form-hint text-subtle">wait up to</span>
          <Input
            aria-label="Late start limit, seconds"
            className="w-16 font-mono"
            data-testid="automation-misfire-grace"
            inputMode="numeric"
            min={0}
            onChange={event => {
              // Kept as typed; the request keeps it only as a positive whole number.
              const raw = event.target.value;
              const seconds = Number(raw);
              form.onMisfireGraceChange(raw === "" || Number.isNaN(seconds) ? undefined : seconds);
            }}
            placeholder="0"
            step={1}
            type="number"
            value={draft.schedule.misfire_grace_seconds ?? ""}
          />
          <span className="font-mono text-form-hint text-subtle">s</span>
        </>
      ) : null}
    </OptionRow>
  );
}

function OptionRow({
  label,
  description,
  children,
}: {
  label: string;
  description?: string;
  children: ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 py-3">
      <div className="min-w-0">
        <div className="text-small-body font-medium text-fg">{label}</div>
        {description ? <div className="text-form-hint text-muted">{description}</div> : null}
      </div>
      <div className="flex shrink-0 items-center gap-2">{children}</div>
    </div>
  );
}
