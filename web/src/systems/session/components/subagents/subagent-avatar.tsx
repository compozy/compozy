import { useReducedMotionConfig } from "motion/react";

import { Avatar, AvatarFallback, KindIcon, StatusDot, cn } from "@compozy/ui";

import { AgentIdentityMark } from "@/systems/agent/components/agent-identity-mark";

import { subagentDot } from "./subagent-format";
import type { SubagentStatus } from "./types";

export type SubagentAvatarSize = "md" | "sm";

export interface SubagentAvatarProps {
  /** Provider id for the `KindIcon` registry; unknown or missing renders the bot glyph. */
  provider: string | null | undefined;
  /** The delegated agent's slug: its face replaces the provider tile, which stays the fallback. */
  agent?: string | null;
  /** Outcome dot (reply cards); subagent rows omit it and carry a `StateGlyph` in their trail. */
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
 * Round provider tile, with an optional bottom-right status dot. Subagent rows
 * show the provider alone; their state reads from the trail's glyph and word.
 */
export function SubagentAvatar({
  provider,
  agent,
  status,
  size = "md",
  surface = "canvas",
  still = false,
  className,
}: SubagentAvatarProps) {
  const reduced = useReducedMotionConfig();
  const dot = status ? subagentDot(status) : null;
  const statusDot = dot ? (
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
  ) : null;
  if (agent) {
    return (
      <span
        className={cn("relative inline-grid shrink-0 rounded-full", className)}
        data-slot="subagent-avatar"
        data-status={status}
      >
        <AgentIdentityMark name={agent} quietIdle size={size === "sm" ? 20 : 24} />
        {statusDot}
      </span>
    );
  }
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
      {statusDot}
    </Avatar>
  );
}
