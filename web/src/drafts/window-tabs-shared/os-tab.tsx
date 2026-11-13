/**
 * `os-tab` (S2): one plate per tab, after the Monocode title-bar tab. An icon tile per window (a
 * split stacks up to three, the lead in front), the lead window's title, the member count of a
 * split, and a close button on hover. Width rules come from container queries so a shrinking bar
 * needs no measurement: under 72px the close button goes, at the 40px floor only the tile stays.
 */
import { X } from "@compozy/ui/icons";
import type { CSSProperties, ReactNode } from "react";

import { KindIcon, Pill, StateGlyph, cn, providerKindIconRegistry } from "@compozy/ui";

import { APP_ICON } from "./apps";
import { leafIds, type DraftTab, type DraftWindow } from "./model";

const STACK_MAX = 3;
const TILE_W = 24;
const TILE_H = 20;
const PEEK = 4;
const SHRINK = 2;

function TileMark({ window }: { window: DraftWindow }) {
  if (window.provider) {
    return (
      <KindIcon
        aria-hidden="true"
        kind={window.provider}
        registry={providerKindIconRegistry}
        size="xs"
      />
    );
  }
  const Icon = APP_ICON[window.app];
  return <Icon aria-hidden="true" className="size-3 text-fg-2" />;
}

/** The raised tile that stands for one window; a split stacks one per member behind the lead. */
export function OsTabTiles({ windows }: { windows: readonly DraftWindow[] }) {
  const shown = windows.slice(0, STACK_MAX);
  const width = TILE_W + PEEK * (shown.length - 1);
  return (
    <span
      aria-hidden="true"
      className="relative block shrink-0"
      data-slot="os-tab-tiles"
      style={{ width, height: TILE_H }}
    >
      {shown.map((window, depth) => {
        const style: CSSProperties = {
          width: TILE_W - SHRINK * depth,
          height: TILE_H - SHRINK * depth,
          left: PEEK * depth + SHRINK * depth,
          top: (SHRINK * depth) / 2,
          zIndex: STACK_MAX - depth,
        };
        return (
          <span
            className={cn(
              "absolute grid place-items-center rounded-[5px]",
              // Opaque mixes: a translucent front tile would show the one tucked behind it.
              depth === 0
                ? "bg-[color-mix(in_srgb,var(--color-fg)_13%,var(--color-rail))]"
                : "bg-[color-mix(in_srgb,var(--color-fg)_20%,var(--color-rail))]"
            )}
            key={window.id}
            style={style}
          >
            {depth === 0 ? <TileMark window={window} /> : null}
          </span>
        );
      })}
    </span>
  );
}

/**
 * What a shrinking tab gives up, by its inner width: the member count, then the waiting count,
 * then the inline signal (it moves to the tile's corner), then the close button, then the title.
 * The active tab keeps its close button longer and ends as the tile alone, like the others.
 */
const NARROW = {
  rest: {
    title: "@max-[64px]:hidden",
    close: "@max-[112px]:hidden",
    center: "@max-[64px]:justify-center",
  },
  active: {
    title: "@max-[80px]:hidden",
    close: "@max-[52px]:hidden",
    center: "@max-[52px]:justify-center",
  },
} as const;

export interface OsTabProps {
  tab: DraftTab;
  windows: Record<string, DraftWindow>;
  active?: boolean;
  /** Forced presentation for static states. */
  hovered?: boolean;
  focused?: boolean;
  dragging?: boolean;
  /** The tab a dragged window head is dwelling on (US-008.AC-3). */
  dwelling?: boolean;
  /** Fixed width in px for specimens; the bar lets tabs flex. */
  width?: number;
  onActivate?: () => void;
  onClose?: () => void;
  onPointerDown?: (event: React.PointerEvent<HTMLElement>) => void;
}

function tabSignal(members: readonly DraftWindow[]) {
  return (
    members.find(member => member.signal === "attention")?.signal ??
    members.find(member => member.signal === "failed")?.signal ??
    members.find(member => member.signal === "running")?.signal
  );
}

export function OsTab({
  tab,
  windows,
  active = false,
  hovered = false,
  focused = false,
  dragging = false,
  dwelling = false,
  width,
  onActivate,
  onClose,
  onPointerDown,
}: OsTabProps) {
  const members = leafIds(tab.root).map(id => windows[id]!);
  const lead = members[0]!;
  const pinned = tab.pinned === true;
  const signal = tabSignal(members);
  const pending = members.reduce((sum, member) => sum + (member.pending ?? 0), 0);
  const label = members.length > 1 ? `${lead.title} and ${members.length - 1} more` : lead.title;

  const narrow = active ? NARROW.active : NARROW.rest;
  // The plate is a size container, so it has no width of its own: a pinned tab takes exactly its
  // tiles plus the 3.5px inset on each side.
  const pinnedWidth = TILE_W + PEEK * (Math.min(members.length, STACK_MAX) - 1) + 7;
  let plateStyle: CSSProperties | undefined;
  if (width) plateStyle = { width, flex: "none" };
  else if (pinned) plateStyle = { width: pinnedWidth };

  let signalMark: ReactNode = null;
  if (signal) signalMark = <StateGlyph size="sm" state={signal} />;

  return (
    <div
      className={cn(
        "group/tab @container relative flex h-6.75 items-center rounded-sm pl-[3.5px] text-body font-medium transition-colors duration-fast select-none",
        // A pinned tab hugs its tile: the same 3.5px on every side.
        pinned ? "shrink-0 pr-[3.5px]" : "max-w-56 min-w-10 pr-1",
        // Every tab gives up width together; the active one gives up a third as fast, so it
        // stays the widest until all of them reach the 40px floor.
        !pinned && (active ? "flex-[1_0.35_14rem]" : "flex-[1_1_14rem]"),
        active ? "bg-fg/7 text-fg" : "bg-fg/4 text-fg-2 hover:bg-fg/6 hover:text-fg",
        hovered && !active && "bg-fg/6 text-fg",
        dwelling && "bg-brand-tint-strong text-fg ring-1 ring-brand",
        focused && "shadow-focus-ring",
        dragging && "opacity-40"
      )}
      data-active={active ? "" : undefined}
      data-pinned={pinned ? "" : undefined}
      data-slot="os-tab"
      data-tab-id={tab.id}
      style={plateStyle}
    >
      <button
        aria-label={label}
        aria-selected={active}
        className={cn(
          "flex h-full min-w-0 flex-1 cursor-pointer items-center gap-2 rounded-[inherit] text-left outline-none",
          narrow.center
        )}
        onClick={onActivate}
        onPointerDown={onPointerDown}
        role="tab"
        type="button"
      >
        <span className="relative shrink-0">
          <OsTabTiles windows={members} />
          {signalMark ? (
            <span
              className={cn(
                "absolute -top-1 -right-1 z-10 place-items-center rounded-pill bg-rail p-px",
                pinned ? "grid" : "hidden @max-[112px]:grid"
              )}
            >
              {signalMark}
            </span>
          ) : null}
        </span>
        {pinned ? null : (
          <>
            <span className={cn("min-w-0 flex-1 truncate", narrow.title)}>{lead.title}</span>
            {members.length > 1 ? (
              <span className="shrink-0 text-meta text-subtle tabular-nums @max-[152px]:hidden">
                {members.length}
              </span>
            ) : null}
            {signalMark ? (
              <span className="grid shrink-0 place-items-center @max-[112px]:hidden">
                {signalMark}
              </span>
            ) : null}
            <Pill.Count className="@max-[136px]:hidden" count={pending} />
          </>
        )}
      </button>
      {pinned ? null : (
        <button
          aria-label={`Close ${lead.title}`}
          className={cn(
            "ml-1 grid size-4.5 shrink-0 place-items-center rounded-xs text-subtle opacity-0 transition-opacity duration-fast",
            narrow.close,
            "hover:bg-fg/10 hover:text-fg focus-visible:opacity-100 focus-visible:shadow-focus-ring focus-visible:outline-none",
            "group-hover/tab:opacity-100",
            (active || hovered) && "opacity-100"
          )}
          data-slot="os-tab-close"
          onClick={onClose}
          onPointerDown={event => event.stopPropagation()}
          type="button"
        >
          <X aria-hidden="true" className="size-3" />
        </button>
      )}
    </div>
  );
}
