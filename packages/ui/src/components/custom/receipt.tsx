"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

export type ReceiptTone = "neutral" | "allowed" | "rejected";

export interface ReceiptProps extends React.ComponentProps<"div"> {
  /** Outcome tone carried by the glyph only — allowed check, rejected ×. */
  tone?: ReceiptTone;
  /** 12px leading glyph (lucide icon element). */
  icon?: React.ReactNode;
  children: React.ReactNode;
}

const TONE_ICON: Record<ReceiptTone, string> = {
  neutral: "text-faint",
  allowed: "text-success",
  rejected: "text-danger",
};

/**
 * One-line decision record in the transcript — the durable trace a resolved
 * permission or clarification leaves behind once the decision surface docks to
 * the composer. Same quiet grammar as a marker: glyph + one sentence; `<b>`
 * carries the subject, `<code>` the tool or command in mono.
 */
function Receipt({ tone = "neutral", icon, children, className, ...props }: ReceiptProps) {
  return (
    <div
      data-slot="receipt"
      data-tone={tone}
      className={cn(
        "flex min-h-transcript-row min-w-0 items-center gap-transcript-inline-gap px-1 py-0.5",
        "text-transcript-body text-subtle",
        className
      )}
      {...props}
    >
      {icon ? (
        <span
          aria-hidden="true"
          data-slot="receipt-icon"
          className={cn("shrink-0 *:[svg]:size-3", TONE_ICON[tone])}
        >
          {icon}
        </span>
      ) : null}
      <span
        data-slot="receipt-body"
        className={cn(
          "min-w-0 truncate",
          "[&_b]:font-medium [&_b]:text-muted [&_strong]:font-medium [&_strong]:text-muted",
          "[&_code]:font-mono [&_code]:text-transcript-caption [&_code]:text-muted"
        )}
      >
        {children}
      </span>
    </div>
  );
}

export { Receipt };
