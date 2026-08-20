import { useState } from "react";
import { CornerDownRight, Info } from "lucide-react";

import { ConfirmDialog, Eyebrow, Field, FieldLabel, Textarea } from "@compozy/ui";

import type { LoopControlAnswer } from "../../lib/loop-node-controls";
import type { LoopNodeLifecycle } from "../../lib/loop-node-lifecycle";
import { loopNodeStateStrip } from "../../lib/loop-node-verb-copy";
import { LOOP_NODE_VERB_ICON_TONE, LOOP_NODE_VERB_ICONS } from "../../lib/loop-node-verb-icons";
import type { LoopRerunSet } from "../../lib/loop-rerun-set";
import { LoopControlAnswerAlert } from "./loop-control-answer-alert";

export interface LoopNodeRerunDialogProps {
  open: boolean;
  node: LoopNodeLifecycle | null;
  rerunSet: LoopRerunSet;
  isPending?: boolean;

  answer?: LoopControlAnswer | null;
  onOpenChange: (open: boolean) => void;
  onConfirm: (input: { reason: string }) => void;
}

type OpenLoopNodeRerunDialogProps = Omit<LoopNodeRerunDialogProps, "node"> & {
  node: LoopNodeLifecycle;
};

const SET_LABEL_ID = "loop-rerun-set-label";

export function LoopNodeRerunDialog({ node, ...props }: LoopNodeRerunDialogProps) {
  if (!node) return null;

  const formKey = [node.nodeId, node.generation, node.revision].join(":");
  return <LoopNodeRerunDialogForm key={formKey} {...props} node={node} />;
}

function LoopNodeRerunDialogForm({
  open,
  node,
  rerunSet,
  isPending,
  answer,
  onOpenChange,
  onConfirm,
}: OpenLoopNodeRerunDialogProps) {
  const [reason, setReason] = useState("");
  const disabled = Boolean(isPending);
  const rerunNodes = rerunSet.rerunNodes;
  return (
    <ConfirmDialog
      cancelLabel="Cancel"
      confirmButtonProps={{
        "data-testid": "loop-rerun-confirm",
        disabled: disabled || rerunNodes.length === 0,
      }}
      confirmLabel="Rerun"
      contentProps={{ "data-testid": "loop-node-rerun-dialog" }}
      description="Opens a new generation. Everything else carries forward."
      eyebrow="Node"
      footNote={
        <>
          <Info aria-hidden="true" />
          <span>{`origin operator_rerun · from gen ${node.generation}`}</span>
        </>
      }
      icon={LOOP_NODE_VERB_ICONS.rerun}
      iconTone={LOOP_NODE_VERB_ICON_TONE.rerun}
      isPending={isPending}
      note={loopNodeStateStrip(node)}
      noteTone="info"
      onConfirm={() => onConfirm({ reason })}
      onOpenChange={onOpenChange}
      open={open}
      title="Rerun from here"
      tone="accent"
      body={
        <>
          {answer ? <LoopControlAnswerAlert answer={answer} /> : null}
          <section className="flex min-w-0 flex-col gap-1.5">
            <div className="flex items-baseline justify-between gap-2">
              <Eyebrow className="text-subtle" id={SET_LABEL_ID}>
                Rerun set
              </Eyebrow>
              <span className="font-mono text-mono-id text-faint">{countLabel(rerunNodes)}</span>
            </div>
            <div
              aria-labelledby={SET_LABEL_ID}
              className="flex max-h-52 min-w-0 flex-col overflow-y-auto rounded-md border border-line-soft bg-canvas-tint px-3 focus-visible:shadow-focus-ring focus-visible:outline-none"
              data-testid="loop-rerun-set"
              role="group"
              tabIndex={0}
            >
              {rerunNodes.length === 0 ? (
                <p className="py-2.5 text-form-hint text-muted">
                  This node has nothing to re-execute.
                </p>
              ) : (
                rerunNodes.map((nodeId, index) => (
                  <div
                    className="flex min-h-8 items-center gap-2 border-t border-line-soft py-1 first:border-t-0"
                    data-testid={`loop-rerun-node-${nodeId}`}
                    key={nodeId}
                  >
                    <CornerDownRight aria-hidden="true" className="size-3 shrink-0 text-faint" />
                    <span className="min-w-0 truncate font-mono text-form-label text-fg-strong">
                      {nodeId}
                    </span>
                    <span className="ml-auto shrink-0 text-form-hint text-subtle">
                      {index === 0 ? "start" : "dependent"}
                    </span>
                  </div>
                ))
              )}
            </div>
            <p className="text-form-hint text-muted" data-testid="loop-rerun-carried">
              {carriedLabel(rerunSet.carriedNodes.length)}
            </p>
            <p className="text-form-hint text-subtle">
              This preview comes from the pinned graph. CompozyOS returns the set it re-executes.
            </p>
          </section>
          <Field>
            <FieldLabel htmlFor="loop-rerun-reason">
              Reason <span className="text-muted">optional</span>
            </FieldLabel>
            <Textarea
              data-testid="loop-rerun-reason"
              disabled={disabled}
              id="loop-rerun-reason"
              onChange={event => setReason(event.target.value)}
              placeholder="Why this rerun"
              rows={2}
              value={reason}
            />
          </Field>
        </>
      }
    />
  );
}

function countLabel(rerunNodes: readonly string[]): string {
  return `${rerunNodes.length} ${rerunNodes.length === 1 ? "node" : "nodes"}`;
}

function carriedLabel(carried: number): string {
  if (carried === 0) return "Nothing else carries forward.";
  if (carried === 1) return "1 node carries forward unchanged.";
  return `${carried} nodes carry forward unchanged.`;
}
