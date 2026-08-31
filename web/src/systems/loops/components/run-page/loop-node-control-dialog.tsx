import { useState } from "react";
import { Ban, Hourglass, Info } from "lucide-react";

import { ConfirmDialog, Label, RadioCard, Textarea } from "@compozy/ui";

import {
  LOOP_NODE_PAUSE_MODES,
  LOOP_NODE_RESUME_MODES,
  type LoopControlAnswer,
  type LoopNodePauseMode,
  type LoopNodeVerb,
} from "../../lib/loop-node-controls";
import { isOpenWait, type LoopNodeLifecycle } from "../../lib/loop-node-lifecycle";
import { LOOP_NODE_VERB_ICON_TONE, LOOP_NODE_VERB_ICONS } from "../../lib/loop-node-verb-icons";
import { loopNodeStateStrip, loopNodeVerbConfirmCopy } from "../../lib/loop-node-verb-copy";
import { checkLoopWaitPayload } from "../../lib/loop-node-wait-payload";
import { LoopControlAnswerAlert } from "./loop-control-answer-alert";

export interface LoopNodeVerbRequest {
  verb: LoopNodeVerb;
  node: LoopNodeLifecycle;
}

/** What the caller must POST once the operator confirms. */
export interface LoopNodeVerbCommit {
  verb: LoopNodeVerb;
  node: LoopNodeLifecycle;
  /** `drain`/`cancel` for pause, `plain`/`reset_attempts`/`immediate` for resume. */
  mode?: string;
  /** Raw JSON typed by the operator for a by-hand wait resume. */
  payload?: string;
  /** Optional provenance posted on pause and requeue. */
  reason?: string;
}

interface LoopNodeControlDialogProps {
  request: LoopNodeVerbRequest | null;
  isPending?: boolean;
  /** Deterministic daemon answer — rendered as information in the body. */
  answer?: LoopControlAnswer;
  /** Transport failure only. Deterministic answers must not land here. */
  error?: string;
  onOpenChange: (open: boolean) => void;
  onConfirm: (commit: LoopNodeVerbCommit) => void;
}

type OpenLoopNodeControlDialogProps = Omit<LoopNodeControlDialogProps, "request"> & {
  request: LoopNodeVerbRequest;
};

const PAUSE_MODE_COPY: Record<
  LoopNodePauseMode,
  { title: string; description: string; icon: typeof Hourglass }
> = {
  drain: {
    title: "Let the current attempt finish",
    description: "Nothing is interrupted. The lane parks once the attempt in flight settles.",
    icon: Hourglass,
  },
  cancel: {
    title: "Ask the current attempt to stop",
    description: "The in-flight attempt is asked to cancel, then the lane parks.",
    icon: Ban,
  },
};

const DIALOG_WIDTH = { className: "sm:max-w-(--width-modal-sm)" };

/**
 * The confirm surface for every node verb (VC-R3). It always shows the strip of
 * *current* daemon state it is acting on, so a stale screen cannot lead to a
 * confident wrong click, and it carries the daemon's own rejection text inline
 * rather than throwing it away in a toast.
 */
export function LoopNodeControlDialog({ request, ...props }: LoopNodeControlDialogProps) {
  if (!request) return null;
  const formKey = [
    request.verb,
    request.node.nodeId,
    request.node.generation,
    request.node.revision,
  ].join(":");
  return <LoopNodeControlDialogForm key={formKey} {...props} request={request} />;
}

function LoopNodeControlDialogForm({
  request,
  isPending,
  answer,
  error,
  onOpenChange,
  onConfirm,
}: OpenLoopNodeControlDialogProps) {
  const [pauseMode, setPauseMode] = useState<LoopNodePauseMode>("drain");
  const [payload, setPayload] = useState("");
  const [reason, setReason] = useState("");
  const copy = loopNodeVerbConfirmCopy(request.verb, request.node, { pauseMode });
  if (!copy) return null;
  const isPause = request.verb === "pause";
  const isRequeue = request.verb === "requeue";
  const isWaitResume = request.verb === "resume-wait";
  const offersReason = isPause || isRequeue;
  const waitExpect = request.node.waits.find(isOpenWait)?.expect;
  const waitCheck = isWaitResume ? checkLoopWaitPayload(payload, waitExpect) : null;
  const waitInvalid = waitCheck !== null && !waitCheck.ok;
  const resumeMode =
    request.verb === "resume" ||
    request.verb === "resume-reset-attempts" ||
    request.verb === "resume-immediate"
      ? LOOP_NODE_RESUME_MODES[request.verb]
      : undefined;
  const confirmDisabled = Boolean(isPending) || waitInvalid;
  return (
    <ConfirmDialog
      cancelLabel={copy.cancelLabel}
      confirmButtonProps={
        copy.tone === "danger"
          ? { disabled: confirmDisabled, variant: "destructive-solid" }
          : { disabled: confirmDisabled }
      }
      confirmLabel={copy.confirmLabel}
      contentProps={{ "data-testid": "loop-node-control-dialog", ...DIALOG_WIDTH }}
      description={copy.body}
      error={error}
      eyebrow={copy.eyebrow}
      footNote={
        <>
          <Info aria-hidden="true" />
          <span>{copy.micro}</span>
        </>
      }
      icon={LOOP_NODE_VERB_ICONS[request.verb]}
      iconTone={LOOP_NODE_VERB_ICON_TONE[request.verb]}
      isPending={isPending}
      note={loopNodeStateStrip(request.node)}
      noteTone="info"
      onConfirm={() =>
        onConfirm({
          verb: request.verb,
          node: request.node,
          mode: isPause ? LOOP_NODE_PAUSE_MODES[pauseMode] : resumeMode,
          payload: isWaitResume ? payload : undefined,
          reason: offersReason ? reason : undefined,
        })
      }
      onOpenChange={onOpenChange}
      open
      title={copy.title}
      tone={copy.tone}
      body={
        <>
          {answer ? <LoopControlAnswerAlert answer={answer} /> : null}
          {isPause ? (
            <fieldset className="flex flex-col gap-2">
              <legend className="sr-only">What happens to the attempt already in flight</legend>
              {(Object.keys(PAUSE_MODE_COPY) as LoopNodePauseMode[]).map(mode => (
                <RadioCard
                  data-testid={`loop-node-pause-mode-${mode}`}
                  description={PAUSE_MODE_COPY[mode].description}
                  icon={PAUSE_MODE_COPY[mode].icon}
                  iconWellSize="lg"
                  key={mode}
                  onSelect={() => setPauseMode(mode)}
                  selected={pauseMode === mode}
                  title={PAUSE_MODE_COPY[mode].title}
                />
              ))}
            </fieldset>
          ) : null}
          {isWaitResume ? (
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="loop-node-wait-payload">Payload</Label>
              <Textarea
                aria-invalid={waitInvalid || undefined}
                className="font-mono text-mono-id"
                data-testid="loop-node-wait-payload"
                id="loop-node-wait-payload"
                onChange={event => setPayload(event.target.value)}
                rows={4}
                value={payload}
              />
              {waitCheck?.hint ? (
                <p
                  className="font-mono text-form-hint text-subtle"
                  data-testid="loop-node-wait-expect"
                >
                  {waitCheck.hint}
                </p>
              ) : null}
              {waitCheck?.error ? (
                <p
                  className="text-form-hint text-danger"
                  data-testid="loop-node-wait-invalid"
                  role="alert"
                >
                  {waitCheck.error}
                </p>
              ) : null}
            </div>
          ) : null}
          {offersReason ? (
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="loop-node-reason">
                Reason <span className="text-muted">optional</span>
              </Label>
              <Textarea
                data-testid="loop-node-reason"
                id="loop-node-reason"
                onChange={event => setReason(event.target.value)}
                rows={3}
                value={reason}
              />
            </div>
          ) : null}
        </>
      }
    />
  );
}
