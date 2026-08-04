import { ConfirmDialog, Eyebrow } from "@compozy/ui";

import { loopRunVerbConfirmCopy } from "../../lib/loop-node-verb-copy";
import { loopStatusLabel } from "../../lib/loop-formatters";

/** The run-level verbs that require a confirmation before they commit. */
export type LoopRunConfirmVerb = "cancel" | "kill";

interface LoopRunControlDialogProps {
  /** The verb awaiting confirmation; null closes the dialog. */
  verb: LoopRunConfirmVerb | null;
  runId: string;
  /** The run's current status, restated in the strip the operator confirms against. */
  status: string;
  generation: number;
  isPending?: boolean;
  /** The daemon's answer when the last attempt was rejected; shown inline. */
  error?: string;
  onOpenChange: (open: boolean) => void;
  onConfirm: (verb: LoopRunConfirmVerb) => void;
}

/**
 * The run cancel / kill confirmation (PRD UI/UX: "every destructive or
 * state-changing action confirms with the current state it acts on").
 *
 * Both verbs land on the `canceled` terminal; the difference is what happens to
 * in-flight work, so the body says exactly that rather than leaving the operator
 * to infer it. The note strip restates the run's identity and *current* status,
 * so a stale screen cannot lead to a confident wrong click.
 */
export function LoopRunControlDialog({
  verb,
  runId,
  status,
  generation,
  isPending,
  error,
  onOpenChange,
  onConfirm,
}: LoopRunControlDialogProps) {
  if (!verb) return null;
  const copy = loopRunVerbConfirmCopy(verb, runId, status);
  return (
    <ConfirmDialog
      cancelLabel={copy.cancelLabel}
      confirmLabel={copy.confirmLabel}
      contentProps={{ "data-testid": "loop-run-control-dialog" }}
      description={copy.body}
      error={error}
      isPending={isPending}
      note={`${runId} is ${loopStatusLabel(status).toLowerCase()} · generation ${generation}`}
      noteTone="info"
      onConfirm={() => onConfirm(verb)}
      onOpenChange={onOpenChange}
      open
      title={
        <span className="flex flex-col gap-1">
          <Eyebrow className={verb === "kill" ? "text-danger" : "text-accent"}>
            {copy.eyebrow}
          </Eyebrow>
          <span>{copy.title}</span>
        </span>
      }
      tone={copy.tone}
      body={<p className="font-mono text-pill-group-badge text-faint">{copy.micro}</p>}
    />
  );
}
