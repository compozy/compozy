/**
 * Prototype gallery at `/`: a collapsible drill-down sidebar and a main area that renders the
 * selected level: the specs index, a spec overview (`?spec=`), a prototype in one state or a
 * journey flow (`?flow=`). The
 * whole view lives in the URL (`view.ts`); keyboard shortcuts in `use-gallery-shortcuts.ts`.
 */
import { QueryClientProvider } from "@tanstack/react-query";
import { useRef } from "react";

import { Empty, Toaster } from "@compozy/ui";

import { flowKey, type FlowEntry } from "#gallery/flow";
import { useTheme } from "#gallery/render/use-theme";
import { galleryQueryClient } from "#gallery/shell/comments/api";
import { useCommentReview } from "#gallery/shell/comments/use-comment-review";
import { useComments } from "#gallery/shell/comments/use-comments";
import { FlowToolbar } from "#gallery/shell/flow-toolbar";
import { FlowMembership } from "#gallery/shell/flows/membership";
import { FlowView } from "#gallery/shell/flows/flow-view";
import { IndexOverview, SpecOverview } from "#gallery/shell/overview";
import { PrototypeView } from "#gallery/shell/prototype-view";
import type { Entry } from "#gallery/shell/registry-model";
import { registry } from "#gallery/shell/registry";
import { GallerySidebar } from "#gallery/shell/sidebar";
import {
  useGalleryShortcuts,
  useGalleryState,
  type GalleryState,
} from "#gallery/shell/use-gallery-shortcuts";
import { step, useGallerySelection, type GallerySelection } from "#gallery/shell/use-selection";
import { select } from "#gallery/shell/view";

/** The prototype level, with its review comments in the view's slots. */
function ReviewedPrototype({
  current,
  selection,
  ui,
  dark,
}: {
  current: Entry;
  selection: GallerySelection;
  ui: GalleryState;
  dark: boolean;
}) {
  const review = useCommentReview({
    entry: current,
    view: selection.view,
    state: selection.state,
    dark,
    frame: ui.frame,
    picking: ui.picking,
    onPicking: ui.setPicking,
  });
  return (
    <PrototypeView
      dark={dark}
      entry={current}
      group={selection.group}
      helpOpen={ui.helpOpen}
      onHelp={ui.setHelpOpen}
      onInfo={info => {
        ui.setFrame(info);
        // A link inside the prototype opened another one: follow it in the gallery.
        if (
          info &&
          info.slug !== current.slug &&
          registry.entries.some(e => e.slug === info.slug)
        ) {
          select({ kind: "prototype", slug: info.slug, state: info.state });
        }
      }}
      onStep={direction => {
        const next = step(ui.navigable, current, direction);
        if (next) select({ kind: "prototype", slug: next.slug });
      }}
      review={{
        ...review,
        actions: (
          <>
            <FlowMembership entry={current} registry={registry} state={selection.state} />
            {review.actions}
          </>
        ),
      }}
      state={selection.state}
      view={selection.view}
    />
  );
}

/** The flow level: header, canvas and the side panel with the flow's comments. */
function ReviewedFlow({
  flow,
  selection,
  ui,
  dark,
}: {
  flow: FlowEntry;
  selection: GallerySelection;
  ui: GalleryState;
  dark: boolean;
}) {
  const { comments, error } = useComments(flowKey(flow.id));
  const { view } = selection;
  return (
    <>
      <FlowToolbar
        dark={dark}
        flow={flow}
        group={selection.group}
        helpOpen={ui.helpOpen}
        node={selection.node}
        onHelp={ui.setHelpOpen}
        onSide={ui.setFlowSide}
        openComments={comments.filter(comment => comment.status === "open").length}
        panel={view.comments}
        side={ui.flowSide}
      />
      <FlowView
        comment={view.comment}
        comments={comments}
        commentsError={error}
        dark={dark}
        flow={flow}
        node={selection.node}
        onSide={ui.setFlowSide}
        panel={view.comments}
        registry={registry}
        side={ui.flowSide}
        theme={view.theme}
      />
    </>
  );
}

function MainArea({ selection, ui }: { selection: GallerySelection; ui: GalleryState }) {
  const dark = useTheme() === "dark";
  const { current, group, flow } = selection;
  const help = { helpOpen: ui.helpOpen, onHelp: ui.setHelpOpen };
  if (flow) return <ReviewedFlow dark={dark} flow={flow} selection={selection} ui={ui} />;
  if (selection.resolved.kind === "missing-flow") {
    return (
      <Empty
        className="m-auto"
        description={`No flow "${selection.resolved.flow}" in web/gallery/flows (bun run proto flow list).`}
        title="Flow not found"
      />
    );
  }
  if (current) {
    return <ReviewedPrototype current={current} dark={dark} selection={selection} ui={ui} />;
  }
  if (selection.resolved.kind === "spec" && group) {
    return <SpecOverview dark={dark} group={group} registry={registry} {...help} />;
  }
  return <IndexOverview dark={dark} registry={registry} {...help} />;
}

export function Gallery() {
  const searchRef = useRef<HTMLInputElement>(null);
  const theme = useTheme();
  const selection = useGallerySelection(registry);
  const ui = useGalleryState(registry);
  useGalleryShortcuts(selection, ui, searchRef);
  return (
    <QueryClientProvider client={galleryQueryClient}>
      <div className="flex h-dvh min-h-0 overflow-hidden bg-canvas text-fg">
        <GallerySidebar
          collapsed={ui.collapsed}
          current={selection.current}
          flow={selection.flow}
          group={selection.group}
          level={selection.level}
          onCollapse={ui.setCollapsed}
          onQuery={ui.setQuery}
          query={ui.query}
          registry={registry}
          searchRef={searchRef}
          state={selection.state}
        />
        <main className="flex min-h-0 min-w-0 flex-1 flex-col gap-3 p-3">
          <MainArea selection={selection} ui={ui} />
        </main>
        <Toaster position="bottom-right" theme={theme} />
      </div>
    </QueryClientProvider>
  );
}
