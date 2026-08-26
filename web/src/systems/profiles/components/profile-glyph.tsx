import type { ComponentProps } from "react";
import { Layers } from "lucide-react";

import { cn, identityColorsFor, SpriteIcon } from "@compozy/ui";

import { PROFILE_SPRITE_URL, symbolOf } from "../lib/profile-identity";

export type ProfileGlyphSize = "sm" | "default" | "lg";

export interface ProfileGlyphProps extends Omit<ComponentProps<"span">, "children"> {
  name: string;
  color?: string;
  icon?: string | null;
  emoji?: string | null;
  size?: ProfileGlyphSize;
  /** Ring in the identity color — data, not a signal. */
  current?: boolean;
  /** Warning dot paired with the words "needs setup" by the row that owns it. */
  needsSetup?: boolean;
  /** The neutral layered mark: an aggregate is not an identity. */
  aggregate?: boolean;
  /** Surface the glyph sits on, so the ink is measured against the right plate. */
  surface?: string;
  /**
   * Drops the image role and label.
   *
   * Set it wherever visible text already names the profile: the glyph is then a
   * second rendering of the same fact, and announcing it twice makes a two-word
   * tag read as four.
   */
  decorative?: boolean;
}

const SIZE_CLASS: Record<ProfileGlyphSize, string> = {
  sm: "size-profile-glyph-sm rounded-xs text-small-body",
  default: "size-topbar-glyph rounded-sm text-small-body",
  lg: "size-7 rounded-md text-item-title",
};

const GLYPH_CLASS: Record<ProfileGlyphSize, string> = {
  sm: "size-3.5",
  default: "size-3.5",
  lg: "size-4",
};

/** Renders user-chosen identity color with measured foreground contrast. */
export function ProfileGlyph({
  className,
  name,
  color,
  icon,
  emoji,
  size = "default",
  current = false,
  needsSetup = false,
  aggregate = false,
  surface,
  decorative = false,
  style,
  ...props
}: ProfileGlyphProps) {
  const symbol = symbolOf({ icon: icon ?? null, emoji: emoji ?? null });
  const identity = identityColorsFor(color, surface);
  const label = aggregate ? "All profiles" : name;

  return (
    <span
      data-slot="profile-glyph"
      data-current={current ? "true" : undefined}
      data-aggregate={aggregate ? "true" : undefined}
      role={decorative ? undefined : "img"}
      aria-label={decorative ? undefined : label}
      aria-hidden={decorative ? true : undefined}
      className={cn(
        "relative inline-grid shrink-0 place-items-center leading-none",
        SIZE_CLASS[size],
        aggregate && "border border-line-strong bg-badge-fill text-muted",
        current && !aggregate && "ring-[length:var(--ring-width-profile-current)]",
        className
      )}
      style={
        aggregate
          ? style
          : {
              backgroundColor: identity.bg,
              color: identity.fg,
              ...(current ? { "--tw-ring-color": identity.fg } : {}),
              ...style,
            }
      }
      {...props}
    >
      {aggregate ? (
        <Layers aria-hidden="true" className={GLYPH_CLASS[size]} strokeWidth={1.75} />
      ) : symbol.kind === "emoji" ? (
        <span aria-hidden="true">{symbol.value}</span>
      ) : (
        <SpriteIcon
          spriteUrl={PROFILE_SPRITE_URL}
          name={symbol.value}
          className={cn(GLYPH_CLASS[size], "text-current")}
        />
      )}
      {needsSetup ? (
        <span
          aria-hidden="true"
          data-slot="profile-glyph-dot"
          className="absolute -top-0.5 -right-0.5 size-2 rounded-full bg-warning ring-2 ring-canvas-soft"
        />
      ) : null}
    </span>
  );
}
