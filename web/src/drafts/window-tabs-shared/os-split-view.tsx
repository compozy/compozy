/**
 * `os-split-view` (S3), `os-split-seam` (S6), `os-drop-zone` (S5) and the window head (S4): the
 * active tab's tree fills the content area. Windows are plain boxes laid out from their shares;
 * the only resize affordance is the seam between two of them.
 */
import { ArrowUpRight, X } from "@compozy/ui/icons";
import { Fragment, type ReactNode } from "react";

import {
  Button,
  PillGroup,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  Topbar,
  TopbarSlotProvider,
  cn,
} from "@compozy/ui";

import {
  SplitViewOptionsContext,
  useSplitViewOptions,
  type SeamState,
  type SplitViewOptions,
} from "./hooks/use-split-view-options";
import type { DropEdge, LayoutNode } from "./model";
import { leafIds } from "./model";
import { APP_ICON, APP_NAME } from "./apps";
import { WindowBody } from "./window-bodies";

const ZONE_BOX: Record<DropEdge, string> = {
  left: "inset-y-0 left-0 w-1/2 border-l-2",
  right: "inset-y-0 right-0 w-1/2 border-r-2",
  top: "inset-x-0 top-0 h-1/2 border-t-2",
  bottom: "inset-x-0 bottom-0 h-1/2 border-b-2",
};

/** The tinted half and its edge line: where the dragged window would land. */
export function OsDropZone({ edge, mode = "split" }: { edge: DropEdge; mode?: "split" | "swap" }) {
  if (mode === "swap") {
    return (
      <span
        className="pointer-events-none absolute inset-0 z-20 grid place-items-center border-2 border-brand bg-brand-tint-strong"
        data-slot="os-drop-zone"
      >
        <span className="rounded-pill bg-elevated px-3 py-1 text-meta font-medium text-fg shadow-pop">
          Swap
        </span>
      </span>
    );
  }
  return (
    <span
      className={cn(
        "pointer-events-none absolute z-20 border-brand bg-brand-tint-strong",
        ZONE_BOX[edge]
      )}
      data-edge={edge}
      data-slot="os-drop-zone"
    />
  );
}

function HeadControl({
  label,
  onClick,
  children,
}: {
  label: string;
  onClick?: () => void;
  children: ReactNode;
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            aria-label={label}
            onClick={onClick}
            onPointerDown={event => event.stopPropagation()}
            size="icon-sm"
            type="button"
            variant="quiet"
          />
        }
      >
        {children}
      </TooltipTrigger>
      <TooltipContent side="bottom">{label}</TooltipContent>
    </Tooltip>
  );
}

function WindowPane({ windowId }: { windowId: string }) {
  const options = useSplitViewOptions();
  const window = options.windows[windowId]!;
  const Icon = APP_ICON[window.app];
  const focused = options.focusedId === undefined || options.focusedId === windowId;
  const dragging = options.draggingId === windowId;
  return (
    <section
      className={cn(
        "relative flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-canvas",
        dragging && "opacity-45"
      )}
      data-focused={focused ? "" : undefined}
      data-slot="os-window"
      data-window-id={windowId}
      onPointerDownCapture={() => options.onFocus?.(windowId)}
    >
      <TopbarSlotProvider>
        <Topbar
          className={cn(
            "cursor-grab",
            !focused && "[&_[data-slot=topbar-identity]]:opacity-55",
            dragging && "cursor-grabbing"
          )}
          controls={
            <span className="flex items-center gap-0.5">
              {options.inSplit ? (
                <HeadControl label="Move to new tab" onClick={() => options.onUnsplit?.(windowId)}>
                  <ArrowUpRight aria-hidden="true" className="size-3.75" />
                </HeadControl>
              ) : null}
              <HeadControl label="Close" onClick={() => options.onClose?.(windowId)}>
                <X aria-hidden="true" className="size-3.75" />
              </HeadControl>
            </span>
          }
          data-slot="os-window-head"
          glyph={<Icon aria-hidden="true" />}
          onPointerDown={event => options.onHeadPointerDown?.(windowId, event)}
          title={APP_NAME[window.app]}
        />
      </TopbarSlotProvider>
      <WindowBody window={window} />
      {options.drop?.windowId === windowId ? (
        <OsDropZone edge={options.drop.edge} mode={options.drop.mode} />
      ) : null}
    </section>
  );
}

function OsSplitSeam({
  direction,
  state,
  onPointerDown,
  onDoubleClick,
}: {
  direction: "row" | "column";
  state: SeamState | undefined;
  onPointerDown?: (event: React.PointerEvent<HTMLElement>) => void;
  onDoubleClick?: () => void;
}) {
  const row = direction === "row";
  const lit = state === "hover" || state === "drag" || state === "focus";
  return (
    <div
      aria-orientation={row ? "vertical" : "horizontal"}
      className={cn(
        "group/seam relative z-10 shrink-0 bg-line",
        row ? "w-px cursor-col-resize" : "h-px cursor-row-resize"
      )}
      data-slot="os-split-seam"
      data-state={state}
      onDoubleClick={onDoubleClick}
      onPointerDown={onPointerDown}
      role="separator"
      tabIndex={0}
    >
      {/* Hit area wider than the visible line. */}
      <span className={cn("absolute", row ? "inset-y-0 -inset-x-1.5" : "inset-x-0 -inset-y-1.5")} />
      <span
        className={cn(
          "pointer-events-none absolute rounded-pill transition-colors duration-fast",
          row ? "inset-y-0 -left-px w-0.75" : "inset-x-0 -top-px h-0.75",
          "group-hover/seam:bg-brand",
          lit && "bg-brand",
          state === "limit" && "bg-attn",
          state === "focus" && "shadow-focus-ring"
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
      ) : null}
    </div>
  );
}

/** `os-split-member-switcher`: a split that cannot fit shows one member and this switcher. */
function MemberSwitcher({
  node,
  splitId,
  shownId,
}: {
  node: LayoutNode;
  splitId: string;
  shownId: string;
}) {
  const options = useSplitViewOptions();
  return (
    <div
      className="flex h-9 shrink-0 items-center justify-center border-b border-line bg-sunken px-2"
      data-slot="os-split-member-switcher"
    >
      <PillGroup
        aria-label="Windows in this split"
        items={leafIds(node).map(id => ({ value: id, label: options.windows[id]!.title }))}
        onChange={id => options.onShowMember?.(splitId, id)}
        size="sm"
        value={shownId}
      />
    </div>
  );
}

function Node({ node }: { node: LayoutNode }) {
  const options = useSplitViewOptions();
  if (node.kind === "leaf") return <WindowPane windowId={node.windowId} />;
  const splitId = node.id;
  const shownId = options.stacked?.[splitId];
  if (shownId) {
    return (
      <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        <MemberSwitcher node={node} shownId={shownId} splitId={splitId} />
        <WindowPane windowId={shownId} />
      </div>
    );
  }
  return (
    <div
      className={cn(
        "flex min-h-0 min-w-0 flex-1",
        node.direction === "row" ? "flex-row" : "flex-col"
      )}
      data-slot="os-split"
    >
      {node.children.map((child, index) => (
        <Fragment key={child.kind === "leaf" ? child.windowId : child.id}>
          {index > 0 ? (
            <OsSplitSeam
              direction={node.direction}
              onDoubleClick={() => options.onSeamBalance?.(splitId, index - 1)}
              onPointerDown={event => options.onSeamPointerDown?.(splitId, index - 1, event)}
              state={options.seams?.[`${splitId}:${index - 1}`]}
            />
          ) : null}
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

const ROOT_BAND: Record<DropEdge, string> = {
  left: "inset-y-0 left-0 w-1/3 border-l-2",
  right: "inset-y-0 right-0 w-1/3 border-r-2",
  top: "inset-x-0 top-0 h-1/3 border-t-2",
  bottom: "inset-x-0 bottom-0 h-1/3 border-b-2",
};

export interface OsSplitViewProps extends Omit<SplitViewOptions, "inSplit"> {
  root: LayoutNode;
  /** Root-edge drop: a full-height or full-width member at the tab's outer edge. */
  rootDrop?: DropEdge;
  className?: string;
  children?: ReactNode;
}

export function OsSplitView({ root, rootDrop, className, children, ...options }: OsSplitViewProps) {
  const value: SplitViewOptions = { ...options, inSplit: root.kind === "split" };
  return (
    <SplitViewOptionsContext value={value}>
      <div
        className={cn("relative flex min-h-0 min-w-0 flex-1 bg-canvas", className)}
        data-slot="os-split-view"
      >
        <Node node={root} />
        {rootDrop ? (
          <span
            className={cn(
              "pointer-events-none absolute z-30 border-brand bg-brand-tint-strong",
              ROOT_BAND[rootDrop]
            )}
            data-edge={rootDrop}
            data-slot="os-drop-zone-root"
          />
        ) : null}
        {children}
      </div>
    </SplitViewOptionsContext>
  );
}
