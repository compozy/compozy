import { cn } from "@agh/ui";
import type { ComponentProps } from "react";
import { formatDate, formatDateCompact } from "./format";

export type DateStampTone = "neutral" | "accent" | "success" | "danger" | "warning" | "info";

export interface DateStampProps extends Omit<ComponentProps<"time">, "children" | "dateTime"> {
  date: string;
  format?: "default" | "compact" | "compact-year";
  tone?: DateStampTone;
  tracking?: "default" | "wide";
}

const toneClass: Record<DateStampTone, string> = {
  neutral: "text-muted",
  accent: "text-accent",
  success: "text-success",
  danger: "text-danger",
  warning: "text-warning",
  info: "text-info",
};

function displayDate(date: string, format: NonNullable<DateStampProps["format"]>) {
  if (format === "compact") {
    return formatDateCompact(date);
  }
  if (format === "compact-year") {
    return `${formatDateCompact(date)} · ${new Date(date).getUTCFullYear()}`;
  }
  return formatDate(date);
}

export function DateStamp({
  date,
  format = "default",
  tone = "neutral",
  tracking = "default",
  className,
  ...props
}: DateStampProps) {
  return (
    <time
      dateTime={date}
      {...props}
      className={cn(
        "eyebrow font-semibold!",
        tracking === "wide" && "tracking-badge!",
        toneClass[tone],
        className
      )}
    >
      {displayDate(date, format)}
    </time>
  );
}
