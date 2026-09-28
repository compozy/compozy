import type { ReactNode } from "react";
import { History, Info, Lightbulb, ShieldAlert } from "lucide-react";

import {
  Alert,
  AlertDescription,
  AlertTitle,
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
import type { LoopQuarantineEntry } from "../../lib/loop-quarantine-entry";
import { LoopSection } from "../loop-section";
import { LoopQuarantineChain } from "./loop-quarantine-chain";

interface LoopQuarantineSheetProps {
  /** The quarantined node, or null when the sheet is closed. */
  node: LoopNodeLifecycle | null;
  open: boolean;
  isRequeuePending?: boolean;
  /** The run reached a terminal status, so the daemon rejects requeue and cancel. */
  runEnded?: boolean;
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
 * Requeue disappears the moment refreshed truth says the node left quarantine
 * or the run ended, so the sheet can never offer a verb the daemon would now
 * reject. The entry itself stays readable either way.
 */
export function LoopQuarantineSheet({
  node,
  open,
  isRequeuePending,
  runEnded = false,
  onOpenChange,
  onVerb,
  children,
}: LoopQuarantineSheetProps) {
  const entry = node?.quarantineEntry ?? null;
  return (
    <Sheet onOpenChange={onOpenChange} open={open}>
      <SheetContent
        className="flex w-full flex-col gap-0 p-0 sm:max-w-(--width-modal-sm)"
        data-testid="loop-quarantine-sheet"
      >
        {node && entry ? (
          <>
            <QuarantineSheetHeader entry={entry} node={node} />
            <ScrollArea className="min-h-0 flex-1">
              <div className="flex flex-col gap-5 px-5 py-4">
                {entry.hint ? (
                  <Alert data-testid="loop-quarantine-hint" variant="neutral">
                    <Lightbulb aria-hidden="true" />
                    <AlertTitle>What to try</AlertTitle>
                    <AlertDescription>{entry.hint}</AlertDescription>
                  </Alert>
                ) : null}
                <QuarantineFacts entry={entry} />
                <QuarantineChainSection entry={entry} />
              </div>
            </ScrollArea>
            <QuarantineSheetFooter
              isRequeuePending={isRequeuePending}
              node={node}
              onVerb={onVerb}
              runEnded={runEnded}
            />
            {children}
          </>
        ) : (
          <QuarantineSheetEmpty node={node} />
        )}
      </SheetContent>
    </Sheet>
  );
}

function QuarantineSheetHeader({
  node,
  entry,
}: {
  node: LoopNodeLifecycle;
  entry: LoopQuarantineEntry;
}) {
  return (
    <SheetHeader className="gap-2 border-b border-line px-5 py-4">
      <div className="flex items-start gap-3">
        <span
          aria-hidden="true"
          className="mt-0.5 inline-flex size-9 shrink-0 items-center justify-center rounded-md bg-danger-tint text-danger ring-1 ring-danger/24 ring-inset"
        >
          <ShieldAlert className="size-4" />
        </span>
        <div className="min-w-0">
          <Eyebrow className="text-danger">Set aside</Eyebrow>
          <SheetTitle className="mt-0.5 truncate" title={node.nodeId}>
            {node.label}
          </SheetTitle>
          <SheetDescription className="mt-1">
            {`Set aside after ${entry.attemptCount} ${
              entry.attemptCount === 1 ? "try" : "tries"
            } in round ${node.generation}`}
          </SheetDescription>
        </div>
      </div>
    </SheetHeader>
  );
}

function QuarantineFacts({ entry }: { entry: LoopQuarantineEntry }) {
  return (
    <LoopSection data-testid="loop-quarantine-facts" icon={<Info />} title="At a glance">
      <div className="grid grid-cols-2 gap-2">
        <MetadataTile
          label="Tries"
          value={entry.attemptCount}
          detail={`across ${entry.episodes.length} ${
            entry.episodes.length === 1 ? "time" : "times"
          }`}
        />
        <MetadataTile
          label="Times set aside"
          value={entry.episodes.length}
          detail={requeueDetail(entry.requeues.length)}
        />
        {entry.target ? <MetadataTile label="Target" value={entry.target} /> : null}
        {entry.quarantinedAt ? (
          <MetadataTile
            label="Set aside"
            value={formatRelativeTime(entry.quarantinedAt)}
            detail={formatAbsoluteTime(entry.quarantinedAt)}
          />
        ) : null}
      </div>
      {entry.inputRef ? (
        <div className="mt-2 rounded bg-canvas-soft px-3 py-2.5">
          <Eyebrow className="text-muted">Input</Eyebrow>
          <p className="mt-1 truncate font-mono text-mono-id text-fg">{entry.inputRef}</p>
        </div>
      ) : null}
    </LoopSection>
  );
}

function requeueDetail(requeues: number): string {
  if (requeues === 0) return "first time";
  return `after ${requeues} ${requeues === 1 ? "retry" : "retries"}`;
}

function QuarantineChainSection({ entry }: { entry: LoopQuarantineEntry }) {
  return (
    <LoopSection
      data-testid="loop-quarantine-chain"
      gist={countGist(entry.attemptCount, entry.episodes.length) || undefined}
      icon={<History />}
      title="What failed, in order"
    >
      <LoopQuarantineChain entry={entry} />
      {entry.truncated ? (
        <p className="mt-2 text-form-hint text-subtle">
          Older failures were dropped to keep this record short.
        </p>
      ) : null}
    </LoopSection>
  );
}

function QuarantineSheetFooter({
  node,
  runEnded,
  isRequeuePending,
  onVerb,
}: {
  node: LoopNodeLifecycle;
  runEnded: boolean;
  isRequeuePending?: boolean;
  onVerb: (verb: LoopNodeVerb, node: LoopNodeLifecycle) => void;
}) {
  const CancelIcon = LOOP_NODE_VERB_ICONS.cancel;
  return (
    <SheetFooter className="flex-row items-center justify-between gap-3 border-t border-line px-5 py-3">
      <span className="text-small-body text-muted" data-testid="loop-quarantine-foot">
        {runEnded
          ? "This run has ended. The entry is kept as a record."
          : "The run keeps working. Setting a step aside never stops it."}
      </span>
      <span className="flex shrink-0 items-center gap-2">
        {node.quarantined && !runEnded ? (
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
              Retry…
            </Button>
          </>
        ) : null}
      </span>
    </SheetFooter>
  );
}

function QuarantineSheetEmpty({ node }: { node: LoopNodeLifecycle | null }) {
  return (
    <div className="px-5 py-6">
      <SheetTitle>Nothing set aside</SheetTitle>
      <SheetDescription className="mt-1">
        {node
          ? `${node.label} was not set aside, so there is nothing to show. ` +
            "If a step is waiting on one that was set aside, open that step instead."
          : "This step has nothing to show. Pick a step that was set aside from the Needs you panel."}
      </SheetDescription>
    </div>
  );
}
