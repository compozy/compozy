/**
 * Comments of the open prototype or flow: open/resolved filter, this state (step) or all of them,
 * and the threads. Selecting a comment jumps the canvas to where it was written. Prototype
 * comments are written on the canvas (comment mode); flow comments in the `composer` slot.
 */
import { XIcon } from "lucide-react";
import { useState, type ReactNode } from "react";

import { Alert, AlertDescription, Button, Empty, PillGroup, cn } from "@compozy/ui";

import { CommentCard } from "#gallery/shell/comments/comment-card";
import type { ReviewComment } from "#gallery/shell/comments/model";

type Place = { id: string; label: string };

type PanelProps = {
  comments: readonly ReviewComment[];
  /** The prototype's states. */
  places: readonly Place[];
  /** The state on screen. */
  place: string;
  selectedId: string | null;
  /** The API error, when the dev server cannot answer. */
  error: string | null;
  onSelect: (comment: ReviewComment | null) => void;
  onClose: () => void;
  /** `flow`: places are steps and new comments come from `composer`. */
  kind?: "prototype" | "flow";
  composer?: ReactNode;
  /** Shows the title row with the close button (off inside the flow side panel's tabs). */
  heading?: boolean;
  className?: string;
};

type Status = "open" | "resolved";
type Scope = "state" | "all";

export function CommentsPanel(props: PanelProps) {
  const { comments, place, selectedId } = props;
  const [status, setStatus] = useState<Status>("open");
  const [scope, setScope] = useState<Scope>("state");
  // The selected comment (pin click or link) always shows, whatever the filters; a flow's
  // whole-flow comments (no step) show beside every step.
  const inScope = (comment: ReviewComment) =>
    scope === "all" ||
    comment.context.state === place ||
    (props.kind === "flow" && !comment.context.state);
  const shown = comments.filter(
    comment => comment.id === selectedId || (comment.status === status && inScope(comment))
  );
  const open = comments.filter(comment => comment.status === "open").length;
  const labelOf = (id: string) => props.places.find(option => option.id === id)?.label ?? id;
  const flow = props.kind === "flow";
  const unit = flow ? "step" : "state";

  return (
    <aside
      aria-label="Comments"
      className={cn(
        "flex size-full flex-col gap-3 overflow-hidden rounded-xl bg-surface-2/50 p-3",
        props.className
      )}
      data-testid="comments-panel"
    >
      <div className={cn("flex items-center gap-2 px-1", props.heading === false && "hidden")}>
        <h2 className="text-small-body font-medium text-fg">Comments</h2>
        <span className="font-mono text-mono-id tracking-mono-id text-subtle">{open} open</span>
        <Button
          aria-label="Close comments"
          className="ml-auto"
          onClick={props.onClose}
          size="icon-sm"
          variant="quiet"
        >
          <XIcon aria-hidden="true" />
        </Button>
      </div>
      <div className="flex flex-wrap gap-1.5">
        <PillGroup<Status>
          aria-label="Status"
          items={[
            { value: "open", label: "Open" },
            { value: "resolved", label: "Resolved" },
          ]}
          onChange={setStatus}
          value={status}
        />
        <PillGroup<Scope>
          aria-label={flow ? "Steps" : "States"}
          items={[
            { value: "state", label: `This ${unit}` },
            { value: "all", label: `All ${unit}s` },
          ]}
          onChange={setScope}
          value={scope}
        />
      </div>
      {props.error ? (
        <Alert variant="warning">
          <AlertDescription>
            Comments are unavailable ({props.error}). Is `bun run proto dev` running?
          </AlertDescription>
        </Alert>
      ) : null}
      <ol className="-mx-1 flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto px-1 pb-1">
        {shown.length === 0 ? (
          <li>
            <Empty
              description={
                status === "resolved"
                  ? "Resolved comments show here."
                  : flow
                    ? "Write one below, about the selected step or the whole flow."
                    : "Press C, then click an element or drag an area of the canvas."
              }
              title={status === "resolved" ? "No resolved comments" : "No open comments"}
            />
          </li>
        ) : null}
        {shown.map(comment => (
          <CommentCard
            comment={comment}
            key={comment.id}
            onSelect={() => props.onSelect(comment.id === selectedId ? null : comment)}
            selected={comment.id === selectedId}
            stateLabel={
              flow && !comment.context.state ? "Whole flow" : labelOf(comment.context.state)
            }
          />
        ))}
      </ol>
      {props.composer}
    </aside>
  );
}
