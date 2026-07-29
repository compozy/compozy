"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

export type MarkerTone = "neutral" | "danger" | "warning" | "info";

export interface MarkerProps extends React.ComponentProps<"div"> {
  /** Signal tone carried by the glyph only — the sentence stays neutral. */
  tone?: MarkerTone;
  /** 12px leading glyph (lucide icon element). */
  icon?: React.ReactNode;
  children: React.ReactNode;
}

const TONE_ICON: Record<MarkerTone, string> = {
  neutral: "text-faint",
  danger: "text-danger",
  warning: "text-warning",
  info: "text-info",
};

/**
 * One-line system event in the transcript — replaces every in-transcript Alert
 * card: a 12px tone glyph plus one muted sentence. Tone lives in the glyph,
 * never in the text or a background; `<b>` inside the sentence carries the
 * subject at `--muted`. Compose `MarkerMeta` for raw kind strings or ×N counts.
 */
function Marker({ tone = "neutral", icon, children, className, ...props }: MarkerProps) {
  return (
    <div
      data-slot="marker"
      data-tone={tone}
      className={cn(
        "flex min-h-[22px] min-w-0 items-start gap-[7px] px-1 py-0.5",
        "text-[12px] leading-[1.55] text-subtle",
        className
      )}
      {...props}
    >
      {icon ? (
        <span
          aria-hidden="true"
          data-slot="marker-icon"
          className={cn("mt-[2.5px] shrink-0 *:[svg]:size-3", TONE_ICON[tone])}
        >
          {icon}
        </span>
      ) : null}
      <span
        data-slot="marker-body"
        className="min-w-0 [&_b]:font-medium [&_b]:text-muted [&_strong]:font-medium [&_strong]:text-muted"
      >
        {children}
      </span>
    </div>
  );
}

export interface MarkerMetaProps extends React.ComponentProps<"span"> {
  children: React.ReactNode;
}

/** Faint mono meta inside a marker sentence — raw kind strings, ×N counts. */
function MarkerMeta({ children, className, ...props }: MarkerMetaProps) {
  return (
    <span
      data-slot="marker-meta"
      className={cn("font-mono text-badge text-faint", className)}
      {...props}
    >
      {children}
    </span>
  );
}

export { Marker, MarkerMeta };
