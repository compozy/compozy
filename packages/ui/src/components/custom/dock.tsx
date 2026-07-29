"use client";

import * as React from "react";

import { cn } from "../../lib/utils";
import { Eyebrow } from "./eyebrow";

export interface DockProps extends React.ComponentProps<"div"> {
  children: React.ReactNode;
}

/**
 * Decision panel fused to the composer top — permission and clarification
 * surfaces leave the transcript and dock here. The shell shares its border
 * with the composer below (open bottom, top radius only); emphasis comes from
 * geometry and the eyebrow, never from a tinted wash.
 */
function DockRoot({ children, className, ...props }: DockProps) {
  return (
    <div
      data-slot="dock"
      className={cn(
        "rounded-t-lg border border-b-0 border-line-strong bg-canvas-soft",
        "px-3.5 pt-[11px] pb-3",
        className
      )}
      {...props}
    >
      {children}
    </div>
  );
}

function DockHead({ children, className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dock-head"
      className={cn("flex flex-wrap items-center gap-2", className)}
      {...props}
    >
      {children}
    </div>
  );
}

/** Uppercase kicker naming the decision kind ("Permission", "Question"). */
function DockEyebrow({ children, className, ...props }: React.ComponentProps<"span">) {
  return (
    <Eyebrow data-slot="dock-eyebrow" className={cn("text-subtle", className)} {...props}>
      {children}
    </Eyebrow>
  );
}

function DockTitle({ children, className, ...props }: React.ComponentProps<"span">) {
  return (
    <span
      data-slot="dock-title"
      className={cn("text-small-body font-medium text-fg", className)}
      {...props}
    >
      {children}
    </span>
  );
}

/** Faint mono counter ("1/2") for stacked pending decisions. */
function DockCount({ children, className, ...props }: React.ComponentProps<"span">) {
  return (
    <span
      data-slot="dock-count"
      className={cn("font-mono text-[10px] text-faint tabular-nums", className)}
      {...props}
    >
      {children}
    </span>
  );
}

/** Static mono deadline hint ("times out 14:32") — it never ticks. */
function DockDeadline({ children, className, ...props }: React.ComponentProps<"span">) {
  return (
    <span
      data-slot="dock-deadline"
      className={cn("ml-auto font-mono text-[10px] text-faint tabular-nums", className)}
      {...props}
    >
      {children}
    </span>
  );
}

function DockBody({ children, className, ...props }: React.ComponentProps<"div">) {
  return (
    <div data-slot="dock-body" className={cn("mt-2", className)} {...props}>
      {children}
    </div>
  );
}

/** Mono command/subject block on the code wash — what the decision is about. */
function DockPre({ children, className, ...props }: React.ComponentProps<"pre">) {
  return (
    <pre
      data-slot="dock-pre"
      className={cn(
        "max-h-[130px] overflow-auto rounded-md border border-line bg-chat-fill-code",
        "px-2.5 py-2 font-mono text-[11px] leading-relaxed text-fg",
        "break-words whitespace-pre-wrap",
        className
      )}
      {...props}
    >
      {children}
    </pre>
  );
}

function DockMeta({ children, className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dock-meta"
      className={cn(
        "mt-[5px] text-[11px] text-subtle",
        "[&_code]:font-mono [&_code]:text-badge [&_code]:text-muted",
        className
      )}
      {...props}
    >
      {children}
    </div>
  );
}

function DockActions({ children, className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dock-actions"
      className={cn("mt-2.5 flex items-center gap-[7px]", className)}
      {...props}
    >
      {children}
    </div>
  );
}

export interface DockStatusProps extends React.ComponentProps<"div"> {
  /** `danger` for retryable errors; default is the quiet submitting line. */
  tone?: "neutral" | "danger";
}

/** Quiet status line under the actions — submitting / retryable error. */
function DockStatus({ tone = "neutral", children, className, ...props }: DockStatusProps) {
  return (
    <div
      data-slot="dock-status"
      data-tone={tone}
      className={cn(
        "mt-[7px] text-[11px]",
        tone === "danger" ? "text-danger" : "text-subtle",
        className
      )}
      {...props}
    >
      {children}
    </div>
  );
}

/** Keyboard shortcut chip on a dock action button ("1"–"4"). */
function DockKey({ children, className, ...props }: React.ComponentProps<"kbd">) {
  return (
    <kbd
      data-slot="dock-key"
      className={cn("ml-[3px] font-mono text-[9px] text-current opacity-60", className)}
      {...props}
    >
      {children}
    </kbd>
  );
}

const Dock = Object.assign(DockRoot, {
  Head: DockHead,
  Eyebrow: DockEyebrow,
  Title: DockTitle,
  Count: DockCount,
  Deadline: DockDeadline,
  Body: DockBody,
  Pre: DockPre,
  Meta: DockMeta,
  Actions: DockActions,
  Status: DockStatus,
  Key: DockKey,
});

export { Dock };
