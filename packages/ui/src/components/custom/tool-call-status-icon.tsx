"use client";

import { CheckIcon, MinusIcon, XIcon } from "lucide-react";
import type * as React from "react";

import { cn } from "../../lib/utils";
import { StateGlyph } from "./state-glyph";
import type { ToolCallStatus } from "./tool-call-row";

const LABEL: Record<ToolCallStatus, string> = {
  pending: "Pending",
  running: "Running",
  failed: "Error",
  absorbed: "Failed",
  stopped: "Stopped",
  success: "Done",
  empty: "Empty",
};

type GlyphStatus = Exclude<ToolCallStatus, "pending" | "running" | "stopped">;

// Calm-transcript status budget: besides the mint in-progress ring, only a
// failure that ended the turn (`failed`) carries a signal hue. Success is a GREY check — completion is the resting
// state, not an event — and an absorbed failure is a GREY × (ADR-009).
const TONE_CLASS: Record<GlyphStatus, string> = {
  failed: "text-danger",
  absorbed: "text-subtle",
  success: "text-subtle",
  empty: "text-subtle",
};

const ICON: Record<GlyphStatus, React.ElementType> = {
  failed: XIcon,
  absorbed: XIcon,
  success: CheckIcon,
  empty: MinusIcon,
};

export interface ToolCallStatusIconProps {
  status: ToolCallStatus;
  className?: string;
  /** Hold the running glyph still (a paused window, US-018.EC-2). */
  still?: boolean;
}

/**
 * Trailing status glyph for `ToolCallRow`, one visual language across every tool
 * state: `pending` and `stopped` render nothing (the row is muted while it
 * prepares input; a stopped call carries its word instead), `running` is the
 * mint StateGlyph ring (the one in-progress signal the transcript allows),
 * and the resolved states map to a single Lucide glyph — X danger (`failed`),
 * X subtle (`absorbed`), Check (success), Minus (faint empty-neutral). Neutral
 * is promoted to `success` upstream once the turn settles, so no premature
 * green appears mid-stream.
 */
export function ToolCallStatusIcon({ status, className, still }: ToolCallStatusIconProps) {
  if (status === "pending" || status === "stopped") {
    return null;
  }
  const label = LABEL[status];
  if (status === "running") {
    return (
      <StateGlyph
        data-slot="tool-call-row-status"
        data-status={status}
        state="running"
        size="sm"
        label={label}
        // A live region, as the spinner was: assistive tech hears "Running".
        role="status"
        still={still}
        className={className}
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
