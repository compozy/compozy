import type { ReactNode } from "react";

import { HelpTip, Skeleton, SkeletonText, Switch } from "@compozy/ui";

const SWITCH_ROW_CLASS = "flex flex-col gap-2 border-t border-line-soft px-4 py-3 first:border-t-0";

interface LoopConfigureSwitchRowProps {
  title: string;
  description?: string;
  /** Explanatory prose, shown as a `HelpTip` beside the title. */
  help?: string;
  checked: boolean;
  /** Locked rows render the switch on + disabled (can't be removed without a fork). */
  disabled?: boolean;
  lockedHint?: string;
  onCheckedChange?: (checked: boolean) => void;
  testId?: string;
  /** Optional slot rendered below the row (the conditional command field). */
  children?: ReactNode;
}

/**
 * One switch row of the configure dialog (verification check or human gate): a title +
 * description, an enable switch, and an optional command-field slot. A locked row (a
 * structural, non-command check) is rendered on and disabled.
 */
export function LoopConfigureSwitchRow({
  title,
  description,
  help,
  checked,
  disabled = false,
  lockedHint,
  onCheckedChange,
  testId,
  children,
}: LoopConfigureSwitchRowProps) {
  return (
    <div
      className={SWITCH_ROW_CLASS}
      data-testid={testId}
      data-locked={disabled ? "true" : undefined}
    >
      <div className="flex items-center gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-1.5">
            <p className="text-small-body font-medium text-fg-strong">{title}</p>
            {help ? <HelpTip label={`About ${title.toLowerCase()}`}>{help}</HelpTip> : null}
          </div>
          {description ? (
            <p className="mt-0.5 text-form-hint leading-snug text-subtle">{description}</p>
          ) : null}
          {disabled && lockedHint ? (
            <p className="mt-0.5 text-form-hint text-faint">{lockedHint}</p>
          ) : null}
        </div>
        <Switch
          size="sm"
          checked={checked}
          disabled={disabled}
          onCheckedChange={onCheckedChange}
          aria-label={title}
        />
      </div>
      {children}
    </div>
  );
}

/**
 * A switch row while the settings are being read: the row's frame with a title, a description
 * and a switch-sized placeholder. Presentational; the caller owns the `SkeletonRegion`.
 */
export function LoopConfigureSwitchRowSkeleton({
  title,
  description = true,
}: {
  /** The row's title when it is static (the approval row); a placeholder otherwise. */
  title?: string;
  description?: boolean;
}) {
  return (
    <div aria-hidden="true" className={SWITCH_ROW_CLASS}>
      <div className="flex items-center gap-3">
        <div className="min-w-0 flex-1">
          {title ? (
            <p className="text-small-body font-medium text-fg-strong">{title}</p>
          ) : (
            <SkeletonText className="w-32 text-small-body font-medium" />
          )}
          {description ? <SkeletonText className="mt-0.5 w-3/4 text-form-hint" /> : null}
        </div>
        <Skeleton className="h-switch-sm w-switch-sm rounded-full" />
      </div>
    </div>
  );
}
