import { useState } from "react";

import { Button } from "@compozy/ui";

import { useCmdPaletteDeclarativeView } from "../hooks/use-cmd-palette-declarative-view";
import type { CmdPaletteDispatch } from "../hooks/use-cmd-palette-dispatch";
import { useOsPaletteDomainView } from "../hooks/use-os-palette-domain-view";
import { useOsPaletteSessionsView } from "../hooks/use-os-palette-sessions-view";
import { OS_APP_DESCRIPTORS } from "../lib/app-catalog";
import { paletteViewDefinition, type PaletteViewId } from "../lib/palette-view-registry";
import type { PaletteBreadcrumb } from "../lib/palette-view-stack";
import { OsPaletteViewShell } from "./os-palette-view-shell";

interface PaletteViewFrameProps {
  dispatch: CmdPaletteDispatch;
  breadcrumb: PaletteBreadcrumb;
  query: string;
  onQueryChange: (query: string) => void;
  onPop: () => void;
  onDismiss: () => void;
}

/**
 * One registration = one frame: it names the view's controller and hands its
 * content to the shared shell. Everything visible and every keystroke belong to
 * the shell, so a second view is this file plus a controller — not a second
 * palette.
 */
function SessionsPaletteViewFrame({
  dispatch: _dispatch,
  onDismiss,
  ...shell
}: PaletteViewFrameProps) {
  const content = useOsPaletteSessionsView({ query: shell.query, onDismiss });
  const definition = paletteViewDefinition("sessions");
  if (definition === null) return null;
  return <OsPaletteViewShell definition={definition} content={content} {...shell} />;
}

function DomainPaletteViewFrame({
  dispatch: _dispatch,
  onDismiss,
  viewId,
  ...shell
}: PaletteViewFrameProps & { viewId: string }) {
  const definition = paletteViewDefinition(viewId);
  const content = useOsPaletteDomainView(definition ?? unavailableDefinition(viewId), {
    query: shell.query,
    onDismiss,
  });
  return (
    <OsPaletteViewShell
      definition={definition ?? unavailableDefinition(viewId)}
      content={content}
      {...shell}
    />
  );
}

function DeclarativePaletteViewFrame({
  dispatch,
  onDismiss,
  viewId,
  ...shell
}: PaletteViewFrameProps & { viewId: string }) {
  const model = useCmdPaletteDeclarativeView({
    dispatch,
    onDismiss,
    query: shell.query,
    viewId,
  });
  if (model.timedOut || model.error) {
    const source = extensionName(viewId);
    return (
      <OsPaletteViewShell
        definition={model.definition}
        content={{
          kind: "list",
          rows: [],
          header: null,
          empty: (
            <div className="space-y-3 px-3 py-6 text-center text-small-body text-muted">
              <p>{source ? `${source} view unavailable.` : "View unavailable."}</p>
              <Button size="sm" variant="outline" onClick={model.retry}>
                Retry
              </Button>
            </div>
          ),
          note: null,
          backHint: "back",
          resetKey: `${viewId}:unavailable`,
          onEmptyQueryBackspace: () => false,
        }}
        {...shell}
      />
    );
  }
  return <OsPaletteViewShell definition={model.definition} content={model.content} {...shell} />;
}

export interface OsPaletteViewStackProps {
  dispatch: CmdPaletteDispatch;
  viewId: PaletteViewId;
  breadcrumb: PaletteBreadcrumb;
  onPop: () => void;
  onDismiss: () => void;
}

/**
 * The palette while it is showing a pushed view.
 *
 * The query lives here rather than in the shell so that entering or leaving a
 * level starts the new level's search clean — the level owns the question being
 * asked, and a stale query would ask it of the wrong list.
 */
export function OsPaletteViewStack({
  dispatch,
  viewId,
  breadcrumb,
  onPop,
  onDismiss,
}: OsPaletteViewStackProps) {
  const [query, setQuery] = useState("");
  const definition = paletteViewDefinition(viewId);
  const shell = {
    breadcrumb,
    dispatch,
    query,
    onQueryChange: setQuery,
    onPop,
    onDismiss,
  };
  if (viewId === "sessions") {
    return <SessionsPaletteViewFrame {...shell} />;
  }
  if (definition?.domainTitle) {
    return <DomainPaletteViewFrame {...shell} viewId={viewId} />;
  }
  return <DeclarativePaletteViewFrame {...shell} viewId={viewId} />;
}

/** A view the catalog offers that this client cannot render — named, never blank. */
export function OsPaletteViewUnavailable({
  breadcrumb,
  onPop,
  viewId,
}: {
  breadcrumb: PaletteBreadcrumb;
  onPop: () => void;
  viewId: string;
}) {
  return (
    <OsPaletteViewShell
      breadcrumb={breadcrumb}
      content={{
        rows: [],
        header: null,
        empty: (
          <p className="px-3 py-6 text-center text-small-body text-muted">
            {extensionName(viewId) ?? viewId} view is not available in this client.
          </p>
        ),
        note: null,
        backHint: "back",
        resetKey: viewId,
        onEmptyQueryBackspace: () => false,
      }}
      definition={{
        ...unavailableDefinition(viewId),
      }}
      query=""
      onPop={onPop}
      onQueryChange={() => {}}
    />
  );
}

function unavailableDefinition(viewId: string) {
  return {
    id: viewId,
    title: viewId,
    icon: OS_APP_DESCRIPTORS.session.icon,
    placeholder: "Search…",
    enterHint: "open",
    description: "This view is not available in this client",
  };
}

function extensionName(viewId: string): string | null {
  return /^ext\.([^.]+)\./.exec(viewId)?.[1] ?? null;
}
