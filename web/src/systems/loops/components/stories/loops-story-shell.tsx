import type { ReactNode } from "react";

import { useTopbarSlot } from "@compozy/ui";

import { OsWindowSurface } from "@/systems/os";

interface LoopsStoryShellProps {
  /** The leaf crumb, matching what the route location publishes. */
  crumb: string;
  crumbs?: { id: string; label: string; onSelect: () => void }[];
  glyph?: ReactNode;
  toolbar?: ReactNode;
  actions?: ReactNode;
  onBack?: () => void;
  children: ReactNode;
}

/**
 * The product window surface around a loops route — the same `OsWindowSurface` the
 * window manager mounts, so the captured chrome (topbar, toolbar band, scrolling body)
 * is the shipped one rather than a story approximation. The desktop around it
 * (menubar, dock, window frame) is deliberately out of frame.
 *
 * Surfaces that publish their own topbar payload need no props here; the catalog and
 * run form get theirs from their route location, so this shell republishes it.
 */
export function LoopsStoryShell({
  crumb,
  crumbs,
  glyph,
  toolbar,
  actions,
  onBack,
  children,
}: LoopsStoryShellProps) {
  // A surface that publishes its own payload (the detail view) owns the slot; publishing
  // a second one here would clobber its actions.
  const republish = Boolean(crumbs || toolbar || onBack || actions);
  return (
    <div className="flex h-screen flex-col overflow-hidden">
      <OsWindowSurface controls="deck" glyph={glyph} title={crumb}>
        {republish ? (
          <LoopsStoryTopbarPayload
            actions={actions}
            crumb={crumb}
            crumbs={crumbs}
            onBack={onBack}
            toolbar={toolbar}
          />
        ) : null}
        {children}
      </OsWindowSurface>
    </div>
  );
}

function LoopsStoryTopbarPayload({
  crumb,
  crumbs,
  toolbar,
  actions,
  onBack,
}: Pick<LoopsStoryShellProps, "crumb" | "crumbs" | "toolbar" | "actions" | "onBack">) {
  useTopbarSlot({ actions, crumb, crumbs, onBack, toolbar });
  return null;
}
