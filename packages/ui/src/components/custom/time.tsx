"use client";

import * as React from "react";

import { formatAbsoluteTime, formatRelativeTime } from "../../lib/format-time";
import { cn } from "../../lib/utils";
import { useRelativeTick } from "./hooks/use-relative-tick";

export type TimeMode = "relative" | "absolute";

export interface TimeProps extends Omit<React.ComponentProps<"time">, "title" | "children"> {
  /** ISO 8601 timestamp. */
  iso: string;
  /** `relative` (default) renders `"5m ago"`; `absolute` renders a locale timestamp. */
  mode?: TimeMode;
  /**
   * Tick interval for `relative` mode in milliseconds. Defaults to 30 s per
   *
   */
  refreshMs?: number;
}

const DEFAULT_REFRESH_MS = 30_000;

function Time({
  iso,
  mode = "relative",
  refreshMs = DEFAULT_REFRESH_MS,
  className,
  ...props
}: TimeProps) {
  useRelativeTick(mode === "relative", refreshMs);

  const relative = formatRelativeTime(iso);
  const absolute = formatAbsoluteTime(iso);
  const primary = mode === "relative" ? relative : absolute;
  const title = mode === "relative" ? absolute : relative;

  return (
    <time
      data-slot="time"
      data-mode={mode}
      dateTime={iso}
      title={title}
      className={cn("tabular-nums", className)}
      {...props}
    >
      {primary}
    </time>
  );
}

export { Time };
