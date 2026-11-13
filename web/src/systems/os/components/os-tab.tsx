import {
  KindIcon,
  Pill,
  StateGlyph,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  type TopbarSlotValue,
  providerKindIconRegistry,
} from "@compozy/ui";
import { X } from "@compozy/ui/icons";
import {
  Fragment,
  type CSSProperties,
  type MouseEvent,
  type PointerEvent,
  type ReactNode,
} from "react";

import { cn } from "@/lib/utils";
import { pendingInteractions, type SessionPayload } from "@/systems/session";

import {
  pinnedTabWidth,
  TAB_TILE_PEEK,
  TAB_TILE_STACK_MAX,
  TAB_TILE_WIDTH,
} from "../hooks/use-os-tab-bar";
import { useOsWindowTitle } from "../hooks/use-os-window-title";
import { useWindowMemberSlot } from "../hooks/use-window-member-slot";
import { getOsAppDescriptor } from "../lib/app-catalog";
import type { OsWindow } from "../lib/os-types";
import { sessionSignal, tabSignal } from "../lib/tab-signal";
import { TAB_MAX_WIDTH, tabPresentation } from "../lib/tab-width";
import { ensureWindowSlotStore } from "../lib/window-slot-registry";

const TILE_HEIGHT = 20;
const TILE_SHRINK = 2;

function sessionProvider(session: SessionPayload | undefined): string | null {
  const runtime = session?.runtime;
  const provider =
    runtime?.effective?.provider ?? runtime?.last_known?.provider ?? runtime?.selected?.provider;
  return provider?.trim() ? provider : null;
}

/** The mark on a window's front tile: a session's provider, else its app's icon. */
function TileMark({ win, session }: { win: OsWindow; session: SessionPayload | undefined }) {
  const provider = sessionProvider(session);
  if (provider !== null) {
    return (
      <KindIcon aria-hidden="true" kind={provider} registry={providerKindIconRegistry} size="xs" />
    );
  }
  const AppIcon = getOsAppDescriptor(win.app).icon;
  return <AppIcon aria-hidden="true" className="size-3 text-fg-2" />;
}

/** The raised tile that stands for one window; a split stacks one per member behind the lead. */
export function OsTabTiles({
  members,
  sessionOf,
}: {
  members: readonly OsWindow[];
  sessionOf: (win: OsWindow) => SessionPayload | undefined;
}) {
  const shown = members.slice(0, TAB_TILE_STACK_MAX);
  const width = TAB_TILE_WIDTH + TAB_TILE_PEEK * (shown.length - 1);
  return (
    <span
      aria-hidden="true"
      className="relative block shrink-0"
      data-slot="os-tab-tiles"
      style={{ width, height: TILE_HEIGHT }}
    >
      {shown.map((win, depth) => {
        const style: CSSProperties = {
          width: TAB_TILE_WIDTH - TILE_SHRINK * depth,
          height: TILE_HEIGHT - TILE_SHRINK * depth,
          left: TAB_TILE_PEEK * depth + TILE_SHRINK * depth,
          top: (TILE_SHRINK * depth) / 2,
          zIndex: TAB_TILE_STACK_MAX - depth,
        };
        return (
          <span
            key={win.id}
            className={cn(
              "absolute grid place-items-center rounded-[5px]",
              // Opaque mixes: a translucent front tile would show the one tucked behind it.
              depth === 0
                ? "bg-[color-mix(in_srgb,var(--color-fg)_13%,var(--color-rail))]"
                : "bg-[color-mix(in_srgb,var(--color-fg)_20%,var(--color-rail))]"
            )}
            style={style}
          >
            {depth === 0 ? <TileMark win={win} session={sessionOf(win)} /> : null}
          </span>
        );
      })}
    </span>
  );
}

/** The tooltip's full trail: every drill-in crumb of the lead window, then the tab's title. */
function OsTabTrail({
  crumbs,
  title,
}: {
  crumbs: TopbarSlotValue["crumbs"] | undefined;
  title: string;
}) {
  return (
    <span className="flex max-w-64 min-w-0 items-center gap-1">
      {(crumbs ?? []).map(part => (
        <Fragment key={part.id}>
          <span className="truncate">{part.label}</span>
          <span aria-hidden="true">/</span>
        </Fragment>
      ))}
      <span className="truncate">{title}</span>
    </span>
  );
}

export interface OsTabProps {
  tabId: string;
  members: readonly OsWindow[];
  pinned: boolean;
  active: boolean;
  sessionOf: (win: OsWindow) => SessionPayload | undefined;
  /** Allocated width; omitted renders at the widest. Pinned tabs ignore it. */
  width?: number;
  /** This tab is being dragged. */
  dragging?: boolean;
  /** A dragged window head is held over this tab (US-008.AC-3). */
  dwelling?: boolean;
  onActivate?: () => void;
  onClose?: () => void;
  /** Starts a drag: along the bar to reorder, into the content to split. */
  onPointerDown?: (event: PointerEvent<HTMLElement>) => void;
}

/**
 * One plate per tab: an icon tile per window (a split stacks up to three, the lead in front),
 * the lead window's title, the member count of a split, the loudest signal, the waiting count,
 * and a close button. A narrowing tab drops them in that order from the end (`tabPresentation`).
 */
export function OsTab({
  tabId,
  members,
  pinned,
  active,
  sessionOf,
  width,
  dragging = false,
  dwelling = false,
  onActivate,
  onClose,
  onPointerDown,
}: OsTabProps) {
  const lead = members[0]!;
  const title = useOsWindowTitle(lead, sessionOf(lead));
  const crumbs = useWindowMemberSlot(ensureWindowSlotStore(lead.id))?.crumbs;
  const signal = tabSignal(members.map(member => sessionSignal(sessionOf(member))));
  const pending = members.reduce((sum, member) => {
    const session = sessionOf(member);
    return sum + (member.app === "session" && session ? pendingInteractions(session).length : 0);
  }, 0);
  const label = members.length > 1 ? `${title} and ${members.length - 1} more` : title;
  const plateWidth = pinned ? pinnedTabWidth(members.length) : (width ?? TAB_MAX_WIDTH);
  const show = tabPresentation(plateWidth, active);
  const iconOnly = !pinned && !show.title;

  let signalMark: ReactNode = null;
  if (signal) signalMark = <StateGlyph label={signal.label} size="sm" state={signal.state} />;
  // The inline signal moves to the tile's corner once the tab is too narrow to show it.
  const cornerSignal = signalMark !== null && (pinned || !show.signal);

  const closeOnMiddleClick = (event: MouseEvent<HTMLButtonElement>) => {
    if (event.button !== 1 || pinned) return;
    event.preventDefault();
    onClose?.();
  };

  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <div
            className={cn(
              "group/tab relative flex h-6.75 shrink-0 items-center rounded-sm pl-[3.5px] text-body font-medium transition-colors duration-fast select-none",
              // A pinned tab hugs its tile: the same 3.5px on every side.
              pinned ? "pr-[3.5px]" : "pr-1",
              active ? "bg-fg/7 text-fg" : "bg-fg/4 text-fg-2 hover:bg-fg/6 hover:text-fg",
              "has-[:focus-visible]:shadow-focus-ring",
              dwelling && "bg-brand-tint-strong text-fg ring-1 ring-brand",
              dragging && "opacity-40"
            )}
            data-dragging={dragging ? "" : undefined}
            data-active={active ? "" : undefined}
            data-pinned={pinned ? "" : undefined}
            data-slot="os-tab"
            data-tab-id={tabId}
            style={{ width: plateWidth }}
          >
            <button
              type="button"
              role="tab"
              aria-label={label}
              aria-selected={active}
              data-testid={`os-tab-${tabId}`}
              data-active={active ? "" : undefined}
              data-pinned={pinned ? "" : undefined}
              data-lead-window-id={lead.id}
              className={cn(
                "flex h-full min-w-0 flex-1 cursor-pointer items-center gap-2 rounded-[inherit] text-left outline-none",
                iconOnly && "justify-center"
              )}
              onClick={onActivate}
              onAuxClick={closeOnMiddleClick}
              onPointerDown={onPointerDown}
            >
              <span className="relative shrink-0">
                <OsTabTiles members={members} sessionOf={sessionOf} />
                {cornerSignal ? (
                  <span className="absolute -top-1 -right-1 z-10 grid place-items-center rounded-pill bg-rail p-px">
                    {signalMark}
                  </span>
                ) : null}
              </span>
              {pinned ? null : (
                <>
                  {show.title ? <span className="min-w-0 flex-1 truncate">{title}</span> : null}
                  {members.length > 1 && show.memberCount ? (
                    <span className="shrink-0 text-meta text-subtle tabular-nums">
                      {members.length}
                    </span>
                  ) : null}
                  {signalMark && show.signal ? (
                    <span className="grid shrink-0 place-items-center">{signalMark}</span>
                  ) : null}
                  {show.waitingCount ? <Pill.Count count={pending} /> : null}
                </>
              )}
            </button>
            {pinned || !show.close ? null : (
              <button
                type="button"
                aria-label="Close tab"
                className={cn(
                  "ml-1 grid size-4.5 shrink-0 place-items-center rounded-xs text-subtle opacity-0 transition-opacity duration-fast",
                  "hover:bg-fg/10 hover:text-fg focus-visible:opacity-100 focus-visible:shadow-focus-ring focus-visible:outline-none",
                  "group-hover/tab:opacity-100",
                  active && "opacity-100"
                )}
                data-slot="os-tab-close"
                onClick={onClose}
                onPointerDown={event => event.stopPropagation()}
              >
                <X aria-hidden="true" className="size-3" />
              </button>
            )}
          </div>
        }
      />
      <TooltipContent side="bottom">
        <OsTabTrail crumbs={crumbs} title={label} />
      </TooltipContent>
    </Tooltip>
  );
}
