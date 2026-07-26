import {
  Field,
  FieldDescription,
  FieldLabel,
  FieldTitle,
  Input,
  PillGroup,
  type PillGroupItem,
} from "@agh/ui";

import type { AutomationCatchUpPolicy, AutomationFireLimit, AutomationRetry } from "../../types";
import {
  ReliabilityEnabledField,
  ReliabilityFireLimitFields,
  ReliabilityRetryFields,
  ReliabilitySectionShell,
} from "../reliability-controls";

/** Sentinel for the target-aware default: the daemon picks the policy, so the request omits it. */
const CATCH_UP_DEFAULT = "default";
type CatchUpChoice = AutomationCatchUpPolicy | typeof CATCH_UP_DEFAULT;

const CATCH_UP_ITEMS: PillGroupItem<CatchUpChoice>[] = [
  { value: CATCH_UP_DEFAULT, label: "Default", testId: "job-catch-up-default" },
  { value: "skip_missed", label: "Skip missed", testId: "job-catch-up-skip-missed" },
  { value: "coalesce", label: "Coalesce", testId: "job-catch-up-coalesce" },
  { value: "replay", label: "Replay", testId: "job-catch-up-replay" },
  { value: "run_once_on_catchup", label: "Run once", testId: "job-catch-up-run-once" },
];

/** How each catch-up choice reacts to fires missed while the runtime was down. */
const CATCH_UP_DESCRIPTIONS: Record<CatchUpChoice, string> = {
  [CATCH_UP_DEFAULT]: "Runtime picks the catch-up behavior for this target.",
  skip_missed: "Run the latest missed fire within grace; skip it beyond grace.",
  coalesce: "Collapse all missed fires into one catch-up run.",
  replay: "Run every missed fire in order.",
  run_once_on_catchup: "Run once to catch up, then resume the schedule.",
};

interface ReliabilitySectionProps {
  retry: AutomationRetry;
  fireLimit: AutomationFireLimit | undefined;
  enabled: boolean;
  locked: boolean;
  mode: "create" | "edit";
  badge: string;
  defaultOpen: boolean;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  /** True for cron/every schedules; catch-up + grace are recurring-only, hidden for one-shot `at`. */
  recurring: boolean;
  catchUpPolicy: AutomationCatchUpPolicy | undefined;
  misfireGraceSeconds: number | undefined;
  onRetryChange: (retry: AutomationRetry) => void;
  onFireLimitChange: (fireLimit: AutomationFireLimit) => void;
  onEnabledChange: (enabled: boolean) => void;
  /** `undefined` selects the target-aware default (omitted from the request). */
  onCatchUpPolicyChange: (policy: AutomationCatchUpPolicy | undefined) => void;
  /** `undefined` (zero/empty) applies the scheduler's default jitter grace. */
  onMisfireGraceChange: (seconds: number | undefined) => void;
}

/**
 * Collapsible reliability & limits controls (retry, rate limit, enabled). When a
 * job delegates to a task, the task owns retries — `locked` disables the retry
 * controls and surfaces an "owned by the task" hint.
 */
export function ReliabilitySection({
  retry,
  fireLimit,
  enabled,
  locked,
  mode,
  badge,
  defaultOpen,
  open,
  onOpenChange,
  recurring,
  catchUpPolicy,
  misfireGraceSeconds,
  onRetryChange,
  onFireLimitChange,
  onEnabledChange,
  onCatchUpPolicyChange,
  onMisfireGraceChange,
}: ReliabilitySectionProps) {
  const catchUpValue: CatchUpChoice = catchUpPolicy ?? CATCH_UP_DEFAULT;

  return (
    <ReliabilitySectionShell
      badge={badge}
      defaultOpen={defaultOpen}
      onOpenChange={onOpenChange}
      open={open}
      testId="job-governance-toggle"
      title="Reliability & limits"
    >
      <ReliabilityRetryFields
        idPrefix="job"
        locked={locked}
        onChange={onRetryChange}
        retry={retry}
      />
      <ReliabilityFireLimitFields
        fireLimit={fireLimit}
        idPrefix="job"
        onChange={onFireLimitChange}
      />
      {recurring ? (
        <>
          <Field className="col-span-2" data-testid="job-catch-up-field">
            <FieldTitle>Catch-up policy</FieldTitle>
            <PillGroup
              aria-label="Catch-up policy"
              items={CATCH_UP_ITEMS}
              onChange={next => onCatchUpPolicyChange(next === CATCH_UP_DEFAULT ? undefined : next)}
              size="sm"
              value={catchUpValue}
            />
            <FieldDescription>{CATCH_UP_DESCRIPTIONS[catchUpValue]}</FieldDescription>
          </Field>
          <Field>
            <FieldLabel htmlFor="job-misfire-grace">Grace window</FieldLabel>
            <Input
              className="font-mono"
              data-testid="job-misfire-grace"
              id="job-misfire-grace"
              inputMode="numeric"
              min={0}
              step={1}
              onChange={event => {
                // Store the entered value as-is (no flooring); serialization keeps
                // it only when it is a positive whole number of seconds.
                const raw = event.target.value;
                const seconds = Number(raw);
                onMisfireGraceChange(raw === "" || Number.isNaN(seconds) ? undefined : seconds);
              }}
              placeholder="0"
              type="number"
              value={misfireGraceSeconds ?? ""}
            />
            <FieldDescription>
              Applies only when the effective policy is Skip missed. Whole seconds the latest missed
              fire may still run; 0 or empty uses the scheduler&apos;s default grace.
            </FieldDescription>
          </Field>
        </>
      ) : null}
      <ReliabilityEnabledField
        description="Disabled jobs stay stored but never dispatch on their schedule."
        enabled={enabled}
        idPrefix="job"
        label={mode === "create" ? "Enabled on create" : "Enabled"}
        onChange={onEnabledChange}
      />
    </ReliabilitySectionShell>
  );
}
