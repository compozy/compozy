import { cn } from "@compozy/ui";
import type { ComponentProps } from "react";

export type MonoBadgeTone = "neutral" | "brand" | "success" | "destructive" | "warning" | "info";

const toneClass: Record<MonoBadgeTone, string> = {
  neutral: "border-line text-muted-foreground",
  brand: "border-transparent bg-brand-tint text-brand",
  success: "border-transparent bg-success-tint text-success",
  destructive: "border-transparent bg-destructive-tint text-destructive",
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
