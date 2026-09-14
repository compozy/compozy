"use client";

import { BoxIcon, ChevronRightIcon } from "lucide-react";
import * as React from "react";

import { cn } from "../lib/utils";

type IconComponent = React.ComponentType<{ className?: string; size?: number }>;
type EmptyTitleTag = "div" | "h1" | "h2" | "h3" | "h4" | "h5" | "h6" | "p" | "span";

export interface EmptyProps extends Omit<React.ComponentProps<"div">, "title"> {
  /** Art slot above the icon well. Sits alongside the icon, never replacing it. */
  illustration?: React.ReactNode;
  icon?: IconComponent | React.ReactNode;
  title: React.ReactNode;
  titleAs?: EmptyTitleTag;
  description?: React.ReactNode;
  /** One line of guidance below the description. */
  hint?: React.ReactNode;
  /**
   * Raw cause — an error string, stack, or payload. It renders collapsed behind
   * a "Details" disclosure so the state reads as a sentence, not a stack trace.
   */
  cause?: React.ReactNode;
  action?: React.ReactNode;
  /** Starter actions below the primary `action` row. */
  nextSteps?: React.ReactNode;
  /**
   * Framed variant — a bordered, intrinsically-sized card for routed
   * empty/error states (absorbs the old `RouteState`). The icon well stays the
   * filled element; the frame is an outline so it never collapses against a
   * canvas-soft parent. `fill` defaults to `false` when framed.
   */
  framed?: boolean;
  fill?: boolean;
  /**
   * `compact` — the rail rendition for 320px inspectors and dense panels:
   * 32px well with a 15px glyph, form-size title, micro description, 8px gaps.
   * `default` — the routed empty state at the canonical empty-h1 scale.
   */
  size?: "default" | "compact";
}

function isComponentType(value: unknown): value is IconComponent {
  if (typeof value === "function") return true;
  if (typeof value === "object" && value !== null && "render" in value) {
    return true;
  }
  return false;
}

function resolveTitleTag(title: React.ReactNode): EmptyTitleTag {
  return typeof title === "string" || typeof title === "number" ? "h3" : "div";
}

type EmptySize = NonNullable<EmptyProps["size"]>;

interface EmptyScale {
  gap: string;
  well: string;
  glyph: string;
  title: string;
  description: string;
}

const EMPTY_SCALES: Record<EmptySize, EmptyScale> = {
  default: {
    gap: "gap-3",
    well: "size-empty-icon rounded-lg bg-canvas-soft",
    glyph: "size-5",
    title: "text-empty-h1 tracking-empty-h1",
    description: "text-small-body leading-relaxed",
  },
  compact: {
    gap: "gap-2",
    well: "size-8 rounded-md bg-canvas-tint",
    glyph: "size-3.75",
    title: "text-form-label",
    description: "text-micro leading-4",
  },
};

function resolveIconContent(icon: EmptyProps["icon"], glyphClass: string): React.ReactNode {
  if (icon === undefined) return <BoxIcon className={glyphClass} />;
  if (isComponentType(icon)) {
    const IconComp = icon;
    return <IconComp className={glyphClass} />;
  }
  return icon;
}

function Empty({
  illustration,
  icon,
  title,
  titleAs,
  description,
  hint,
  cause,
  action,
  nextSteps,
  framed = false,
  fill,
  size = "default",
  className,
  ...props
}: EmptyProps) {
  const isFill = fill ?? !framed;
  const scale = EMPTY_SCALES[size];
  const iconContent = resolveIconContent(icon, scale.glyph);

  const titleTag = titleAs ?? resolveTitleTag(title);

  return (
    <div
      data-slot="empty"
      data-fill={isFill ? "true" : "false"}
      data-framed={framed ? "true" : undefined}
      data-size={size === "compact" ? "compact" : undefined}
      className={cn(
        "flex w-full flex-col items-center justify-center rounded-lg text-center",
        scale.gap,
        framed && "min-h-40 border border-line px-6 py-8",
        isFill && "h-full min-h-0 flex-1",
        className
      )}
      {...props}
    >
      {illustration ? (
        <div aria-hidden="true" data-slot="empty-illustration">
          {illustration}
        </div>
      ) : null}
      <span
        aria-hidden="true"
        data-slot="empty-icon"
        className={cn("inline-flex items-center justify-center text-subtle", scale.well)}
      >
        {iconContent}
      </span>
      {React.createElement(
        titleTag,
        {
          "data-slot": "empty-title",
          className: cn("font-medium leading-snug text-fg-strong", scale.title),
        },
        title
      )}
      {description ? (
        <p data-slot="empty-description" className={cn("max-w-md text-muted", scale.description)}>
          {description}
        </p>
      ) : null}
      {hint ? (
        <p data-slot="empty-hint" className="max-w-md text-small-body leading-relaxed text-subtle">
          {hint}
        </p>
      ) : null}
      {cause ? (
        <details data-slot="empty-cause" className="group w-full max-w-md text-left">
          <summary
            data-slot="empty-cause-summary"
            className="inline-flex cursor-pointer list-none items-center gap-1 rounded-sm text-small-body text-muted outline-none transition-colors duration-fast ease-out hover:text-fg focus-visible:shadow-focus-ring [&::-webkit-details-marker]:hidden"
          >
            <ChevronRightIcon
              aria-hidden="true"
              className="size-3.5 shrink-0 transition-transform duration-fast ease-out group-open:rotate-90"
            />
            Details
          </summary>
          <div
            data-slot="empty-cause-detail"
            className="mt-2 max-h-48 overflow-auto rounded border border-line bg-canvas px-3 py-2 font-mono text-badge leading-relaxed whitespace-pre-wrap break-words text-subtle"
          >
            {cause}
          </div>
        </details>
      ) : null}
      {action ? (
        <div
          data-slot="empty-action"
          className="mt-1 flex flex-wrap items-center justify-center gap-2"
        >
          {action}
        </div>
      ) : null}
      {nextSteps ? (
        <div
          data-slot="empty-next-steps"
          className="flex flex-wrap items-center justify-center gap-2"
        >
          {nextSteps}
        </div>
      ) : null}
    </div>
  );
}

export { Empty };
