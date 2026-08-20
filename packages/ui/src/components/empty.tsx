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
  className,
  ...props
}: EmptyProps) {
  const isFill = fill ?? !framed;
  let iconContent: React.ReactNode;
  if (icon === undefined) {
    iconContent = <BoxIcon className="size-5" />;
  } else if (isComponentType(icon)) {
    const IconComp = icon;
    iconContent = <IconComp className="size-5" />;
  } else {
    iconContent = icon;
  }

  const titleTag = titleAs ?? resolveTitleTag(title);

  return (
    <div
      data-slot="empty"
      data-fill={isFill ? "true" : "false"}
      data-framed={framed ? "true" : undefined}
      className={cn(
        "flex w-full flex-col items-center justify-center gap-3 rounded-lg text-center",
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
        className="inline-flex size-empty-icon items-center justify-center rounded-lg bg-canvas-soft text-subtle"
      >
        {iconContent}
      </span>
      {React.createElement(
        titleTag,
        {
          "data-slot": "empty-title",
          className: "text-empty-h1 font-medium leading-snug tracking-empty-h1 text-fg-strong",
        },
        title
      )}
      {description ? (
        <p
          data-slot="empty-description"
          className="max-w-md text-small-body leading-relaxed text-muted"
        >
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
