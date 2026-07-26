import type { ReactNode } from "react";

import { Switch } from "@agh/ui";

import { MonoTag } from "../mono-tag";

interface LoopConfigureSwitchRowProps {
  /** Mono type badge (`command` | `agent-judge` | `human` | …); omitted when not typed. */
  typeLabel?: string;
  title: string;
  description?: string;
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
 * One switch row of the configure dialog (verification check or human gate): a typed mono
 * badge, a title + description, an enable switch, and an optional command-field slot. A
 * locked row (a structural, non-command check) is rendered on and disabled.
 */
export function LoopConfigureSwitchRow({
  typeLabel,
  title,
  description,
  checked,
  disabled = false,
  lockedHint,
  onCheckedChange,
  testId,
  children,
}: LoopConfigureSwitchRowProps) {
  return (
    <div
      className="flex flex-col gap-2.5 border-t border-line-soft px-3.5 py-3 first:border-t-0"
      data-testid={testId}
      data-locked={disabled ? "true" : undefined}
    >
      <div className="flex items-center gap-3">
        {typeLabel ? (
          <MonoTag className="min-w-[84px] shrink-0 justify-center rounded-xs bg-badge-fill px-1.5 py-1 text-pill-group-badge">
            {typeLabel}
          </MonoTag>
        ) : null}
        <div className="min-w-0 flex-1">
          <p className="text-small-body font-medium text-fg-strong">{title}</p>
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
