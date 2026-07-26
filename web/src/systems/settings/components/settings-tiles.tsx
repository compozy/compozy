import type { ReactNode } from "react";

import { cn, MetadataTile, Pill } from "@agh/ui";

/**
 * Read-only runtime facts (design system §06): quiet bordered tiles, visibly
 * non-editable. Replaces the old MetricGrid usage on settings pages.
 */
export function SettingsTiles({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return <div className={cn("grid gap-2.5 sm:grid-cols-2", className)}>{children}</div>;
}

export interface SettingsTileProps {
  label: ReactNode;
  value: ReactNode;
  detail?: ReactNode;
  /** Leading status dot tone (e.g. success for healthy counters). */
  dotTone?: "success" | "warning" | "danger" | "neutral";
  mono?: boolean;
  "data-testid"?: string;
}

export function SettingsTile({
  label,
  value,
  detail,
  dotTone,
  mono = false,
  "data-testid": testId,
}: SettingsTileProps) {
  return (
    <MetadataTile
      className="border border-line-soft bg-input-fill"
      data-testid={testId}
      detail={detail}
      label={label}
      value={
        <span className="inline-flex min-w-0 items-center gap-1.5">
          {dotTone ? <Pill.Dot tone={dotTone} /> : null}
          <span className={cn("truncate", mono && "font-mono text-eyebrow text-muted")}>
            {value}
          </span>
        </span>
      }
    />
  );
}
