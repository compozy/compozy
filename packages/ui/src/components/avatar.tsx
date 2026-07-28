"use client";

import { Avatar as AvatarPrimitive } from "@base-ui/react/avatar";
import * as React from "react";

import { cn } from "../lib/utils";

type AvatarShape = "circle" | "square";
type AvatarSize = "default" | "sm" | "lg";

const SHAPE_RADIUS: Record<AvatarShape, string> = {
  circle: "rounded-full after:rounded-full",
  square: "rounded-md after:rounded-md",
};

interface AvatarOwnProps {
  size?: AvatarSize;
  shape?: AvatarShape;
}

function Avatar({
  className,
  size = "default",
  shape = "circle",
  ...props
}: AvatarPrimitive.Root.Props & AvatarOwnProps) {
  return (
    <AvatarPrimitive.Root
      data-slot="avatar"
      data-size={size}
      data-shape={shape}
      className={cn(
        "group/avatar relative flex size-8 shrink-0 select-none after:absolute after:inset-0 after:border after:border-line data-[size=lg]:size-10 data-[size=sm]:size-6",
        SHAPE_RADIUS[shape],
        className
      )}
      {...props}
    />
  );
}

function AvatarImage({ className, ...props }: AvatarPrimitive.Image.Props) {
  return (
    <AvatarPrimitive.Image
      data-slot="avatar-image"
      className={cn(
        "aspect-square size-full object-cover group-data-[shape=circle]/avatar:rounded-full group-data-[shape=square]/avatar:rounded-md",
        className
      )}
      {...props}
    />
  );
}

function AvatarFallback({ className, ...props }: AvatarPrimitive.Fallback.Props) {
  return (
    <AvatarPrimitive.Fallback
      data-slot="avatar-fallback"
      className={cn(
        "flex size-full items-center justify-center bg-elevated text-small-body font-medium text-fg-strong group-data-[size=sm]/avatar:text-eyebrow group-data-[shape=circle]/avatar:rounded-full group-data-[shape=square]/avatar:rounded-md",
        className
      )}
      {...props}
    />
  );
}

type AvatarBadgeTone = "neutral" | "success" | "warning" | "danger" | "info" | "accent";

const AVATAR_BADGE_TONE: Record<AvatarBadgeTone, string> = {
  neutral: "bg-neutral",
  success: "bg-success",
  warning: "bg-warning",
  danger: "bg-danger",
  info: "bg-info",
  accent: "bg-accent",
};

function AvatarBadge({
  className,
  tone = "neutral",
  ...props
}: React.ComponentProps<"span"> & { tone?: AvatarBadgeTone }) {
  return (
    <span
      data-slot="avatar-badge"
      data-tone={tone}
      className={cn(
        "absolute right-0 bottom-0 z-10 inline-flex items-center justify-center rounded-full text-accent-ink ring-2 ring-canvas select-none",
        AVATAR_BADGE_TONE[tone],
        "group-data-[size=sm]/avatar:size-2 group-data-[size=sm]/avatar:[&>svg]:hidden",
        "group-data-[size=default]/avatar:size-2.5 group-data-[size=default]/avatar:[&>svg]:size-2",
        "group-data-[size=lg]/avatar:size-3 group-data-[size=lg]/avatar:[&>svg]:size-2",
        className
      )}
      {...props}
    />
  );
}

function AvatarGroup({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="avatar-group"
      className={cn(
        "group/avatar-group flex -space-x-2 *:data-[slot=avatar]:ring-2 *:data-[slot=avatar]:ring-canvas",
        className
      )}
      {...props}
    />
  );
}

function AvatarGroupCount({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="avatar-group-count"
      className={cn(
        "relative flex size-8 shrink-0 items-center justify-center rounded-full bg-elevated text-small-body font-medium text-fg ring-2 ring-canvas group-has-data-[size=lg]/avatar-group:size-10 group-has-data-[size=sm]/avatar-group:size-6 [&>svg]:size-4 group-has-data-[size=lg]/avatar-group:[&>svg]:size-5 group-has-data-[size=sm]/avatar-group:[&>svg]:size-3",
        className
      )}
      {...props}
    />
  );
}

export { Avatar, AvatarBadge, AvatarFallback, AvatarGroup, AvatarGroupCount, AvatarImage };
export type { AvatarBadgeTone, AvatarShape, AvatarSize };
