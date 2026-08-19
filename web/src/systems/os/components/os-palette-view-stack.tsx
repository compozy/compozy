import { useState } from "react";

import { useOsPaletteSessionsView } from "../hooks/use-os-palette-sessions-view";
import { OS_APP_DESCRIPTORS } from "../lib/app-catalog";
import { paletteViewDefinition, type PaletteViewId } from "../lib/palette-view-registry";
import type { PaletteBreadcrumb } from "../lib/palette-view-stack";
import { OsPaletteViewShell } from "./os-palette-view-shell";

interface PaletteViewFrameProps {
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
function SessionsPaletteViewFrame({ onDismiss, ...shell }: PaletteViewFrameProps) {
  const content = useOsPaletteSessionsView({ query: shell.query, onDismiss });
  const definition = paletteViewDefinition("sessions");
  if (definition === null) return null;
  return <OsPaletteViewShell definition={definition} content={content} {...shell} />;
}

export interface OsPaletteViewStackProps {
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
  viewId,
  breadcrumb,
  onPop,
  onDismiss,
}: OsPaletteViewStackProps) {
  const [query, setQuery] = useState("");
  if (viewId !== "sessions") {
    return <OsPaletteViewUnavailable breadcrumb={breadcrumb} onPop={onPop} viewId={viewId} />;
  }
  return (
    <SessionsPaletteViewFrame
      breadcrumb={breadcrumb}
      query={query}
      onQueryChange={setQuery}
      onPop={onPop}
      onDismiss={onDismiss}
    />
  );
}

/** A view the catalog offers that this client cannot render — named, never blank. */
function OsPaletteViewUnavailable({
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
            This view is not available in this client.
          </p>
        ),
        note: null,
        backHint: "back",
        resetKey: viewId,
        onEmptyQueryBackspace: () => false,
      }}
      definition={{
        id: viewId,
        title: viewId,
        icon: OS_APP_DESCRIPTORS.session.icon,
        placeholder: "Search…",
        enterHint: "open",
        description: "This view is not available in this client",
      }}
      query=""
      onPop={onPop}
      onQueryChange={() => {}}
    />
  );
}
