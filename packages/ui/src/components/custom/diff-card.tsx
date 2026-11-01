import type * as React from "react";

import { cn } from "../../lib/utils";

export type DiffCardLineKind = "add" | "del" | "context";

export interface DiffCardLine {
  /** Stable identity, for example the line's old/new positions (`d3`, `a4`, `c3:4`). */
  id: string;
  kind: DiffCardLineKind;
  text: string;
}

export interface DiffCardProps extends Omit<React.ComponentProps<"div">, "children"> {
  /** The changed file, shown mono in the header. */
  path?: React.ReactNode;
  /** Header stat; the one place additions and deletions take color. */
  additions?: number;
  deletions?: number;
  lines: readonly DiffCardLine[];
  /** Below the lines: a "show more" control, a truncation note. */
  footer?: React.ReactNode;
}

const LINE_TONE: Record<
  DiffCardLineKind,
  { row: string; bar: string; sign: string; mark: string }
> = {
  add: { row: "bg-success-tint", bar: "bg-success", sign: "text-success", mark: "+" },
  del: { row: "bg-destructive-tint", bar: "bg-destructive", sign: "text-destructive", mark: "−" },
  context: { row: "", bar: "", sign: "text-faint", mark: " " },
};

/**
 * A file change as one quiet card: a header with the path and a colored
 * `+a −d` stat, then the changed lines — added and removed lines on a faint
 * tint with a 2px bar on their leading edge and a sign column, context lines
 * in the subtle ink. Long lines scroll horizontally inside the card; the card
 * bounds its own height.
 */
export function DiffCard({
  path,
  additions,
  deletions,
  lines,
  footer,
  className,
  ...props
}: DiffCardProps) {
  const hasStat = additions !== undefined || deletions !== undefined;
  return (
    <div
      data-slot="diff-card"
      className={cn(
        "flex min-w-0 flex-col overflow-hidden rounded-lg border border-line-soft bg-chat-fill-code",
        className
      )}
      {...props}
    >
      {path || hasStat ? (
        <div
          data-slot="diff-card-header"
          className="flex min-w-0 items-center gap-2 border-b border-line-soft px-2.5 py-1.5"
        >
          {path ? (
            <span className="min-w-0 flex-1 truncate font-mono text-transcript-caption font-medium text-muted-foreground">
              {path}
            </span>
          ) : (
            <span className="flex-1" />
          )}
          {hasStat ? (
            <span
              data-slot="diff-card-stat"
              className="flex shrink-0 items-center gap-1.5 font-mono text-transcript-caption font-medium tabular-nums"
            >
              <span className="text-success">+{additions ?? 0}</span>
              <span className="text-destructive">−{deletions ?? 0}</span>
            </span>
          ) : null}
        </div>
      ) : null}
      <div data-slot="diff-card-lines" className="max-h-60 overflow-auto py-1 select-text">
        <div className="w-max min-w-full">
          {lines.map(line => {
            const tone = LINE_TONE[line.kind];
            return (
              <div
                key={line.id}
                data-slot="diff-card-line"
                data-kind={line.kind}
                className={cn("relative flex items-baseline pr-3", tone.row)}
              >
                {tone.bar ? (
                  <span
                    aria-hidden="true"
                    className={cn("absolute inset-y-0 left-0 w-0.5", tone.bar)}
                  />
                ) : null}
                <span
                  aria-hidden="true"
                  className={cn(
                    "w-6 shrink-0 text-center font-mono text-transcript-caption font-bold",
                    tone.sign
                  )}
                >
                  {tone.mark}
                </span>
                <span
                  className={cn(
                    "font-mono text-transcript-caption leading-[18px] whitespace-pre",
                    line.kind === "context" ? "text-subtle" : "text-muted-foreground"
                  )}
                >
                  {line.text.length > 0 ? line.text : " "}
                </span>
              </div>
            );
          })}
        </div>
      </div>
      {footer ? (
        <div data-slot="diff-card-footer" className="border-t border-line-soft px-2.5 py-1">
          {footer}
        </div>
      ) : null}
    </div>
  );
}
