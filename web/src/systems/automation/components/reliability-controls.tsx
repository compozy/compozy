import { useState, type ReactNode } from "react";
import { ChevronRight } from "lucide-react";

import {
  cn,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Field,
  FieldContent,
  FieldDescription,
  FieldLabel,
  FieldTitle,
  Input,
  PillGroup,
  Switch,
} from "@agh/ui";

import { retryDraftForStrategy } from "../lib/automation-drafts";
import type { AutomationFireLimit, AutomationRetry } from "../types";

interface ReliabilitySectionShellProps {
  badge: string;
  children: ReactNode;
  defaultOpen: boolean;
  /**
   * Controlled disclosure. The automation editors swap their whole body for the
   * live preview, which unmounts this shell — they hold the open state above
   * that swap so returning from the preview does not collapse the section
   * someone was editing.
   */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  testId: string;
  title: string;
}

/** Shared disclosure anatomy for Job and Trigger reliability controls. */
export function ReliabilitySectionShell({
  badge,
  children,
  defaultOpen,
  open: controlledOpen,
  onOpenChange,
  testId,
  title,
}: ReliabilitySectionShellProps) {
  const [uncontrolledOpen, setUncontrolledOpen] = useState(defaultOpen);
  const open = controlledOpen ?? uncontrolledOpen;
  const setOpen = onOpenChange ?? setUncontrolledOpen;

  return (
    <Collapsible className="mt-5 border-t border-line-soft pt-1" onOpenChange={setOpen} open={open}>
      <CollapsibleTrigger
        className="flex w-full items-center gap-2 py-2.5 text-left outline-none focus-visible:shadow-focus-ring"
        data-testid={testId}
        type="button"
      >
        <ChevronRight
          aria-hidden="true"
          className={cn("size-4 text-muted transition-transform", open && "rotate-90")}
        />
        <span className="flex-1 text-small-body font-semibold text-fg-strong">{title}</span>
        <span className="font-mono text-form-hint text-subtle">{badge}</span>
      </CollapsibleTrigger>

      <CollapsibleContent className="grid grid-cols-2 gap-x-4 gap-y-4 pt-2 pb-1">
        {children}
      </CollapsibleContent>
    </Collapsible>
  );
}

interface ReliabilityRetryFieldsProps {
  idPrefix: "job" | "trigger";
  locked?: boolean;
  onChange: (retry: AutomationRetry) => void;
  retry: AutomationRetry;
}

/** Shared retry strategy and backoff fields. */
export function ReliabilityRetryFields({
  idPrefix,
  locked = false,
  onChange,
  retry,
}: ReliabilityRetryFieldsProps) {
  const isBackoff = retry.strategy === "backoff";
  const disabled = locked || !isBackoff;

  return (
    <>
      <Field className="col-span-2">
        <FieldTitle>
          Retry policy
          {locked ? <span className="font-normal text-faint">(owned by the task)</span> : null}
        </FieldTitle>
        <PillGroup
          aria-label="Retry policy"
          items={[
            {
              value: "none",
              label: "None",
              testId: `${idPrefix}-retry-strategy-none`,
              disabled: locked,
            },
            {
              value: "backoff",
              label: "Exponential backoff",
              testId: `${idPrefix}-retry-strategy-backoff`,
              disabled: locked,
            },
          ]}
          onChange={next => onChange(retryDraftForStrategy(next, retry))}
          size="sm"
          value={retry.strategy}
        />
      </Field>
      <Field>
        <FieldLabel htmlFor={`${idPrefix}-retry-max`}>Max retries</FieldLabel>
        <Input
          className="font-mono"
          data-testid={`${idPrefix}-retry-max`}
          disabled={disabled}
          id={`${idPrefix}-retry-max`}
          min={0}
          onChange={event =>
            onChange({
              ...retryDraftForStrategy("backoff", retry),
              max_retries: Number(event.target.value || "0"),
            })
          }
          type="number"
          value={isBackoff ? retry.max_retries : 0}
        />
      </Field>
      <Field>
        <FieldLabel htmlFor={`${idPrefix}-retry-delay`}>Base delay</FieldLabel>
        <Input
          className="font-mono"
          data-testid={`${idPrefix}-retry-delay`}
          disabled={disabled}
          id={`${idPrefix}-retry-delay`}
          onChange={event =>
            onChange({
              ...retryDraftForStrategy("backoff", retry),
              base_delay: event.target.value,
            })
          }
          placeholder="2s"
          value={isBackoff ? retry.base_delay : ""}
        />
      </Field>
    </>
  );
}

interface ReliabilityFireLimitFieldsProps {
  fireLimit: AutomationFireLimit | undefined;
  idPrefix: "job" | "trigger";
  onChange: (fireLimit: AutomationFireLimit) => void;
}

/** Shared fire-limit fields. */
export function ReliabilityFireLimitFields({
  fireLimit,
  idPrefix,
  onChange,
}: ReliabilityFireLimitFieldsProps) {
  return (
    <>
      <Field>
        <FieldLabel htmlFor={`${idPrefix}-fire-limit-max`}>Max fires</FieldLabel>
        <Input
          className="font-mono"
          data-testid={`${idPrefix}-fire-limit-max`}
          id={`${idPrefix}-fire-limit-max`}
          min={1}
          onChange={event =>
            onChange({
              ...(fireLimit ?? { window: "1h", max: 12 }),
              max: Number(event.target.value || "1"),
            })
          }
          type="number"
          value={fireLimit?.max ?? 12}
        />
      </Field>
      <Field>
        <FieldLabel htmlFor={`${idPrefix}-fire-limit-window`}>Per window</FieldLabel>
        <Input
          className="font-mono"
          data-testid={`${idPrefix}-fire-limit-window`}
          id={`${idPrefix}-fire-limit-window`}
          onChange={event =>
            onChange({
              ...(fireLimit ?? { max: 12, window: "1h" }),
              window: event.target.value,
            })
          }
          placeholder="1h"
          value={fireLimit?.window ?? "1h"}
        />
      </Field>
    </>
  );
}

interface ReliabilityEnabledFieldProps {
  description: string;
  enabled: boolean;
  idPrefix: "job" | "trigger";
  label: string;
  onChange: (enabled: boolean) => void;
}

/** Shared enabled control with a visible, programmatically associated label. */
export function ReliabilityEnabledField({
  description,
  enabled,
  idPrefix,
  label,
  onChange,
}: ReliabilityEnabledFieldProps) {
  const id = `${idPrefix}-enabled-toggle`;
  return (
    <Field className="col-span-2" orientation="horizontal">
      <Switch checked={enabled} data-testid={id} id={id} onCheckedChange={onChange} />
      <FieldContent>
        <FieldLabel htmlFor={id}>{label}</FieldLabel>
        <FieldDescription>{description}</FieldDescription>
      </FieldContent>
    </Field>
  );
}
