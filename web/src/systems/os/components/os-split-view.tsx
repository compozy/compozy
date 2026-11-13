import { PillGroup } from "@compozy/ui";
import {
  Fragment,
  type ComponentProps,
  type CSSProperties,
  type KeyboardEvent,
  type PointerEvent,
  type ReactNode,
} from "react";

import { cn } from "@/lib/utils";
import type { SessionPayload } from "@/systems/session";

import {
  SplitViewContext,
  useSplitView,
  type OsSplitSeamSlot,
  type SplitViewContextValue,
} from "../hooks/use-split-view";
import { useOsWindowTitle } from "../hooks/use-os-window-title";
import { descendantWindowIds } from "../lib/layout-minimums";
import { MEMBER_SWITCHER_PX } from "../lib/tab-projection";
import type { OsWindow } from "../lib/os-types";
import type { ProjectionGaps } from "../lib/tab-projection-types";
import type {
  CollapsedSplit,
  LayoutAxis,
  LayoutNode,
  ProjectedSeam,
} from "../lib/window-manager-types";

export interface OsSplitSeamProps extends Omit<
  ComponentProps<"div">,
  "onPointerDown" | "onKeyDown" | "onDoubleClick"
> {
  axis: LayoutAxis;
  /** The inner gap the seam spans, in pixels; the projection allots exactly this between members. */
  gap?: number;
  /** Leading member's share, in percent, for assistive tech. */
  valueNow?: number;
  state?: "drag" | "limit";
  onPointerDown?: (event: PointerEvent<HTMLElement>) => void;
  onKeyDown?: (event: KeyboardEvent<HTMLElement>) => void;
  onDoubleClick?: () => void;
  /** An overlay drawn on the seam while it is held, in place of the minimum-size label. */
  children?: ReactNode;
}

/** The seam's hit box across its axis: what a pointer can grab, whatever the inner gap. */
const SEAM_HIT_PX = 12;

/**
 * The line between two members and the only resize affordance: a hit area wider than the
 * visible line, double-click to balance, arrow keys when focused.
 */
export function OsSplitSeam({
  axis,
  gap = 0,
  valueNow,
  state,
  onPointerDown,
  onKeyDown,
  onDoubleClick,
  className,
  style,
  children,
  ref,
  ...props
}: OsSplitSeamProps) {
  const row = axis === "horizontal";
  const interactive = onPointerDown !== undefined;
  // The separator owns its hit box even at a zero gap: it is wider than the gap and negative
  // margins give the difference back, so it takes exactly `gap` of layout space.
  const size = Math.max(gap, SEAM_HIT_PX);
  const overhang = (gap - size) / 2;
  const box: CSSProperties = row
    ? { width: size, marginLeft: overhang, marginRight: overhang }
    : { height: size, marginTop: overhang, marginBottom: overhang };
  return (
    <div
      {...props}
      ref={ref}
      role="separator"
      aria-orientation={row ? "vertical" : "horizontal"}
      aria-label="Resize windows"
      aria-valuemin={interactive ? 0 : undefined}
      aria-valuemax={interactive ? 100 : undefined}
      aria-valuenow={interactive ? valueNow : undefined}
      tabIndex={interactive ? 0 : undefined}
      className={cn(
        "group/seam relative z-10 shrink-0 outline-none",
        interactive && (row ? "cursor-col-resize" : "cursor-row-resize"),
        className
      )}
      style={{ ...style, ...box }}
      data-slot="os-split-seam"
      data-state={state}
      onDoubleClick={onDoubleClick}
      onKeyDown={onKeyDown}
      onPointerDown={onPointerDown}
    >
      {/* The hairline sits centered in the gap. */}
      <span
        className={cn(
          "pointer-events-none absolute bg-line",
          row
            ? "inset-y-0 left-1/2 w-px -translate-x-1/2"
            : "inset-x-0 top-1/2 h-px -translate-y-1/2"
        )}
      />
      <span
        className={cn(
          "pointer-events-none absolute rounded-pill transition-colors duration-fast",
          row
            ? "inset-y-0 left-1/2 w-0.75 -translate-x-1/2"
            : "inset-x-0 top-1/2 h-0.75 -translate-y-1/2",
          interactive && "group-hover/seam:bg-brand group-focus-visible/seam:bg-brand",
          "group-focus-visible/seam:shadow-focus-ring",
          state === "drag" && "bg-brand",
          state === "limit" && "bg-attn"
        )}
      />
      {state === "limit" ? (
        <span
          className={cn(
            "absolute z-10 rounded-pill bg-elevated px-2 py-0.5 text-badge font-medium whitespace-nowrap text-fg shadow-pop",
            row ? "top-1/2 left-2 -translate-y-1/2" : "top-2 left-1/2 -translate-x-1/2"
          )}
        >
          Minimum size
        </span>
      ) : (
        children
      )}
    </div>
  );
}

const renderInertSeam = (slot: OsSplitSeamSlot) => <OsSplitSeam axis={slot.axis} gap={slot.gap} />;

/** The projection's rule for a configured gap: a finite, non-negative whole pixel count. */
function gapPixels(value: number | undefined): number {
  return value !== undefined && Number.isFinite(value) ? Math.max(0, Math.round(value)) : 0;
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
      className="flex shrink-0 items-center justify-center border-b border-line bg-sunken px-2"
      data-slot="os-split-member-switcher"
      style={{ height: MEMBER_SWITCHER_PX }}
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
          {index > 0
            ? view.renderSeam({
                splitId: node.id,
                boundaryIndex: index - 1,
                axis: node.axis,
                memberCount: node.children.length,
                gap: view.innerGap,
                seam: view.seams.get(`${node.id}:${index - 1}`),
              })
            : null}
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
  /** The projection's seams, keyed `${splitId}:${boundaryIndex}`; absent renders inert seams. */
  seams?: readonly ProjectedSeam[];
  /** Renders each seam; the default is the inert line. */
  renderSeam?: (slot: OsSplitSeamSlot) => ReactNode;
  /** Shows a collapsed split's member: focusing it makes it the one shown. */
  onShowMember: (windowId: string) => void;
  /**
   * The configured gaps, laid out exactly as the projection allots them: the outer gaps pad the
   * content, the inner gap separates members. Drop zones and seams read the same projection.
   */
  gaps?: ProjectionGaps;
  className?: string;
  /** Overlays above the content: drop zones while a tab or window head is dragged. */
  children?: ReactNode;
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
  seams = [],
  renderSeam = renderInertSeam,
  onShowMember,
  gaps,
  className,
  children,
}: OsSplitViewProps) {
  const value: SplitViewContextValue = {
    windows,
    collapsed: new Map(collapsed.map(entry => [entry.splitId, entry])),
    seams: new Map(seams.map(seam => [seam.id, seam])),
    sessionOf,
    renderWindow,
    renderSeam,
    onShowMember,
    innerGap: gapPixels(gaps?.inner),
  };
  return (
    <SplitViewContext value={value}>
      <div
        className={cn("relative flex min-h-0 min-w-0 flex-1 bg-canvas", className)}
        data-slot="os-split-view"
        style={{
          paddingTop: gapPixels(gaps?.top),
          paddingRight: gapPixels(gaps?.right),
          paddingBottom: gapPixels(gaps?.bottom),
          paddingLeft: gapPixels(gaps?.left),
        }}
      >
        <Node node={root} />
        {children}
      </div>
    </SplitViewContext>
  );
}
