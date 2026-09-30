import type { ComponentProps } from "react";
import { Layers } from "lucide-react";

import { cn, identityColorsFor, identitySurfaceFor, SpriteIcon } from "@compozy/ui";

import { useThemePreference } from "@/systems/theme";

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
  /**
   * Surface the glyph sits on, so the ink is measured against the right plate.
   * Defaults to the active theme's panel surface.
   */
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

type GlyphIdentity = ReturnType<typeof identityColorsFor>;

/** Presence flag rendered as `"true"`, omitted when unset. */
function trueAttr(value: boolean): "true" | undefined {
  return value ? "true" : undefined;
}

/** Accessible naming, dropped entirely when visible text already names the profile. */
function glyphA11yProps(decorative: boolean, label: string) {
  if (decorative) return { role: undefined, "aria-label": undefined, "aria-hidden": true as const };
  return { role: "img", "aria-label": label, "aria-hidden": undefined };
}

/** The aggregate mark keeps the caller's style; an identity paints its measured colors. */
function glyphStyle(
  aggregate: boolean,
  current: boolean,
  identity: GlyphIdentity,
  style: ComponentProps<"span">["style"]
): ComponentProps<"span">["style"] {
  if (aggregate) return style;
  return {
    backgroundColor: identity.bg,
    color: identity.fg,
    ...(current ? { "--tw-ring-color": identity.fg } : {}),
    ...style,
  };
}

function ProfileGlyphMark({
  aggregate,
  icon,
  emoji,
  size,
}: {
  aggregate: boolean;
  icon: string | null;
  emoji: string | null;
  size: ProfileGlyphSize;
}) {
  if (aggregate) {
    return <Layers aria-hidden="true" className={GLYPH_CLASS[size]} strokeWidth={1.75} />;
  }
  const symbol = symbolOf({ icon, emoji });
  if (symbol.kind === "emoji") return <span aria-hidden="true">{symbol.value}</span>;
  return (
    <SpriteIcon
      spriteUrl={PROFILE_SPRITE_URL}
      name={symbol.value}
      className={cn(GLYPH_CLASS[size], "text-current")}
    />
  );
}

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
  const { resolvedTheme } = useThemePreference();
  const identity = identityColorsFor(color, surface ?? identitySurfaceFor(resolvedTheme));
  const label = aggregate ? "All profiles" : name;

  return (
    <span
      data-slot="profile-glyph"
      data-current={trueAttr(current)}
      data-aggregate={trueAttr(aggregate)}
      {...glyphA11yProps(decorative, label)}
      className={cn(
        "relative inline-grid shrink-0 place-items-center leading-none",
        SIZE_CLASS[size],
        aggregate && "border border-line-strong bg-surface-2 text-muted",
        current && !aggregate && "ring-[length:var(--ring-width-profile-current)]",
        className
      )}
      style={glyphStyle(aggregate, current, identity, style)}
      {...props}
    >
      <ProfileGlyphMark
        aggregate={aggregate}
        emoji={emoji ?? null}
        icon={icon ?? null}
        size={size}
      />
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
