import { PillGroup } from "@compozy/ui";
import { createContext, Fragment, use, type ReactNode } from "react";

import { cn } from "@/lib/utils";
import type { SessionPayload } from "@/systems/session";

import { descendantWindowIds } from "../lib/layout-minimums";
import type { OsWindow } from "../lib/os-types";
import type { CollapsedSplit, LayoutAxis, LayoutNode } from "../lib/window-manager-types";
import { useOsWindowTitle } from "./os-tab";

interface SplitViewContextValue {
  windows: Readonly<Record<string, OsWindow>>;
  collapsed: ReadonlyMap<string, CollapsedSplit>;
  sessionOf: (win: OsWindow) => SessionPayload | undefined;
  renderWindow: (windowId: string) => ReactNode;
  onShowMember: (windowId: string) => void;
}

const SplitViewContext = createContext<SplitViewContextValue | null>(null);

function useSplitView(): SplitViewContextValue {
  const value = use(SplitViewContext);
  if (value === null) throw new Error("OsSplitView parts render inside <OsSplitView>.");
  return value;
}

/** The hairline between two members. Read-only: resizing a split arrives with seam drags. */
function OsSplitSeam({ axis }: { axis: LayoutAxis }) {
  const row = axis === "horizontal";
  return (
    <div
      role="separator"
      aria-orientation={row ? "vertical" : "horizontal"}
      className={cn("relative z-10 shrink-0 bg-line", row ? "w-px" : "h-px")}
      data-slot="os-split-seam"
    />
  );
}

function MemberTitle({ win }: { win: OsWindow }) {
  const { sessionOf } = useSplitView();
  return <>{useOsWindowTitle(win, sessionOf(win))}</>;
}

/** A split that cannot fit shows one member at a time; the switcher picks which. */
function MemberSwitcher({ collapsed }: { collapsed: CollapsedSplit }) {
  const { windows, onShowMember } = useSplitView();
  const items = collapsed.windowIds.flatMap(windowId => {
    const win = windows[windowId];
    return win ? [{ value: windowId, label: <MemberTitle win={win} /> }] : [];
  });
  return (
    <div
      className="flex h-9 shrink-0 items-center justify-center border-b border-line bg-sunken px-2"
      data-slot="os-split-member-switcher"
    >
      <PillGroup
        aria-label="Windows in this split"
        items={items}
        onChange={onShowMember}
        size="sm"
        value={collapsed.shownWindowId}
      />
    </div>
  );
}

function Node({ node }: { node: LayoutNode }) {
  const view = useSplitView();
  if (node.kind === "leaf") return <>{view.renderWindow(node.windowId)}</>;
  const collapsed = view.collapsed.get(node.id);
  if (collapsed !== undefined) {
    // Every member stays mounted; each pane hides itself unless it is the one shown.
    return (
      <div className="flex min-h-0 min-w-0 flex-1 flex-col" data-slot="os-split-collapsed">
        <MemberSwitcher collapsed={collapsed} />
        {descendantWindowIds(node).map(windowId => (
          <Fragment key={windowId}>{view.renderWindow(windowId)}</Fragment>
        ))}
      </div>
    );
  }
  return (
    <div
      className={cn(
        "flex min-h-0 min-w-0 flex-1",
        node.axis === "horizontal" ? "flex-row" : "flex-col"
      )}
      data-slot="os-split"
      data-split-id={node.id}
    >
      {node.children.map((child, index) => (
        <Fragment key={child.id}>
          {index > 0 ? <OsSplitSeam axis={node.axis} /> : null}
          <div
            className="flex min-h-0 min-w-0"
            data-slot="os-split-member"
            style={{ flex: `${node.weights[index] ?? 1} 1 0` }}
          >
            <Node node={child} />
          </div>
        </Fragment>
      ))}
    </div>
  );
}

export interface OsSplitViewProps {
  root: LayoutNode;
  windows: Readonly<Record<string, OsWindow>>;
  /** The projection's collapsed splits: each shows one member at a time. */
  collapsed: readonly CollapsedSplit[];
  sessionOf: (win: OsWindow) => SessionPayload | undefined;
  renderWindow: (windowId: string) => ReactNode;
  /** Shows a collapsed split's member: focusing it makes it the one shown. */
  onShowMember: (windowId: string) => void;
  className?: string;
}

/**
 * A tab's content: its split tree laid out from the snapshot's weights. A split whose members
 * cannot fit — or every split in the compact viewport — shows one member at a time with a
 * switcher, leaving the weights untouched.
 */
export function OsSplitView({
  root,
  windows,
  collapsed,
  sessionOf,
  renderWindow,
  onShowMember,
  className,
}: OsSplitViewProps) {
  const value: SplitViewContextValue = {
    windows,
    collapsed: new Map(collapsed.map(entry => [entry.splitId, entry])),
    sessionOf,
    renderWindow,
    onShowMember,
  };
  return (
    <SplitViewContext value={value}>
      <div
        className={cn("relative flex min-h-0 min-w-0 flex-1 bg-canvas", className)}
        data-slot="os-split-view"
      >
        <Node node={root} />
      </div>
    </SplitViewContext>
  );
}
