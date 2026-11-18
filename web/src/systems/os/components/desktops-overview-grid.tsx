import type * as React from "react";

import { Skeleton, SkeletonRegion, SkeletonText } from "@compozy/ui";

import { useDesktopCardDrag } from "../hooks/use-desktop-card-drag";
import { DesktopOverviewCard, DesktopOverviewCardGap } from "./desktop-overview-card";
import type { DesktopOverviewAction, DesktopsOverviewState } from "../lib/desktops-overview-model";
import type { DesktopsOverviewProps } from "./desktops-overview";

type DesktopsOverviewGridProps = Pick<
  DesktopsOverviewProps,
  | "onOpenChange"
  | "onSwitchDesktop"
  | "onRenameDesktop"
  | "onReorderDesktop"
  | "onDeleteDesktop"
  | "onMoveTab"
  | "onChooseTab"
  | "onTabPointerDown"
  | "dropTargetDesktopId"
  | "dragKind"
  | "onDesktopActionApplied"
> & {
  state: Extract<DesktopsOverviewState, { status: "ready" }>;
  busy: boolean;
  sessionOf: NonNullable<DesktopsOverviewProps["sessionOf"]>;
  initialFocusDesktopId: string | null;
  initialFocusRef: React.RefObject<HTMLButtonElement | null>;
  desktopAction: DesktopOverviewAction | null;
};

const CARD_SELECTOR = '[data-slot="desktops-overview-card"]:not(:disabled)';
const DEFAULT_ASPECT = 16 / 10;

/** Arrows move across the cards' thumbnails (Up/Down by one row), Home/End jump; Enter is native. */
function moveCardFocus(event: React.KeyboardEvent<HTMLElement>): void {
  const grid = event.currentTarget;
  const cards = Array.from(grid.querySelectorAll<HTMLElement>(CARD_SELECTOR));
  const index = cards.findIndex(card => card === event.target);
  if (index < 0) return;
  const columns = Math.max(
    1,
    getComputedStyle(grid).gridTemplateColumns.split(" ").filter(Boolean).length
  );
  const targets: Record<string, number> = {
    ArrowRight: index + 1,
    ArrowLeft: index - 1,
    ArrowDown: index + columns,
    ArrowUp: index - columns,
    Home: 0,
    End: cards.length - 1,
  };
  const target = targets[event.key];
  if (target === undefined) return;
  event.preventDefault();
  cards[Math.min(cards.length - 1, Math.max(0, target))]?.focus();
}

const GRID_CLASS = "grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] items-start gap-5 pb-8";
const CARD_FRAME_CLASS = "flex min-w-0 flex-col gap-3 rounded-xl bg-card p-3 shadow-card";

/**
 * The ready overview: one card per desktop in desktop order. Cards drag to reorder and take a
 * dragged tab; the thumbnail switches, a tab row jumps to its tab, and each closes the overlay.
 */
export function DesktopsOverviewGrid({
  state,
  busy,
  sessionOf,
  initialFocusDesktopId,
  initialFocusRef,
  desktopAction,
  dropTargetDesktopId,
  dragKind,
  onDesktopActionApplied,
  onOpenChange,
  onSwitchDesktop,
  onRenameDesktop,
  onReorderDesktop,
  onDeleteDesktop,
  onMoveTab,
  onChooseTab,
  onTabPointerDown,
}: DesktopsOverviewGridProps) {
  const byId = new Map(state.desktops.map(desktop => [desktop.id, desktop]));
  const cards = useDesktopCardDrag(
    state.desktops.map(desktop => desktop.id),
    onReorderDesktop
  );
  const lifted = cards.drag ? byId.get(cards.drag.desktopId) : undefined;

  const card = (desktopId: string, index: number, floating: boolean) => {
    const desktop = byId.get(desktopId)!;
    const active = desktop.id === state.activeDesktopId;
    return (
      <DesktopOverviewCard
        key={desktop.id}
        active={active}
        busy={busy}
        desktop={desktop}
        desktops={state.desktops}
        dropTarget={!floating && dropTargetDesktopId === desktop.id}
        dragKind={dragKind}
        index={index}
        initialAction={
          !floating && desktopAction?.desktopId === desktop.id ? desktopAction.action : undefined
        }
        lifted={floating}
        onInitialActionApplied={onDesktopActionApplied}
        sessionOf={sessionOf}
        thumbnailRef={
          !floating && desktop.id === initialFocusDesktopId ? initialFocusRef : undefined
        }
        onCardPointerDown={busy || floating ? undefined : event => cards.start(desktop.id, event)}
        onChooseTab={windowId => {
          onChooseTab(desktop.id, windowId);
          onOpenChange(false);
        }}
        onDelete={destinationId => onDeleteDesktop(desktop.id, destinationId)}
        onMoveTab={onMoveTab}
        onRename={name => onRenameDesktop(desktop.id, name)}
        onReorder={order => onReorderDesktop(desktop.id, order)}
        onSwitch={() => {
          if (!active) onSwitchDesktop(desktop.id);
          onOpenChange(false);
        }}
        onTabPointerDown={busy ? undefined : onTabPointerDown}
      />
    );
  };

  return (
    <div className="relative" data-slot="desktops-overview-grid">
      <ol aria-label="Desktops" className={GRID_CLASS} onKeyDown={moveCardFocus}>
        {cards.previewOrder.map(desktopId =>
          desktopId === cards.drag?.desktopId ? (
            <DesktopOverviewCardGap desktopId={desktopId} key={desktopId} />
          ) : (
            card(
              desktopId,
              state.desktops.findIndex(desktop => desktop.id === desktopId),
              false
            )
          )
        )}
      </ol>
      {cards.drag && lifted ? (
        <ol
          aria-hidden="true"
          className="pointer-events-none absolute z-10"
          inert
          style={{ left: cards.drag.left, top: cards.drag.top, width: cards.drag.width }}
        >
          {card(lifted.id, state.desktops.indexOf(lifted), true)}
        </ol>
      ) : null}
    </div>
  );
}

/** Cards a loading overview stands in with: a few desks, as many as a fresh install has. */
const LOADING_DESKTOP_CARDS = ["first", "second", "third"] as const;

/**
 * The overview's grid while the layout snapshot is read: cards in the real card frame, a thumbnail
 * in the work area's default ratio over the name line. No desktop data is invented, and nothing
 * here can be switched to.
 */
export function DesktopsOverviewSkeleton() {
  return (
    <SkeletonRegion className={GRID_CLASS} label="Loading desktops">
      {LOADING_DESKTOP_CARDS.map(card => (
        <div aria-hidden="true" className={CARD_FRAME_CLASS} key={card}>
          <Skeleton className="w-full rounded-lg" style={{ aspectRatio: DEFAULT_ASPECT }} />
          <span className="flex min-h-5 min-w-0 items-center gap-2 px-1 pr-9">
            <SkeletonText className="w-32 font-medium" />
          </span>
        </div>
      ))}
    </SkeletonRegion>
  );
}
