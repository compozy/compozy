import { useState } from "react";

import { ConfirmDialog, Eyebrow, Label, RadioCard, Textarea } from "@compozy/ui";

import {
  LOOP_NODE_PAUSE_MODES,
  LOOP_NODE_RESUME_MODES,
  type LoopNodePauseMode,
  type LoopNodeVerb,
} from "../../lib/loop-node-controls";
import type { LoopNodeLifecycle } from "../../lib/loop-node-lifecycle";
import { loopNodeStateStrip, loopNodeVerbConfirmCopy } from "../../lib/loop-node-verb-copy";

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
}

interface LoopNodeControlDialogProps {
  request: LoopNodeVerbRequest | null;
  isPending?: boolean;
  /** The daemon's answer when the last attempt was rejected; shown inline. */
  error?: string;
  onOpenChange: (open: boolean) => void;
  onConfirm: (commit: LoopNodeVerbCommit) => void;
}

type OpenLoopNodeControlDialogProps = Omit<LoopNodeControlDialogProps, "request"> & {
  request: LoopNodeVerbRequest;
};

const PAUSE_MODE_COPY: Record<LoopNodePauseMode, { title: string; description: string }> = {
  drain: {
    title: "Let the current attempt finish",
    description: "Nothing is interrupted. The lane parks once the attempt in flight settles.",
  },
  cancel: {
    title: "Ask the current attempt to stop",
    description: "The in-flight attempt is asked to cancel, then the lane parks.",
  },
};

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
  error,
  onOpenChange,
  onConfirm,
}: OpenLoopNodeControlDialogProps) {
  const [pauseMode, setPauseMode] = useState<LoopNodePauseMode>("drain");
  const [payload, setPayload] = useState("");
  const copy = loopNodeVerbConfirmCopy(request.verb, request.node);
  if (!copy) return null;
  const isPause = request.verb === "pause";
  const isWaitResume = request.verb === "resume-wait";
  const resumeMode =
    request.verb === "resume" ||
    request.verb === "resume-reset-attempts" ||
    request.verb === "resume-immediate"
      ? LOOP_NODE_RESUME_MODES[request.verb]
      : undefined;
  return (
    <ConfirmDialog
      cancelLabel={copy.cancelLabel}
      confirmLabel={copy.confirmLabel}
      contentProps={{ "data-testid": "loop-node-control-dialog" }}
      description={copy.body}
      error={error}
      isPending={isPending}
      note={loopNodeStateStrip(request.node)}
      noteTone="info"
      onConfirm={() =>
        onConfirm({
          verb: request.verb,
          node: request.node,
          mode: isPause ? LOOP_NODE_PAUSE_MODES[pauseMode] : resumeMode,
          payload: isWaitResume ? payload : undefined,
        })
      }
      onOpenChange={onOpenChange}
      open
      title={
        <span className="flex flex-col gap-1">
          <Eyebrow className="text-accent">{copy.eyebrow}</Eyebrow>
          <span>{copy.title}</span>
        </span>
      }
      tone={copy.tone}
      body={
        <>
          {isPause ? (
            <fieldset className="flex flex-col gap-2">
              <legend className="sr-only">What happens to the attempt already in flight</legend>
              {(Object.keys(PAUSE_MODE_COPY) as LoopNodePauseMode[]).map(mode => (
                <RadioCard
                  data-testid={`loop-node-pause-mode-${mode}`}
                  description={PAUSE_MODE_COPY[mode].description}
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
              <Label htmlFor="loop-node-wait-payload">
                Payload{" "}
                <span className="text-muted">validated against the wait&apos;s expectation</span>
              </Label>
              <Textarea
                className="font-mono text-mono-id"
                data-testid="loop-node-wait-payload"
                id="loop-node-wait-payload"
                onChange={event => setPayload(event.target.value)}
                rows={4}
                value={payload}
              />
              <p className="text-form-hint text-subtle">
                One resume wins — a concurrent event claim answers with the winner.
              </p>
            </div>
          ) : null}
          <p className="font-mono text-pill-group-badge text-faint">{copy.micro}</p>
        </>
      }
    />
  );
}
