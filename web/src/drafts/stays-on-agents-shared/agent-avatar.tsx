/**
 * Draft-local `AgentAvatar` (S1). The spec promotes it to `@compozy/ui` with a story and a test;
 * until the direction is approved it lives beside the drafts that compose it.
 */
import { Blobatar } from "@blobatar/react";
import { idle, sad, sleepy, surprised, thinking } from "blobatar/expression";
import "blobatar/motion.css";
import type { CSSProperties } from "react";

import {
  KindIcon,
  SpriteIcon,
  StatusDot,
  cn,
  identityColorsFor,
  identitySurfaceFor,
  providerKindIconRegistry,
  type StatusDotTone,
} from "@compozy/ui";

import { PROFILE_SPRITE_URL } from "@/systems/profiles/lib/profile-identity";

import { AGENT_STATE_LABEL, type AgentAppearance, type AgentState } from "./agent-state";
import { BLOB_TONES, blobHue } from "./blob-tones";
import { useCanvasTheme } from "./use-canvas-theme";

const POSE: Record<AgentState, typeof idle> = {
  working: thinking,
  "needs-you": surprised,
  failed: sad,
  paused: sleepy,
  idle,
  starting: idle,
  unknown: idle,
};

/** The accompanying signal of `_uiux.md`. `unknown` makes no claim and `needs-you` wears its own mark. */
const DOT: Partial<Record<AgentState, StatusDotTone>> = {
  working: "success",
  failed: "destructive",
  paused: "faint",
  idle: "faint",
  starting: "faint",
};

export interface AgentAvatarProps {
  appearance: AgentAppearance;
  /** Accessible name; pair with `scope` so the state, name and scope all reach the label. */
  label: string;
  scope?: string;
  state?: AgentState;
  provider?: string | null;
  size?: number;
  /** Holds the working pose still, the way reduced motion does. */
  still?: boolean;
  /** Hides the idle dot: a resting row should stay quiet. */
  quietIdle?: boolean;
  /** Surface the badges cut into, so their outline matches what sits behind them. */
  surfaceClassName?: string;
  className?: string;
}

const BODY_FILL = 1.34;

function badgePx(size: number) {
  return Math.max(12, Math.round(size * 0.36));
}

function Mark({
  appearance,
  label,
  size,
  state,
  still,
}: Required<Pick<AgentAvatarProps, "appearance" | "label" | "size" | "state" | "still">>) {
  const resolvedTheme = useCanvasTheme();
  if (appearance.avatar === "generated") {
    // The body sits inside the library's 100-unit frame with room for its decoration, so the mark
    // is drawn past the box to fill it at every size.
    return (
      <span
        className="absolute top-1/2 left-1/2 grid -translate-x-1/2 -translate-y-1/2 place-items-center [&>img]:max-w-none"
        style={{ height: Math.round(size * BODY_FILL), width: Math.round(size * BODY_FILL) }}
      >
        <Blobatar
          animate={state === "working" && !still ? "hover" : undefined}
          expression={POSE[state]}
          hue={appearance.color ? blobHue(appearance.color) : undefined}
          name={appearance.seed}
          size={Math.round(size * BODY_FILL)}
          title={label}
          traits={{ tone: [...BLOB_TONES[1][resolvedTheme]] }}
        />
      </span>
    );
  }
  const identity = identityColorsFor(appearance.color, identitySurfaceFor(resolvedTheme));
  return (
    <span
      className="grid size-full place-items-center rounded-[28%]"
      style={{ backgroundColor: identity.bg, color: identity.fg }}
    >
      {appearance.avatar === "icon" ? (
        <SpriteIcon
          name={appearance.icon}
          spriteUrl={PROFILE_SPRITE_URL}
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
  const px = Math.min(18, Math.max(8, Math.round(size * 0.3)));
  return (
    <svg
      aria-hidden="true"
      className="absolute -top-0.5 -right-0.5 text-brand"
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
export function AgentAvatar({
  appearance,
  label,
  scope,
  state = "idle",
  provider = null,
  size = 32,
  still = false,
  quietIdle = false,
  surfaceClassName = "ring-canvas",
  className,
}: AgentAvatarProps) {
  const dot = quietIdle && state === "idle" ? undefined : DOT[state];
  const spoken = [label, scope, state === "unknown" ? null : AGENT_STATE_LABEL[state]]
    .filter(Boolean)
    .join(", ");
  const badge = badgePx(size);
  const style: CSSProperties = { height: size, width: size };
  return (
    <span
      aria-label={spoken}
      className={cn("relative inline-grid shrink-0 place-items-center", className)}
      data-state={state}
      role="img"
      style={style}
    >
      <Mark appearance={appearance} label={label} size={size} state={state} still={still} />
      {state === "needs-you" ? <NeedsYouMark size={size} /> : null}
      {provider && size >= 24 ? (
        <span
          aria-hidden="true"
          className={cn(
            "absolute -right-0.5 -bottom-0.5 grid place-items-center rounded-full bg-surface-2 text-fg ring-2",
            surfaceClassName
          )}
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
            size < 24 ? "-translate-y-0.5 translate-x-0.5 ring-1" : "ring-2",
            surfaceClassName
          )}
        >
          <StatusDot size={size < 24 ? "sm" : "default"} tone={dot} />
        </span>
      ) : null}
    </span>
  );
}
