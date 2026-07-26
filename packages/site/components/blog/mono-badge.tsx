import { cn } from "@agh/ui";
import type { ComponentProps } from "react";

export type MonoBadgeTone = "neutral" | "accent" | "success" | "danger" | "warning" | "info";

const toneClass: Record<MonoBadgeTone, string> = {
  neutral: "border-line text-muted",
  accent: "border-transparent bg-accent-tint text-accent",
  success: "border-transparent bg-success-tint text-success",
  danger: "border-transparent bg-danger-tint text-danger",
  warning: "border-transparent bg-warning-tint text-warning",
  info: "border-transparent bg-info-tint text-info",
};

export interface MonoBadgeProps extends ComponentProps<"span"> {
  tone?: MonoBadgeTone;
}

export function MonoBadge({ tone = "neutral", className, ...props }: MonoBadgeProps) {
  return (
    <span
      {...props}
      className={cn(
        "inline-flex items-center rounded-md border px-1.5 py-0.5 font-mono text-eyebrow font-medium tracking-mono",
        toneClass[tone],
        className
      )}
    />
  );
}
