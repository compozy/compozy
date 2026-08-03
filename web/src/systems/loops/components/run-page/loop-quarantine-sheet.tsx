import type { ReactNode } from "react";
import { Lightbulb, ShieldAlert } from "lucide-react";

import {
  Button,
  Eyebrow,
  formatRelativeTime,
  MetadataTile,
  ScrollArea,
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@compozy/ui";

import type { LoopNodeLifecycle } from "../../lib/loop-node-lifecycle";
import { quarantineChainRows } from "../../lib/loop-quarantine-entry";

interface LoopQuarantineSheetProps {
  /** The quarantined node, or null when the sheet is closed. */
  node: LoopNodeLifecycle | null;
  runId: string;
  open: boolean;
  isRequeuePending?: boolean;
  onOpenChange: (open: boolean) => void;
  onRequeue: (node: LoopNodeLifecycle) => void;
  /** Slot for the requeue confirm dialog, nested inside the sheet. */
  children?: ReactNode;
}

const DISPOSITION_TONE: Record<string, string> = {
  retried: "text-warning",
  routed: "text-info",
  absorbed: "text-muted",
  escalated: "text-warning",
  quarantined: "text-danger",
  canceled: "text-muted",
};

/**
 * The quarantine entry sheet (US-024 AC-1, VC-R4). It renders exactly what the
 * daemon retained in the entry: the remediation hint first (it is the only thing
 * that tells the operator what to do), the at-a-glance facts, and the classified
 * attempt chain in order with its episode boundaries.
 *
 * Nothing is synthesized. If the entry carries no hint, no hint section renders;
 * if an attempt recorded no cause, its line is just the class and disposition.
 * Requeue disappears the moment refreshed truth says the node left quarantine,
 * so the sheet can never offer a verb the daemon would now reject.
 */
export function LoopQuarantineSheet({
  node,
  runId,
  open,
  isRequeuePending,
  onOpenChange,
  onRequeue,
  children,
}: LoopQuarantineSheetProps) {
  const entry = node?.quarantineEntry ?? null;
  return (
    <Sheet onOpenChange={onOpenChange} open={open}>
      <SheetContent
        className="flex w-full flex-col gap-0 p-0 sm:max-w-160"
        data-testid="loop-quarantine-sheet"
      >
        {node && entry ? (
          <>
            <SheetHeader className="gap-2 border-b border-line px-5 py-4">
              <div className="flex items-start gap-3">
                <span
                  aria-hidden="true"
                  className="mt-0.5 inline-flex size-8 shrink-0 items-center justify-center rounded bg-danger-tint text-danger"
                >
                  <ShieldAlert className="size-4" />
                </span>
                <div className="min-w-0">
                  <Eyebrow className="text-danger">Quarantine entry</Eyebrow>
                  <SheetTitle className="mt-0.5 truncate">{node.nodeId}</SheetTitle>
                  <SheetDescription className="mt-1">
                    {`Set aside after ${entry.attemptCount} ${
                      entry.attemptCount === 1 ? "attempt" : "attempts"
                    } across ${entry.episodes.length} ${
                      entry.episodes.length === 1 ? "episode" : "episodes"
                    }`}
                    <span className="ml-2 font-mono text-mono-id text-faint">
                      gen {node.generation} · {runId}
                    </span>
                  </SheetDescription>
                </div>
              </div>
            </SheetHeader>
            <ScrollArea className="min-h-0 flex-1">
              <div className="flex flex-col gap-5 px-5 py-4">
                {entry.hint ? (
                  <section
                    className="rounded-lg border border-line bg-canvas-soft px-4 py-3.5"
                    data-testid="loop-quarantine-hint"
                  >
                    <div className="flex items-center gap-2">
                      <Lightbulb aria-hidden="true" className="size-3.5 text-warning" />
                      <span className="text-ws-name font-medium text-fg-strong">What to try</span>
                    </div>
                    <p className="mt-1.5 max-w-[62ch] text-small-body leading-relaxed text-muted">
                      {entry.hint}
                    </p>
                  </section>
                ) : null}
                <section data-testid="loop-quarantine-facts">
                  <Eyebrow className="text-muted">At a glance</Eyebrow>
                  <div className="mt-2 grid grid-cols-2 gap-2">
                    <MetadataTile
                      label="Attempts"
                      value={entry.attemptCount}
                      detail={`across ${entry.episodes.length} ${
                        entry.episodes.length === 1 ? "episode" : "episodes"
                      }`}
                    />
                    <MetadataTile
                      label="Episode"
                      value={entry.episodes.length}
                      detail={
                        entry.requeues.length > 0
                          ? `after ${entry.requeues.length} ${
                              entry.requeues.length === 1 ? "requeue" : "requeues"
                            }`
                          : "first quarantine"
                      }
                    />
                    {entry.target ? (
                      <MetadataTile label="External target" value={entry.target} />
                    ) : null}
                    {entry.quarantinedAt ? (
                      <MetadataTile
                        label="Quarantined"
                        value={formatRelativeTime(entry.quarantinedAt)}
                        detail={entry.quarantinedAt}
                      />
                    ) : null}
                  </div>
                  {entry.inputRef ? (
                    <div className="mt-2 rounded bg-canvas-soft px-3 py-2.5">
                      <Eyebrow className="text-muted">Input</Eyebrow>
                      <p className="mt-1 truncate font-mono text-mono-id text-fg">
                        {entry.inputRef}
                      </p>
                    </div>
                  ) : null}
                </section>
                <section data-testid="loop-quarantine-chain">
                  <div className="flex items-baseline gap-2">
                    <Eyebrow className="text-muted">What failed, in order</Eyebrow>
                    <span className="font-mono text-pill-group-badge text-faint">
                      {entry.attemptCount} attempts · {entry.episodes.length} episodes
                    </span>
                  </div>
                  <ol className="mt-2 flex flex-col rounded-lg border border-line bg-canvas-soft">
                    {quarantineChainRows(entry).map((row, index) => (
                      <li
                        className={index > 0 ? "border-t border-line-soft" : undefined}
                        data-testid={`loop-quarantine-attempt-${row.key}`}
                        key={row.key}
                      >
                        {row.openedBy ? (
                          <div className="border-b border-line-soft px-4 py-1.5">
                            <Eyebrow className="text-faint">
                              {`Episode ${row.episodeIndex + 1} — requeued by ${
                                row.openedBy.actorId || row.openedBy.actorKind || "an operator"
                              }`}
                              {row.openedBy.requestedAt
                                ? ` ${formatRelativeTime(row.openedBy.requestedAt)}`
                                : ""}
                            </Eyebrow>
                          </div>
                        ) : null}
                        <div className="flex items-start gap-3 px-4 py-3">
                          <div className="min-w-0 flex-1">
                            <div className="text-ws-name font-medium text-fg-strong">
                              {row.cause || row.failureCode || "An attempt failed"}
                            </div>
                            {row.hint ? (
                              <p className="mt-1 max-w-[58ch] text-small-body leading-relaxed text-muted">
                                {row.hint}
                              </p>
                            ) : null}
                          </div>
                          <div className="shrink-0 text-right">
                            <div className="font-mono text-mono-id tabular-nums text-subtle">
                              attempt {row.attempt}
                              {row.endedAt ? ` · ${formatRelativeTime(row.endedAt)}` : ""}
                            </div>
                            <div
                              className={`font-mono text-pill-group-badge ${
                                DISPOSITION_TONE[row.disposition ?? ""] ?? "text-faint"
                              }`}
                            >
                              {[row.failureClass, row.disposition].filter(Boolean).join(" → ")}
                            </div>
                          </div>
                        </div>
                      </li>
                    ))}
                  </ol>
                  {entry.truncated ? (
                    <p className="mt-1.5 text-form-hint text-subtle">
                      Older episodes were dropped to keep this entry inside its size limit.
                    </p>
                  ) : null}
                </section>
              </div>
            </ScrollArea>
            <SheetFooter className="flex-row items-center justify-between gap-3 border-t border-line px-5 py-3">
              <span className="text-small-body text-muted">
                The run keeps working — quarantine never stops it by itself.
              </span>
              <span className="flex shrink-0 items-center gap-2">
                <Button onClick={() => onOpenChange(false)} size="sm" type="button" variant="ghost">
                  Close
                </Button>
                {node.quarantined ? (
                  <Button
                    data-testid="loop-quarantine-requeue"
                    disabled={isRequeuePending}
                    onClick={() => onRequeue(node)}
                    size="sm"
                    type="button"
                    variant="primary"
                  >
                    Requeue…
                  </Button>
                ) : null}
              </span>
            </SheetFooter>
            {children}
          </>
        ) : (
          <div className="px-5 py-6">
            <SheetTitle>No quarantine entry</SheetTitle>
            <SheetDescription className="mt-1">
              The daemon has not recorded a repair record for this node.
            </SheetDescription>
          </div>
        )}
      </SheetContent>
    </Sheet>
  );
}
