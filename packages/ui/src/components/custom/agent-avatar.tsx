"use client";

import { Blobatar } from "@blobatar/react";
import { idle, sad, sleepy, surprised, thinking } from "blobatar/expression";
import "blobatar/motion.css";
import { useReducedMotionConfig } from "motion/react";
import type * as React from "react";

import {
  AGENT_AVATAR_STATE_LABEL,
  type AgentAvatarAppearance,
  type AgentAvatarState,
} from "../../lib/agent-avatar";
import { agentAvatarHue, agentAvatarTones } from "../../lib/agent-avatar-tones";
import { identityColorsFor, identitySurfaceFor } from "../../lib/identity-palette";
import { cn } from "../../lib/utils";
import { useDocumentTheme } from "./hooks/use-document-theme";
import { KindIcon } from "./kind-icon";
import { providerKindIconRegistry } from "./kind-icon-registry";
import { SpriteIcon } from "./sprite-icon";
import { StatusDot, type StatusDotTone } from "./status-dot";

const POSE: Record<AgentAvatarState, typeof idle> = {
  working: thinking,
  "needs-you": surprised,
  failed: sad,
  paused: sleepy,
  idle,
  starting: idle,
  unknown: idle,
};

/** The accompanying signal: `unknown` makes no claim and `needs-you` wears its own mark. */
const DOT: Partial<Record<AgentAvatarState, StatusDotTone>> = {
  working: "success",
  failed: "destructive",
  paused: "faint",
  idle: "faint",
  starting: "faint",
};

const FALLBACK_SYMBOL_COLOR = "#7c5cff";

/** The body sits inside the library's 100-unit frame, so the mark is drawn past the box to fill it. */
const BODY_FILL = 1.34;
const BADGE_MIN_SIZE = 24;
const NEEDS_YOU_MAX_PX = 18;

export interface AgentAvatarProps extends Omit<React.ComponentProps<"span">, "children"> {
  appearance: AgentAvatarAppearance;
  /** Accessible name; `scope` and the state join it, so a label always says all three. */
  label: string;
  scope?: string;
  state?: AgentAvatarState;
  /** Provider kind for the corner badge, from 24 px. */
  provider?: string | null;
  /** Rendered size in pixels, 16 to 112. */
  size?: number;
  /** Hides the idle dot: a resting row should stay quiet. */
  quietIdle?: boolean;
  /** Surface the badges cut into, so their outline matches what sits behind them. */
  surfaceClassName?: string;
  /** The Lucide sprite an `icon` appearance reads from. */
  spriteUrl?: string;
}

function GeneratedMark({
  appearance,
  size,
  state,
}: {
  appearance: Extract<AgentAvatarAppearance, { avatar: "generated" }>;
  size: number;
  state: AgentAvatarState;
}) {
  const theme = useDocumentTheme();
  const reduced = useReducedMotionConfig();
  const drawn = Math.round(size * BODY_FILL);
  return (
    <span
      aria-hidden="true"
      className="absolute top-1/2 left-1/2 grid -translate-x-1/2 -translate-y-1/2 place-items-center [&>img]:max-w-none"
      style={{ height: drawn, width: drawn }}
    >
      <Blobatar
        animate={state === "working" && !reduced ? "hover" : undefined}
        expression={POSE[state]}
        hue={appearance.color ? agentAvatarHue(appearance.color) : undefined}
        name={appearance.seed}
        size={drawn}
        traits={{ tone: [...agentAvatarTones(appearance.generation, theme)] }}
      />
    </span>
  );
}

function SymbolMark({
  appearance,
  size,
  spriteUrl,
}: {
  appearance: Exclude<AgentAvatarAppearance, { avatar: "generated" }>;
  size: number;
  spriteUrl?: string;
}) {
  const theme = useDocumentTheme();
  const identity = identityColorsFor(
    appearance.color ?? FALLBACK_SYMBOL_COLOR,
    identitySurfaceFor(theme)
  );
  return (
    <span
      className="grid size-full place-items-center rounded-[28%]"
      style={{ backgroundColor: identity.bg, color: identity.fg }}
    >
      {appearance.avatar === "icon" ? (
        <SpriteIcon
          name={appearance.icon}
          spriteUrl={spriteUrl ?? ""}
          style={{ height: size * 0.56, width: size * 0.56 }}
        />
      ) : (
        <span aria-hidden="true" className="leading-none" style={{ fontSize: size * 0.6 }}>
          {appearance.emoji}
        </span>
      )}
    </span>
  );
}

/** A small triangle with soft corners in the Needs you orange, in the signal dot's slot. */
function NeedsYouMark({ size }: { size: number }) {
  const px = Math.min(NEEDS_YOU_MAX_PX, Math.max(8, Math.round(size * 0.3)));
  return (
    <svg
      aria-hidden="true"
      className="absolute -top-0.5 -right-0.5 text-brand"
      data-slot="agent-avatar-needs-you"
      height={px}
      viewBox="0 0 12 12"
      width={px}
    >
      <path
        d="M6 2.5 10 9.5H2Z"
        fill="currentColor"
        stroke="currentColor"
        strokeLinejoin="round"
        strokeWidth="3"
      />
    </svg>
  );
}

/** Pose, provider badge and state signal on one mark. */
function AgentAvatar({
  appearance,
  label,
  scope,
  state = "idle",
  provider = null,
  size = 32,
  quietIdle = false,
  surfaceClassName = "ring-canvas",
  spriteUrl,
  className,
  style,
  ...props
}: AgentAvatarProps) {
  const dot = quietIdle && state === "idle" ? undefined : DOT[state];
  const spoken = [label, scope, state === "unknown" ? null : AGENT_AVATAR_STATE_LABEL[state]]
    .filter(Boolean)
    .join(", ");
  const badge = Math.max(12, Math.round(size * 0.36));
  return (
    <span
      {...props}
      aria-label={spoken}
      className={cn("relative inline-grid shrink-0 place-items-center", className)}
      data-slot="agent-avatar"
      data-state={state}
      role="img"
      style={{ ...style, height: size, width: size }}
    >
      {appearance.avatar === "generated" ? (
        <GeneratedMark appearance={appearance} size={size} state={state} />
      ) : (
        <SymbolMark appearance={appearance} size={size} spriteUrl={spriteUrl} />
      )}
      {state === "needs-you" ? <NeedsYouMark size={size} /> : null}
      {provider && size >= BADGE_MIN_SIZE ? (
        <span
          aria-hidden="true"
          className={cn(
            "absolute -right-0.5 -bottom-0.5 grid place-items-center rounded-full bg-surface-2 text-fg ring-2",
            surfaceClassName
          )}
          data-slot="agent-avatar-provider"
          style={{ height: badge, width: badge }}
        >
          <KindIcon
            className="size-full"
            kind={provider}
            registry={providerKindIconRegistry}
            style={{ height: badge * 0.66, width: badge * 0.66 }}
          />
        </span>
      ) : null}
      {dot ? (
        <span
          aria-hidden="true"
          className={cn(
            "absolute top-0 right-0 grid place-items-center rounded-full",
            size < BADGE_MIN_SIZE ? "-translate-y-0.5 translate-x-0.5 ring-1" : "ring-2",
            surfaceClassName
          )}
          data-slot="agent-avatar-dot"
        >
          <StatusDot size={size < BADGE_MIN_SIZE ? "sm" : "default"} tone={dot} />
        </span>
      ) : null}
    </span>
  );
}

export { AgentAvatar };
