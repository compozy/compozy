"use client";

import * as React from "react";

import { colorsFor, type OwnerKind } from "../../lib/owner-palette";
import { cn } from "../../lib/utils";

export type OwnerAvatarSize = "sm" | "default" | "lg";

const SIZE_CLASS: Record<OwnerAvatarSize, string> = {
  sm: "size-avatar-sm",
  default: "size-avatar-default",
  lg: "size-avatar-lg",
};

const SIZE_TEXT: Record<OwnerAvatarSize, string> = {
  sm: "text-micro",
  default: "text-badge",
  lg: "text-form-label",
};

const ROLE_LABEL: Record<OwnerKind, string> = {
  agent: "Agent",
  human: "Human",
  system: "System",
};

function deriveMonogram(name: string): string {
  const trimmed = name.trim();
  if (!trimmed) return "??";
  const parts = trimmed.split(/[\s_-]+/).filter(Boolean);
  if (parts.length === 0) return trimmed.slice(0, 2).toUpperCase();
  if (parts.length === 1) {
    return parts[0]!.slice(0, 2).toUpperCase();
  }
  const first = parts[0]!.charAt(0);
  const second = parts[1]!.charAt(0);
  return `${first}${second}`.toUpperCase();
}

export interface OwnerAvatarProps extends Omit<React.ComponentProps<"span">, "children"> {
  /** Owner kind selects the palette tier (agent / human / system). */
  ownerKind: OwnerKind;
  /** Stable identifier hashed into the slot index. Pass the runtime identifier, not the display label. */
  ownerId: string;
  /** Human-readable label announced via `aria-label`. Defaults to the ownerId. */
  name?: string;
  /** Avatar tier — `sm` (20 px), `default` (24 px), `lg` (32 px). */
  size?: OwnerAvatarSize;
  /** Override the auto-derived monogram. */
  monogram?: string;
  /** Replace the monogram with an icon / glyph (mostly for system owners). */
  glyph?: React.ReactNode;
  /** Override the `aria-label` role prefix. Defaults to "Agent" / "Human" / "System". */
  roleLabel?: string;
}

function OwnerAvatar({
  ownerKind,
  ownerId,
  name,
  size = "default",
  monogram,
  glyph,
  roleLabel,
  className,
  style,
  ...props
}: OwnerAvatarProps) {
  const { bg, fg } = colorsFor(ownerKind, ownerId);
  const role = roleLabel ?? ROLE_LABEL[ownerKind];
  const displayName = name?.trim() || ownerId;
  const initials = monogram ?? deriveMonogram(displayName);
  return (
    <span
      data-slot="owner-avatar"
      data-owner-kind={ownerKind}
      data-size={size}
      role="img"
      aria-label={`${role} ${displayName}`}
      className={cn(
        "inline-flex shrink-0 select-none items-center justify-center rounded-full font-medium uppercase tabular-nums",
        SIZE_CLASS[size],
        SIZE_TEXT[size],
        className
      )}
      style={{
        backgroundColor: bg,
        color: fg,
        ...style,
      }}
      {...props}
    >
      {glyph ? (
        <span data-slot="owner-avatar-glyph" aria-hidden="true" className="inline-flex">
          {glyph}
        </span>
      ) : (
        <span data-slot="owner-avatar-monogram" aria-hidden="true">
          {initials}
        </span>
      )}
    </span>
  );
}

export { OwnerAvatar };
