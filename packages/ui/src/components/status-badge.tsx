import type { CSSProperties, HTMLAttributes, ReactElement } from "react";

import { cn } from "../lib/utils";

export type StatusBadgeTone = "accent" | "success" | "warning" | "info" | "danger" | "neutral";

export interface StatusBadgeProps extends HTMLAttributes<HTMLSpanElement> {
  tone?: StatusBadgeTone;
}

const toneStyles: Record<StatusBadgeTone, CSSProperties> = {
  accent: {
    backgroundColor: "var(--tone-accent-bg)",
    borderColor: "var(--tone-accent-border)",
    color: "var(--tone-accent-text)",
  },
  success: {
    backgroundColor: "var(--tone-success-bg)",
    borderColor: "var(--tone-success-border)",
    color: "var(--tone-success-text)",
  },
  warning: {
    backgroundColor: "var(--tone-warning-bg)",
    borderColor: "var(--tone-warning-border)",
    color: "var(--tone-warning-text)",
  },
  info: {
    backgroundColor: "var(--tone-info-bg)",
    borderColor: "var(--tone-info-border)",
    color: "var(--tone-info-text)",
  },
  danger: {
    backgroundColor: "var(--tone-danger-bg)",
    borderColor: "var(--tone-danger-border)",
    color: "var(--tone-danger-text)",
  },
  neutral: {
    backgroundColor: "var(--tone-neutral-bg)",
    borderColor: "var(--tone-neutral-border)",
    color: "var(--tone-neutral-text)",
  },
};

export function StatusBadge({
  children,
  className,
  style,
  tone = "neutral",
  ...props
}: StatusBadgeProps): ReactElement {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-[calc(var(--radius)-2px)] border px-2.5 py-1",
        "eyebrow whitespace-nowrap",
        className
      )}
      style={{ ...toneStyles[tone], ...style }}
      {...props}
    >
      {children}
    </span>
  );
}
