import type { ReactNode } from "react";

import { cn, StateGlyph, type StateGlyphState } from "@compozy/ui";

export type StepProgressStatus =
  | "pending"
  | "running"
  | "needs_input"
  | "done"
  | "failed"
  | "skipped";

export interface StepProgressItem {
  id: string;
  title: ReactNode;
  status: StepProgressStatus;
  /** Right-aligned metadata such as release tag (`v0.8.0 · Sigstore`) or timestamp (`2m ago`). */
  detail?: ReactNode;
  /** Quiet explanatory line or failure message directly beneath the title. */
  caption?: ReactNode;
  /** Compact single-line terminal/stderr preview on a failed step. */
  outputLine?: string;
  /** Optional inline action aligned right of the step title (e.g. `Retry`, `Open again`). */
  action?: ReactNode;
  /**
   * Inline prompt anchored beneath the active step (e.g. `Trust this host?` or
   * `SSH password or code`) so interactive prompts never stack a modal on a modal.
   */
  prompt?: ReactNode;
}

const STEP_GLYPH: Record<StepProgressStatus, StateGlyphState> = {
  pending: "queued",
  running: "running",
  needs_input: "attention",
  done: "done",
  failed: "failed",
  skipped: "stopped",
};

export interface StepProgressListProps {
  steps: readonly StepProgressItem[];
  className?: string;
  "data-testid"?: string;
}

/**
 * Connected vertical step list combining `MCPSignInSteps`'s 1px rail with
 * `WorktreePhaseSteps`'s `StateGlyph` semantics, plus an anchored inline prompt
 * slot under the active step (`research-product` §3.2).
 */
export function StepProgressList({
  steps,
  className,
  "data-testid": testId,
}: StepProgressListProps) {
  return (
    <ol
      aria-live="polite"
      className={cn("flex flex-col", className)}
      data-slot="step-progress-list"
      data-testid={testId}
    >
      {steps.map((step, index) => {
        const isCurrent = step.status === "running" || step.status === "needs_input";
        const isLast = index === steps.length - 1;
        return (
          <li
            className="relative flex gap-3 pb-3.5 last:pb-0"
            data-slot="step-progress-item"
            data-status={step.status}
            key={step.id}
          >
            {!isLast ? (
              <span
                aria-hidden="true"
                className="absolute top-5 bottom-0.5 left-[6.5px] w-px bg-line"
              />
            ) : null}
            <span className="relative mt-0.5 flex size-3.5 shrink-0 items-center justify-center">
              <StateGlyph size="md" state={STEP_GLYPH[step.status]} />
            </span>
            <div className="flex min-w-0 flex-1 flex-col gap-1">
              <div className="flex min-w-0 items-center justify-between gap-2">
                <span
                  aria-current={isCurrent ? "step" : undefined}
                  className={cn(
                    "min-w-0 truncate text-small-body",
                    isCurrent && "font-medium text-fg-strong",
                    step.status === "done" && "text-fg",
                    step.status === "pending" && "text-faint",
                    step.status === "skipped" && "text-subtle",
                    step.status === "failed" && "font-medium text-destructive"
                  )}
                >
                  {step.title}
                </span>
                <div className="flex shrink-0 items-center gap-2">
                  {step.detail ? (
                    <span className="text-eyebrow text-faint tabular-nums">{step.detail}</span>
                  ) : null}
                  {step.action}
                </div>
              </div>
              {step.caption ? (
                <p
                  className={cn(
                    "text-meta",
                    step.status === "failed" ? "text-destructive" : "text-muted-foreground"
                  )}
                >
                  {step.caption}
                </p>
              ) : null}
              {step.outputLine ? (
                <div className="mt-0.5 overflow-x-auto rounded-sm border border-line bg-code-bg px-2.5 py-1.5 font-mono text-mono-id text-muted-foreground">
                  {step.outputLine}
                </div>
              ) : null}
              {step.prompt ? <div className="mt-1.5">{step.prompt}</div> : null}
            </div>
          </li>
        );
      })}
    </ol>
  );
}
