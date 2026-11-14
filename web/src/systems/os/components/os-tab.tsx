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
import { Fragment, type CSSProperties, type MouseEvent, type PointerEvent } from "react";

import { cn } from "@/lib/utils";
import {
  AgentMark,
  agentAvatarAppearance,
  alwaysOnMainInstance,
  useAgentIdentityView,
  useAgentInstances,
} from "@/systems/agent";
import type { SessionPayload } from "@/systems/session";

import {
  TAB_TILE_PEEK,
  TAB_TILE_STACK_MAX,
  TAB_TILE_WIDTH,
  useTabSession,
} from "../hooks/use-os-tab-bar";
import { useOsTabPlate } from "../hooks/use-os-tab-plate";
import type { TabRovingProps } from "../hooks/use-tab-bar-roving";
import { getOsAppDescriptor } from "../lib/app-catalog";
import type { OsWindow } from "../lib/os-types";
import type { TabSignal } from "../lib/tab-signal";
import type { TabPresentation } from "../lib/tab-width";

/**
 * A tab names its trail only when the pointer rests on it, not while it passes or presses, in ms:
 * an instant tooltip mounted and positioned its popup inside every tab press (E2E-023).
 */
const TAB_TOOLTIP_DELAY = 700;

const TILE_HEIGHT = 20;
const TILE_SHRINK = 2;

function sessionProvider(session: SessionPayload | undefined): string | null {
  const runtime = session?.runtime;
  const provider =
    runtime?.effective?.provider ?? runtime?.last_known?.provider ?? runtime?.selected?.provider;
  return provider?.trim() ? provider : null;
}

interface TileMarkProps {
  win: OsWindow;
  session: SessionPayload | undefined;
}

/** The fallback mark: a session's provider, else its app's icon. */
function GenericTileMark({ win, session }: TileMarkProps) {
  const provider = sessionProvider(session);
  if (provider !== null) {
    return (
      <KindIcon aria-hidden="true" kind={provider} registry={providerKindIconRegistry} size="xs" />
    );
  }
  const AppIcon = getOsAppDescriptor(win.app).icon;
  return <AppIcon aria-hidden="true" className="size-3 text-fg-2" />;
}

/** The size of an agent's face on a 20px tile, as the sessions list draws it (#1709). */
const TILE_AGENT_MARK_SIZE = 16;

/** A defined agent's face from the cached catalog, scoped to the session's recorded owner. */
function AgentTileMark({ session }: { session: SessionPayload }) {
  const agent = useAgentIdentityView(session.agent_name ?? "", {
    workspace: session.workspace_id,
    profile: session.profile_name,
  });
  return (
    <AgentMark
      aria-hidden="true"
      appearance={agent.appearance}
      label={agent.label}
      quietIdle
      size={TILE_AGENT_MARK_SIZE}
      state="unknown"
    />
  );
}

/**
 * A session window's mark: an always-on main shows its instance's avatar as the session head does,
 * any other agent session its agent's face, and a session with no agent the generic mark (#1709).
 * Both reads share the instance and catalog queries the shell already holds, never one per tab.
 */
function SessionTileMark({ win, session: listed }: TileMarkProps) {
  const session = useTabSession(win.instanceKey ?? "", listed);
  const instance = alwaysOnMainInstance(useAgentInstances(), win.instanceKey ?? "");
  if (instance) {
    return (
      <AgentMark
        aria-hidden="true"
        appearance={agentAvatarAppearance(instance.resolved_appearance, instance.agent_name)}
        label={instance.label}
        quietIdle
        size={TILE_AGENT_MARK_SIZE}
        state="unknown"
      />
    );
  }
  if (session?.agent_name) return <AgentTileMark session={session} />;
  return <GenericTileMark win={win} session={session} />;
}

/** The mark on a window's front tile: a session's agent, else its provider or app icon. */
function TileMark({ win, session }: TileMarkProps) {
  if (win.app === "session") return <SessionTileMark win={win} session={session} />;
  return <GenericTileMark win={win} session={session} />;
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
              "absolute grid place-items-center rounded-xxs",
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

/** The tab's loudest signal as a glyph, inline or in the tile's corner. */
function OsTabSignalGlyph({ signal }: { signal: TabSignal }) {
  return <StateGlyph label={signal.label} size="sm" state={signal.state} />;
}

/** The tiles, with the signal in their corner once the tab is too narrow to show it inline. */
function OsTabLeadTiles({
  members,
  sessionOf,
  signal,
  cornerSignal,
}: {
  members: readonly OsWindow[];
  sessionOf: (win: OsWindow) => SessionPayload | undefined;
  signal: TabSignal | null;
  cornerSignal: boolean;
}) {
  return (
    <span className="relative shrink-0">
      <OsTabTiles members={members} sessionOf={sessionOf} />
      {cornerSignal && signal ? (
        <span className="absolute -top-1 -right-1 z-10 grid place-items-center rounded-pill bg-rail p-px">
          <OsTabSignalGlyph signal={signal} />
        </span>
      ) : null}
    </span>
  );
}

/** An unpinned tab's title, member count, inline signal and waiting count, as its width keeps. */
function OsTabDetails({
  title,
  memberCount,
  signal,
  pending,
  show,
}: {
  title: string;
  memberCount: number;
  signal: TabSignal | null;
  pending: number;
  show: TabPresentation;
}) {
  return (
    <>
      {show.title ? <span className="min-w-0 flex-1 truncate">{title}</span> : null}
      {memberCount > 1 && show.memberCount ? (
        <span className="shrink-0 text-meta text-subtle tabular-nums">{memberCount}</span>
      ) : null}
      {signal && show.signal ? (
        <span className="grid shrink-0 place-items-center">
          <OsTabSignalGlyph signal={signal} />
        </span>
      ) : null}
      {show.waitingCount ? <Pill.Count count={pending} /> : null}
    </>
  );
}

/** The plate's close button; outside the bar's one tab stop when the bar roves focus. */
function OsTabCloseButton({
  active,
  roving,
  onClose,
}: {
  active: boolean;
  roving: TabRovingProps | undefined;
  onClose: (() => void) | undefined;
}) {
  return (
    <button
      type="button"
      aria-label="Close tab"
      // The bar keeps one tab stop; the keyboard closes through the menu or shortcut.
      tabIndex={roving ? -1 : undefined}
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
  );
}

/** A present-or-absent data attribute: `""` when set, omitted otherwise. */
function dataFlag(on: boolean): "" | undefined {
  return on ? "" : undefined;
}

function plateClassName(pinned: boolean, active: boolean, dwelling: boolean, dragging: boolean) {
  return cn(
    "group/tab relative flex h-6.75 shrink-0 items-center rounded-sm pl-[3.5px] text-body font-medium transition-colors duration-fast select-none",
    // A pinned tab hugs its tile: the same 3.5px on every side.
    pinned ? "pr-[3.5px]" : "pr-1",
    active ? "bg-fg/7 text-fg" : "bg-fg/4 text-fg-2 hover:bg-fg/6 hover:text-fg",
    "has-[:focus-visible]:shadow-focus-ring",
    dwelling && "bg-brand-tint-strong text-fg ring-1 ring-brand",
    dragging && "opacity-40"
  );
}

/**
 * Middle-click closes like "Close tab" (US-002.AC-4): a pinned split tab closes whole; only the
 * last window of a pinned tab is refused (EC-3).
 */
function middleClickClose(pinned: boolean, memberCount: number, onClose: (() => void) | undefined) {
  return (event: MouseEvent<HTMLButtonElement>) => {
    if (event.button !== 1 || (pinned && memberCount === 1)) return;
    event.preventDefault();
    onClose?.();
  };
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
  /** The bar's roving tab stop and arrow keys; omitted, the tab is a plain tab stop. */
  roving?: TabRovingProps;
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
  roving,
}: OsTabProps) {
  const plate = useOsTabPlate({ members, pinned, active, sessionOf, width });
  const closeOnMiddleClick = middleClickClose(pinned, members.length, onClose);

  return (
    <Tooltip>
      <TooltipTrigger
        delay={TAB_TOOLTIP_DELAY}
        render={
          <div
            className={plateClassName(pinned, active, dwelling, dragging)}
            data-dragging={dataFlag(dragging)}
            data-active={dataFlag(active)}
            data-pinned={dataFlag(pinned)}
            data-slot="os-tab"
            data-tab-id={tabId}
            style={{ width: plate.plateWidth }}
          >
            <button
              type="button"
              role="tab"
              aria-label={plate.label}
              aria-selected={active}
              data-testid={`os-tab-${tabId}`}
              data-active={dataFlag(active)}
              data-pinned={dataFlag(pinned)}
              data-lead-window-id={plate.lead.id}
              {...roving}
              className={cn(
                "flex h-full min-w-0 flex-1 cursor-pointer items-center gap-2 rounded-[inherit] text-left outline-none",
                plate.iconOnly && "justify-center"
              )}
              onClick={onActivate}
              onAuxClick={closeOnMiddleClick}
              onPointerDown={onPointerDown}
            >
              <OsTabLeadTiles
                members={members}
                sessionOf={sessionOf}
                signal={plate.signal}
                cornerSignal={plate.cornerSignal}
              />
              {pinned ? null : (
                <OsTabDetails
                  title={plate.title}
                  memberCount={members.length}
                  signal={plate.signal}
                  pending={plate.pending}
                  show={plate.show}
                />
              )}
            </button>
            {pinned || !plate.show.close ? null : (
              <OsTabCloseButton active={active} roving={roving} onClose={onClose} />
            )}
          </div>
        }
      />
      <TooltipContent side="bottom">
        <OsTabTrail crumbs={plate.crumbs} title={plate.label} />
      </TooltipContent>
    </Tooltip>
  );
}
