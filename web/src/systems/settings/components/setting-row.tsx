import { mergeProps } from "@base-ui/react/merge-props";
import { useRender } from "@base-ui/react/use-render";
import { ChevronRight } from "lucide-react";
import { useId, type ComponentProps, type ReactNode } from "react";

import {
  cn,
  Field,
  FieldContent,
  FieldDescription,
  FieldError,
  FieldHeader,
  FieldLabel,
  HelpTip,
} from "@agh/ui";
import { associateSettingsControl } from "../lib/control-association";

export interface SettingRowProps {
  /** Plain-language decision name (13px/500). */
  label: ReactNode;
  /** The consequence sentence — what this does to the user's work (≤52ch). */
  description?: ReactNode;
  /**
   * Explanation, moved into a `HelpTip` beside the label. Modal rows only —
   * `description` stays for what the runtime will do with the value.
   */
  help?: ReactNode;
  error?: ReactNode;
  /** Single control, right-aligned. Labelled via aria association automatically. */
  control?: ReactNode;
  className?: string;
  "data-testid"?: string;
}

function settingRowIds(testId: string | undefined, fallbackId: string) {
  const baseId = testId?.trim().replace(/[^a-zA-Z0-9_-]+/g, "-") || `setting-row-${fallbackId}`;
  return {
    baseId,
    labelId: `${baseId}-label`,
  };
}

/**
 * One row = one decision (settings design system §05). Label + consequence
 * left, one control right, 54px minimum, hairline separators inside a
 * panelbox. Jargon chips are banned here — provenance lives in Advanced.
 */
export function SettingRow({
  label,
  description,
  error,
  control,
  className,
  "data-testid": testId,
}: SettingRowProps) {
  const fallbackId = useId().replace(/:/g, "");
  const { baseId, labelId } = settingRowIds(testId, fallbackId);
  const descriptionId = description ? `${baseId}-description` : undefined;
  const errorId = error ? `${baseId}-error` : undefined;
  const { control: renderedControl, labelHtmlFor } = associateSettingsControl({
    control,
    controlId: `${baseId}-control`,
    labelId,
    descriptionId,
    errorId,
  });

  const LabelTag = labelHtmlFor ? "label" : "span";

  return (
    <div
      className={cn(
        "flex min-h-setting-row items-center justify-between gap-5 border-t border-line-soft px-4 py-3 first:border-t-0",
        className
      )}
      data-slot="setting-row"
      data-testid={testId}
    >
      <div className="min-w-0 flex-1">
        <LabelTag
          className="flex items-center gap-1.5 text-ws-name font-medium text-fg-strong"
          htmlFor={labelHtmlFor}
          id={labelId}
        >
          {label}
        </LabelTag>
        {description ? (
          <p
            className="mt-0.5 max-w-setting-description text-form-label leading-normal text-muted"
            id={descriptionId}
          >
            {description}
          </p>
        ) : null}
        {error ? (
          <p className="mt-1 text-form-hint text-danger" id={errorId} role="alert">
            {error}
          </p>
        ) : null}
      </div>
      {renderedControl ? (
        <div className="flex shrink-0 items-center gap-2">{renderedControl}</div>
      ) : null}
    </div>
  );
}

/**
 * Modal/dialog form of SettingRow — a stacked `Field` with no chrome of its own.
 *
 * Rules and vertical rhythm belong to the enclosing `FormSection`; this row drew
 * its own hairlines and a `text-sm`/`text-xs` type fork, which stacked a second
 * rule system inside every settings modal.
 */
export function ModalSettingRow({
  label,
  description,
  help,
  error,
  control,
  className,
  "data-testid": testId,
}: SettingRowProps) {
  const fallbackId = useId().replace(/:/g, "");
  const { baseId, labelId } = settingRowIds(testId, fallbackId);
  const descriptionId = description ? `${baseId}-description` : undefined;
  const errorId = error ? `${baseId}-error` : undefined;
  const { control: renderedControl, labelHtmlFor } = associateSettingsControl({
    control,
    controlId: `${baseId}-control`,
    labelId,
    descriptionId,
    errorId,
  });

  return (
    <Field className={className} data-testid={testId} orientation="vertical">
      <FieldContent className="min-w-0 gap-1.5">
        <FieldHeader>
          <FieldLabel
            data-testid={testId ? `${testId}-label` : undefined}
            htmlFor={labelHtmlFor}
            id={labelId}
          >
            {label}
          </FieldLabel>
          {help ? <HelpTip label={helpTriggerLabel(label)}>{help}</HelpTip> : null}
        </FieldHeader>
        {description ? <FieldDescription id={descriptionId}>{description}</FieldDescription> : null}
      </FieldContent>
      <div className="flex w-full min-w-0 max-w-full flex-wrap items-center gap-3 [&_input]:max-w-full [&_select]:max-w-full">
        {renderedControl}
      </div>
      {error ? <FieldError id={errorId}>{error}</FieldError> : null}
    </Field>
  );
}

/** Names the help trigger after its row when the label is plain text. */
function helpTriggerLabel(label: ReactNode) {
  return typeof label === "string" ? `About ${label.toLowerCase()}` : "More information";
}

interface SettingNavigationRowContentProps {
  label: ReactNode;
  description?: ReactNode;
  /** Optional trailing value before the chevron. */
  value?: ReactNode;
}

export type SettingLinkRowProps = Omit<useRender.ComponentProps<"a">, "children"> &
  SettingNavigationRowContentProps;

/**
 * Row that leads somewhere else — a sheet, a top-level route, a marketplace.
 * The caller supplies its router-aware anchor through `render`; this component
 * owns only the stable presentation and semantics of the row.
 */
export function SettingLinkRow({
  label,
  description,
  value,
  className,
  render,
  ...props
}: SettingLinkRowProps) {
  return useRender({
    defaultTagName: "a",
    props: mergeProps<"a">(
      {
        className: cn(
          "flex min-h-setting-row w-full items-center justify-between gap-5 border-t border-line-soft px-4 py-3 text-left first:border-t-0",
          "cursor-pointer transition-colors duration-base hover:bg-row-hover",
          "focus-visible:outline-none focus-visible:shadow-focus-ring",
          className
        ),
        children: (
          <>
            <span className="min-w-0 flex-1">
              <span className="flex items-center gap-1.5 text-ws-name font-medium text-fg-strong">
                {label}
              </span>
              {description ? (
                <span className="mt-0.5 block max-w-setting-description text-form-label leading-normal text-muted">
                  {description}
                </span>
              ) : null}
            </span>
            <span className="flex shrink-0 items-center gap-2">
              {value}
              <ChevronRight aria-hidden="true" className="size-3.5 text-faint" />
            </span>
          </>
        ),
      },
      props
    ),
    render,
    state: {
      slot: "setting-link-row",
    },
  });
}

export interface SettingActionRowProps
  extends Omit<ComponentProps<"button">, "children" | "value">, SettingNavigationRowContentProps {}

/** Row that performs an in-place action, such as opening a settings sheet. */
export function SettingActionRow({
  label,
  description,
  value,
  className,
  ...props
}: SettingActionRowProps) {
  return (
    <button
      className={cn(
        "flex min-h-setting-row w-full items-center justify-between gap-5 border-t border-line-soft px-4 py-3 text-left first:border-t-0",
        "cursor-pointer transition-colors duration-base hover:bg-row-hover",
        "focus-visible:outline-none focus-visible:shadow-focus-ring",
        className
      )}
      data-slot="setting-action-row"
      type="button"
      {...props}
    >
      <span className="min-w-0 flex-1">
        <span className="flex items-center gap-1.5 text-ws-name font-medium text-fg-strong">
          {label}
        </span>
        {description ? (
          <span className="mt-0.5 block max-w-setting-description text-form-label leading-normal text-muted">
            {description}
          </span>
        ) : null}
      </span>
      <span className="flex shrink-0 items-center gap-2">
        {value}
        <ChevronRight aria-hidden="true" className="size-3.5 text-faint" />
      </span>
    </button>
  );
}

/** Mono read-only value voice for facts the daemon owns. */
export function SettingValue({ mono = false, children }: { mono?: boolean; children: ReactNode }) {
  return (
    <span
      className={cn(
        "text-small-body font-medium text-fg",
        mono && "font-mono text-eyebrow font-normal text-muted"
      )}
    >
      {children}
    </span>
  );
}
