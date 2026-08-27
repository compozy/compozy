"use client";

import * as React from "react";

import {
  formatAbsoluteTime,
  formatCompactRelativeTime,
  formatRelativeTime,
} from "../../lib/format-time";
import { cn } from "../../lib/utils";
import { useRelativeTick } from "./hooks/use-relative-tick";

export type TimeMode = "relative" | "absolute" | "compact";

export interface TimeProps extends Omit<React.ComponentProps<"time">, "title" | "children"> {
  /** ISO 8601 timestamp. */
  iso: string;
  /**
   * `relative` (default) renders `"5m ago"`; `absolute` renders a locale
   * timestamp; `compact` renders the dense rail age (`5m`, `40s`, `now`).
   */
  mode?: TimeMode;
  /** Tick interval for relative and compact modes, in milliseconds. Default 30s. */
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
  useRelativeTick(mode !== "absolute", refreshMs);

  const relative = formatRelativeTime(iso);
  const compact = formatCompactRelativeTime(iso);
  const absolute = formatAbsoluteTime(iso);
  const primary = mode === "absolute" ? absolute : mode === "compact" ? compact : relative;
  const title = mode === "absolute" ? relative : absolute;

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
