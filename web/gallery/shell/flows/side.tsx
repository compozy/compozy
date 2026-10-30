/**
 * Side panel of a flow: tabs between the selected step ("Step", the inspector) and the flow's
 * review comments ("Comments", `?comments=1`). Comments are written here about the selected step,
 * or the whole flow when none is selected; selecting one moves the canvas to its step.
 */
import { PanelRightCloseIcon } from "lucide-react";

import { Button, Tabs, TabsList, TabsTrigger } from "@compozy/ui";

import { flowKey, type FlowEntry, type ResolvedNode } from "#gallery/flow";
import { commentsApi, commentsAvailable, galleryCommit } from "#gallery/shell/comments/api";
import { FlowComposer } from "#gallery/shell/comments/flow-composer";
import type { NodeTarget, ReviewComment } from "#gallery/shell/comments/model";
import { CommentsPanel } from "#gallery/shell/comments/panel";
import { FlowInspector } from "#gallery/shell/flows/inspector";
import { stepLabel } from "#gallery/shell/flows/node-model";
import type { Registry } from "#gallery/shell/registry-model";
import { updateView, type Theme } from "#gallery/shell/view";

type FlowSideProps = {
  flow: FlowEntry;
  node: ResolvedNode | null;
  registry: Registry;
  comments: ReviewComment[];
  error: string | null;
  /** Comments tab on screen (`?comments=1`). */
  panel: boolean;
  selectedComment: string | null;
  theme: Theme | null;
  dark: boolean;
  onSelectNode: (id: string) => void;
  /** Collapses the whole side panel. */
  onClose: () => void;
};

/** Selecting a comment selects the step it was written on. */
function selectComment(comment: ReviewComment | null) {
  if (!comment) {
    updateView({ comment: null });
    return;
  }
  updateView({ comment: comment.id, comments: true, node: comment.context.state || null });
}

export function FlowSide(props: FlowSideProps) {
  const { flow, node, comments } = props;
  const key = flowKey(flow.id);
  const open = comments.filter(comment => comment.status === "open").length;
  const target: NodeTarget = {
    kind: "node",
    node: node?.id ?? null,
    label: node ? stepLabel(node) : "",
  };

  const create = async (body: string, attachments: string[]) => {
    const created = await commentsApi.create(key, {
      body,
      attachments,
      target,
      context: {
        commit: galleryCommit,
        state: node?.id ?? "",
        theme: props.theme ?? (props.dark ? "dark" : "light"),
        viewport: { width: window.innerWidth, height: window.innerHeight },
      },
    });
    updateView({ comment: created.id, comments: true });
  };

  return (
    <aside
      aria-label="Flow panel"
      className="flex size-full flex-col gap-3 overflow-hidden rounded-xl bg-surface-2/50 p-3"
      data-testid="flow-side"
    >
      <div className="flex items-center gap-2">
        <Tabs
          className="flex-1"
          onValueChange={(value: string) =>
            updateView({ comments: value === "comments", comment: null })
          }
          value={props.panel && commentsAvailable ? "comments" : "step"}
        >
          <TabsList aria-label="Flow panel">
            <TabsTrigger data-testid="flow-tab-step" value="step">
              Step
            </TabsTrigger>
            {commentsAvailable ? (
              <TabsTrigger count={open} data-testid="flow-tab-comments" value="comments">
                Comments
              </TabsTrigger>
            ) : null}
          </TabsList>
        </Tabs>
        <Button
          aria-label="Hide the flow panel"
          data-testid="flow-side-close"
          onClick={props.onClose}
          size="icon-sm"
          variant="quiet"
        >
          <PanelRightCloseIcon aria-hidden="true" />
        </Button>
      </div>
      {props.panel && commentsAvailable ? (
        <CommentsPanel
          className="rounded-none bg-transparent p-0"
          comments={comments}
          composer={
            <FlowComposer
              key={`${key}:${target.node}`}
              onSubmit={create}
              slug={key}
              target={target}
            />
          }
          error={props.error}
          heading={false}
          kind="flow"
          onClose={() => updateView({ comments: false, comment: null })}
          onSelect={selectComment}
          place={node?.id ?? ""}
          places={flow.nodes.map(item => ({ id: item.id, label: stepLabel(item) }))}
          selectedId={props.selectedComment}
        />
      ) : (
        <div
          className="flex min-h-0 flex-1 flex-col gap-3.5 overflow-y-auto px-1"
          data-testid="flow-inspector"
        >
          <FlowInspector
            flow={flow}
            node={node}
            onSelectNode={props.onSelectNode}
            registry={props.registry}
          />
        </div>
      )}
    </aside>
  );
}
