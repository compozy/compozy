"use client";

import { XIcon } from "lucide-react";

import { cn } from "../../lib/utils";
import { Spinner } from "../spinner";
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

type GlyphStatus = Extract<ToolCallStatus, "failed" | "absorbed">;

// Calm-transcript status budget: only a failure that ended the turn (`failed`)
// carries a signal hue, and an absorbed failure is a GREY × (ADR-009).
// Completion is the resting state, not an event: success and empty carry no
// glyph at all — the row's accessible name still states them.
const TONE_CLASS: Record<GlyphStatus, string> = {
  failed: "text-danger",
  absorbed: "text-subtle",
};

export interface ToolCallStatusIconProps {
  status: ToolCallStatus;
  className?: string;
}

/**
 * Trailing status glyph for `ToolCallRow`, one visual language across every tool
 * state: `running` spins, a failure is an × — danger when it ended the turn
 * (`failed`), subtle when the turn kept going (`absorbed`) — and every resting
 * state (`pending`, `stopped`, `success`, `empty`) renders nothing; a stopped
 * call carries its word instead.
 */
export function ToolCallStatusIcon({ status, className }: ToolCallStatusIconProps) {
  if (status !== "running" && status !== "failed" && status !== "absorbed") {
    return null;
  }
  const label = LABEL[status];
  if (status === "running") {
    return (
      <Spinner
        data-slot="tool-call-row-status"
        data-status={status}
        aria-label={label}
        className={cn("size-3 shrink-0 text-subtle", className)}
      />
    );
  }
  return (
    <XIcon
      data-slot="tool-call-row-status"
      data-status={status}
      role="img"
      aria-label={label}
      className={cn("size-3 shrink-0", TONE_CLASS[status], className)}
    />
  );
}
