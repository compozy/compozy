import type { ReactNode } from "react";
import { History, Info, Lightbulb, ShieldAlert } from "lucide-react";

import {
  Button,
  Eyebrow,
  formatAbsoluteTime,
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

import { LOOP_NODE_VERB_PRESENTATION, type LoopNodeVerb } from "../../lib/loop-node-controls";
import { LOOP_NODE_VERB_ICONS } from "../../lib/loop-node-verb-icons";
import type { LoopNodeLifecycle } from "../../lib/loop-node-lifecycle";
import { LoopSection } from "../loop-section";
import { LoopQuarantineChain } from "./loop-quarantine-chain";
import { LoopRunQuietNote } from "./loop-run-quiet-note";

interface LoopQuarantineSheetProps {
  /** The quarantined node, or null when the sheet is closed. */
  node: LoopNodeLifecycle | null;
  runId: string;
  open: boolean;
  isRequeuePending?: boolean;
  onOpenChange: (open: boolean) => void;
  onVerb: (verb: LoopNodeVerb, node: LoopNodeLifecycle) => void;
  /** Slot for the requeue confirm dialog, nested inside the sheet. */
  children?: ReactNode;
}

function countGist(attempts: number, episodes: number): string {
  return [
    attempts > 0 ? `${attempts} ${attempts === 1 ? "attempt" : "attempts"}` : null,
    episodes > 0 ? `${episodes} ${episodes === 1 ? "episode" : "episodes"}` : null,
  ]
    .filter(Boolean)
    .join(" · ");
}

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
  onVerb,
  children,
}: LoopQuarantineSheetProps) {
  const entry = node?.quarantineEntry ?? null;
  const CancelIcon = LOOP_NODE_VERB_ICONS.cancel;
  return (
    <Sheet onOpenChange={onOpenChange} open={open}>
      <SheetContent
        className="flex w-full flex-col gap-0 p-0 sm:max-w-(--width-modal-sm)"
        data-testid="loop-quarantine-sheet"
      >
        {node && entry ? (
          <>
            <SheetHeader className="gap-2 border-b border-line px-5 py-4">
              <div className="flex items-start gap-3">
                <span
                  aria-hidden="true"
                  className="mt-0.5 inline-flex size-9 shrink-0 items-center justify-center rounded-md bg-danger-tint text-danger ring-1 ring-danger/24 ring-inset"
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
                  <LoopRunQuietNote
                    data-testid="loop-quarantine-hint"
                    icon={Lightbulb}
                    title="What to try"
                  >
                    {entry.hint}
                  </LoopRunQuietNote>
                ) : null}
                <LoopSection
                  className="mb-0"
                  data-testid="loop-quarantine-facts"
                  icon={<Info />}
                  title="At a glance"
                >
                  <div className="grid grid-cols-2 gap-2">
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
                        detail={formatAbsoluteTime(entry.quarantinedAt)}
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
                </LoopSection>
                <LoopSection
                  className="mb-0"
                  data-testid="loop-quarantine-chain"
                  gist={countGist(entry.attemptCount, entry.episodes.length) || undefined}
                  icon={<History />}
                  title="What failed, in order"
                >
                  <LoopQuarantineChain entry={entry} />
                  {entry.truncated ? (
                    <p className="mt-1.5 text-form-hint text-subtle">
                      Older episodes were dropped to keep this entry inside its size limit.
                    </p>
                  ) : null}
                </LoopSection>
              </div>
            </ScrollArea>
            <SheetFooter className="flex-row items-center justify-between gap-3 border-t border-line px-5 py-3">
              <span className="text-small-body text-muted">
                The run keeps working — quarantine never stops it by itself.
              </span>
              <span className="flex shrink-0 items-center gap-2">
                {node.quarantined ? (
                  <>
                    <Button
                      data-testid="loop-quarantine-cancel"
                      onClick={() => onVerb("cancel", node)}
                      size="sm"
                      type="button"
                      variant="outline"
                    >
                      <CancelIcon className="size-3.5" />
                      {LOOP_NODE_VERB_PRESENTATION.cancel.label}
                    </Button>
                    <Button
                      data-testid="loop-quarantine-requeue"
                      disabled={isRequeuePending}
                      onClick={() => onVerb("requeue", node)}
                      size="sm"
                      type="button"
                      variant="primary"
                    >
                      Requeue…
                    </Button>
                  </>
                ) : null}
              </span>
            </SheetFooter>
            {children}
          </>
        ) : (
          <div className="px-5 py-6">
            <SheetTitle>No quarantine entry</SheetTitle>
            <SheetDescription className="mt-1">
              {node
                ? `${node.label} is not quarantined, so there is no repair record to show. ` +
                  "If a step is parked behind a quarantined one, open the entry from that step instead."
                : "This node has no repair record. Pick a quarantined step from the Needs attention panel."}
            </SheetDescription>
          </div>
        )}
      </SheetContent>
    </Sheet>
  );
}
