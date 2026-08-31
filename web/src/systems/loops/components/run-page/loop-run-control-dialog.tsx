import { Info } from "lucide-react";

import { ConfirmDialog } from "@compozy/ui";

import type { LoopControlAnswer } from "../../lib/loop-node-controls";
import { LOOP_RUN_VERB_ICON_TONE, LOOP_RUN_VERB_ICONS } from "../../lib/loop-node-verb-icons";
import { loopRunStateStrip, loopRunVerbConfirmCopy } from "../../lib/loop-node-verb-copy";
import { LoopControlAnswerAlert } from "./loop-control-answer-alert";

/** The run-level verbs that require a confirmation before they commit. */
export type LoopRunConfirmVerb = "cancel";

interface LoopRunControlDialogProps {
  /** The verb awaiting confirmation; null closes the dialog. */
  verb: LoopRunConfirmVerb | null;
  runId: string;
  /** The run's current status, restated in the strip the operator confirms against. */
  status: string;
  generation: number;
  inFlightCount?: number;
  waitingOnYouCount?: number;
  elapsedLabel?: string;
  isPending?: boolean;
  /** Deterministic daemon answer — rendered as information in the body. */
  answer?: LoopControlAnswer;
  /** Transport failure only. Deterministic answers must not land here. */
  error?: string;
  onOpenChange: (open: boolean) => void;
  onConfirm: (verb: LoopRunConfirmVerb) => void;
}

/**
 * The run cancellation confirmation (PRD UI/UX: "every destructive or
 * state-changing action confirms with the current state it acts on").
 *
 * Cancellation lands on the `canceled` terminal and stops active work. The note
 * strip restates the run's identity and *current* status, so a stale screen
 * cannot lead to a confident wrong click.
 */
export function LoopRunControlDialog({
  verb,
  runId,
  status,
  generation,
  inFlightCount,
  waitingOnYouCount,
  elapsedLabel,
  isPending,
  answer,
  error,
  onOpenChange,
  onConfirm,
}: LoopRunControlDialogProps) {
  if (!verb) return null;
  const copy = loopRunVerbConfirmCopy(verb, runId, status);
  return (
    <ConfirmDialog
      cancelLabel={copy.cancelLabel}
      confirmButtonProps={{ variant: "destructive-solid" }}
      confirmLabel={copy.confirmLabel}
      contentProps={{
        "data-testid": "loop-run-control-dialog",
        className: "sm:max-w-(--width-modal-sm)",
      }}
      description={copy.body}
      error={error}
      eyebrow={copy.eyebrow}
      footNote={
        <>
          <Info aria-hidden="true" />
          <span>{copy.micro}</span>
        </>
      }
      icon={LOOP_RUN_VERB_ICONS[verb]}
      iconTone={LOOP_RUN_VERB_ICON_TONE[verb]}
      isPending={isPending}
      note={loopRunStateStrip({
        runId,
        status,
        generation,
        inFlightCount,
        waitingOnYouCount,
        elapsedLabel,
      })}
      noteTone="info"
      onConfirm={() => onConfirm(verb)}
      onOpenChange={onOpenChange}
      open
      title={copy.title}
      tone={copy.tone}
      body={answer ? <LoopControlAnswerAlert answer={answer} /> : null}
    />
  );
}
