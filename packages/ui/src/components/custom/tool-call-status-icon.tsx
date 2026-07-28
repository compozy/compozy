"use client";

import { CheckIcon, MinusIcon, XIcon } from "lucide-react";
import type * as React from "react";

import { cn } from "../../lib/utils";
import { Spinner } from "../spinner";
import type { ToolCallStatus } from "./tool-call-row";

const LABEL: Record<ToolCallStatus, string> = {
  pending: "Pending",
  running: "Running",
  failed: "Error",
  success: "Done",
  empty: "Empty",
};

type GlyphStatus = Exclude<ToolCallStatus, "pending" | "running">;

const TONE_CLASS: Record<GlyphStatus, string> = {
  failed: "text-danger",
  success: "text-success",
  empty: "text-subtle",
};

const ICON: Record<GlyphStatus, React.ElementType> = {
  failed: XIcon,
  success: CheckIcon,
  empty: MinusIcon,
};

export interface ToolCallStatusIconProps {
  status: ToolCallStatus;
  className?: string;
}

/**
 * Trailing status glyph for `ToolCallRow`, one visual language across every tool
 * state: `pending` renders nothing (the row is muted while it prepares input),
 * `running` spins, and the resolved states map to a single signal-toned Lucide
 * glyph — X (danger) / Check (success) / Minus (faint empty-neutral). Neutral is
 * promoted to `success` upstream once the turn settles, so no premature green
 * appears mid-stream.
 */
export function ToolCallStatusIcon({ status, className }: ToolCallStatusIconProps) {
  if (status === "pending") {
    return null;
  }
  const label = LABEL[status];
  if (status === "running") {
    return (
      <Spinner
        data-slot="tool-call-row-status"
        data-status={status}
        aria-label={label}
        className={cn("size-3 shrink-0 text-muted", className)}
      />
    );
  }
  const Icon = ICON[status];
  return (
    <Icon
      data-slot="tool-call-row-status"
      data-status={status}
      role="img"
      aria-label={label}
      strokeWidth={1.75}
      className={cn("size-3 shrink-0", TONE_CLASS[status], className)}
    />
  );
}
