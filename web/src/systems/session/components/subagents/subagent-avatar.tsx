import { useReducedMotionConfig } from "motion/react";

import { Avatar, AvatarFallback, KindIcon, StatusDot, cn } from "@compozy/ui";

import { subagentDot } from "./subagent-format";
import type { SubagentStatus } from "./types";

export type SubagentAvatarSize = "md" | "sm";

export interface SubagentAvatarProps {
  /** Provider id for the `KindIcon` registry; unknown or missing renders the bot glyph. */
  provider: string | null | undefined;
  /** Omit to drop the dot (stacked group avatars). */
  status?: SubagentStatus;
  /** `md` = 24 px (cards, groups); `sm` = 20 px (roster, chip preview). */
  size?: SubagentAvatarSize;
  /** Surface behind the avatar, so the ring around the dot matches it. */
  surface?: "canvas" | "elevated" | "rail";
  /** Hold the running pulse still (a stale row the stream has not confirmed). */
  still?: boolean;
  className?: string;
}

const RING_SURFACE = {
  canvas: "ring-canvas",
  elevated: "ring-elevated",
  rail: "ring-canvas-soft",
} as const;

/**
 * Round provider tile with a bottom-right status dot (transcript VC-02 ①). The
 * dot carries the state; the label beside the avatar carries the word.
 */
export function SubagentAvatar({
  provider,
  status,
  size = "md",
  surface = "canvas",
  still = false,
  className,
}: SubagentAvatarProps) {
  const reduced = useReducedMotionConfig();
  const dot = status ? subagentDot(status) : null;
  return (
    <Avatar
      size="sm"
      data-slot="subagent-avatar"
      data-status={status}
      className={cn("after:border-line-strong", size === "sm" && "size-5", className)}
    >
      <AvatarFallback className="bg-surface-2">
        <KindIcon kind={provider ?? undefined} size="xs" tone="muted" />
      </AvatarFallback>
      {dot ? (
        <StatusDot
          tone={dot.tone ?? "faint"}
          variant={dot.tone === null ? "ring" : "solid"}
          size={size === "sm" ? "sm" : "default"}
          className={cn(
            "absolute -right-0.5 -bottom-0.5 z-10 ring-2",
            RING_SURFACE[surface],
            dot.tone === null && "bg-canvas text-indicator",
            dot.pulse && !reduced && !still && "animate-pulse"
          )}
        />
      ) : null}
    </Avatar>
  );
}
