import { useState, type ComponentType } from "react";

import { useOsPaletteSessionsView } from "../hooks/use-os-palette-sessions-view";
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
  return (
    <OsPaletteViewShell
      definition={paletteViewDefinition("sessions")}
      content={content}
      {...shell}
    />
  );
}

/**
 * The built-in view registry's render side (ADR-003). Extension-contributed
 * views are a deliberate v1 non-goal; this map is where they would land.
 */
const PALETTE_VIEW_FRAMES: Record<PaletteViewId, ComponentType<PaletteViewFrameProps>> = {
  sessions: SessionsPaletteViewFrame,
};

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
  const Frame = PALETTE_VIEW_FRAMES[viewId];
  return (
    <Frame
      breadcrumb={breadcrumb}
      query={query}
      onQueryChange={setQuery}
      onPop={onPop}
      onDismiss={onDismiss}
    />
  );
}
