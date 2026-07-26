import { Eyebrow, PropertyRow } from "@agh/ui";

import type { LayoutSelection } from "../../hooks/use-layout-canvas-selection";
import { overlappingGroupIds } from "../../lib/window-manager-layout-canvas";
import { fractionLabel } from "../../lib/window-manager-layout-detents";
import { layoutOrientationOf } from "../../lib/window-manager-layout-mutations";
import { findLayoutNode, layoutNodeWindowIds } from "../../lib/window-manager-layout-tree";
import { layoutWindowFace } from "../../lib/window-manager-layout-window-face";
import type {
  WindowManagerLayoutDesktop,
  WindowManagerLayoutDocument,
} from "../../lib/window-manager-layout-types";
import { LayoutInspectorNode } from "./layout-inspector-node";
import { LayoutInspectorWindow } from "./layout-inspector-window";

interface LayoutInspectorProps {
  document: WindowManagerLayoutDocument;
  desktop: WindowManagerLayoutDesktop;
  selection: LayoutSelection;
  onDesktopChange: (next: WindowManagerLayoutDesktop) => void;
}

/** What the selection on the canvas is, and the few things it can be told to do. */
export function LayoutInspector({
  document,
  desktop,
  selection,
  onDesktopChange,
}: LayoutInspectorProps) {
  if (selection === null) {
    return (
      <InspectorShell subtitle={`${desktop.id} · order ${desktop.order}`} title={desktop.name}>
        <section className="flex flex-col gap-2">
          <Eyebrow className="text-faint">Nothing selected</Eyebrow>
          <p className="text-form-label leading-normal text-subtle">
            Click a window to change what it holds or how it splits. Drag a divider to rebalance, or
            a group edge to repartition the desktop.
          </p>
        </section>
        <section className="flex flex-col gap-2">
          <Eyebrow className="text-faint">Desktop</Eyebrow>
          <div>
            <PropertyRow label="Groups" mono>
              {desktop.groups.length}
            </PropertyRow>
            <PropertyRow label="Tiled windows" mono>
              {desktop.groups.reduce(
                (total, group) => total + layoutNodeWindowIds(group.root).length,
                0
              )}
            </PropertyRow>
            <PropertyRow label="Floating" mono>
              {desktop.floating.length}
            </PropertyRow>
            <PropertyRow label="Purpose">
              {desktop.purpose === "focus" ? "Focus" : "Standard"}
            </PropertyRow>
          </div>
        </section>
      </InspectorShell>
    );
  }

  if (selection.kind === "group") {
    const group = desktop.groups.find(candidate => candidate.id === selection.id);
    if (!group) return null;
    const overlaps = overlappingGroupIds(desktop).has(group.id);
    return (
      <InspectorShell subtitle={group.id} title="Group">
        <section className="flex flex-col gap-2">
          <Eyebrow className="text-faint">Share of the desktop</Eyebrow>
          <div>
            <PropertyRow label="Left" mono>
              {fractionLabel(group.frame.x)}
            </PropertyRow>
            <PropertyRow label="Top" mono>
              {fractionLabel(group.frame.y)}
            </PropertyRow>
            <PropertyRow label="Width" mono>
              {fractionLabel(group.frame.w)}
            </PropertyRow>
            <PropertyRow label="Height" mono>
              {fractionLabel(group.frame.h)}
            </PropertyRow>
          </div>
        </section>
        {overlaps ? (
          <p className="text-form-label text-danger" role="status">
            This group overlaps another one. The daemon refuses a desktop whose groups cover each
            other.
          </p>
        ) : null}
      </InspectorShell>
    );
  }

  if (selection.kind === "window") {
    const window = document.windows[selection.id];
    if (!window) return null;
    const face = layoutWindowFace(window, selection.id);
    return (
      <InspectorShell subtitle={`${window.id} · ${window.route.pathname}`} title={face.title}>
        <section className="flex flex-col gap-2">
          <Eyebrow className="text-faint">Floating window</Eyebrow>
          <div>
            <PropertyRow label="Placement">{window.placement}</PropertyRow>
            <PropertyRow label="Minimized">{window.minimized ? "Yes" : "No"}</PropertyRow>
            <PropertyRow label="Size" mono>
              {`${fractionLabel(window.floatingRect.w)} × ${fractionLabel(window.floatingRect.h)}`}
            </PropertyRow>
          </div>
        </section>
        <p className="text-form-label leading-normal text-subtle">
          Drag it on the canvas to reposition. Floating windows sit above every group on this
          desktop.
        </p>
      </InspectorShell>
    );
  }

  const found = findLayoutNode(desktop, selection.id);
  if (!found) return null;
  const { node } = found;
  const title =
    node.kind === "split"
      ? layoutOrientationOf(node.axis) === "rows"
        ? "Rows"
        : "Columns"
      : node.kind === "stack"
        ? "Stack"
        : "Window";

  return (
    <InspectorShell subtitle={node.id} title={title}>
      <LayoutInspectorNode desktop={desktop} node={node} onDesktopChange={onDesktopChange} />
      <LayoutInspectorWindow
        desktop={desktop}
        document={document}
        node={node}
        onDesktopChange={onDesktopChange}
      />
    </InspectorShell>
  );
}

function InspectorShell({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle: string;
  children: React.ReactNode;
}) {
  return (
    <aside
      aria-label="Selection inspector"
      className="flex min-w-0 flex-col border-line-soft bg-canvas-tint"
      data-testid="layout-inspector"
    >
      <header className="border-b border-line-soft px-3.5 py-3">
        <h3 className="truncate text-small-body font-semibold text-fg-strong">{title}</h3>
        <p className="mt-0.5 font-mono text-mono-id break-all text-faint">{subtitle}</p>
      </header>
      <div className="flex min-h-0 flex-1 flex-col gap-3.5 overflow-y-auto px-3.5 py-3">
        {children}
      </div>
    </aside>
  );
}
