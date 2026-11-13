/**
 * The desktops overview (S8): an overlay above the current desktop, one card per desktop with a
 * thumbnail drawn from its active tab's split tree, its tabs, and its actions.
 */
import { GripVertical, Pencil, Plus } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import {
  Button,
  Input,
  Kbd,
  NativeSelect,
  NativeSelectOption,
  StateGlyph,
  StatusDot,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  cn,
} from "@compozy/ui";

import { WINDOWS } from "../window-tabs-shared/fixtures";
import {
  leadWindowId,
  leafIds,
  type DraftDesktop,
  type DraftTab,
  type LayoutNode,
} from "../window-tabs-shared/model";
import { APP_ICON } from "../window-tabs-shared/apps";
import { OsTabTiles } from "../window-tabs-shared/os-tab";
import { DeleteButton, type DeleteButtonPhase } from "./delete-button";

const TAB_ROWS = 4;

function ThumbnailNode({ node }: { node: LayoutNode }) {
  if (node.kind === "leaf") {
    const Icon = APP_ICON[WINDOWS[node.windowId]!.app];
    return (
      <span className="relative grid min-h-0 min-w-0 flex-1 place-items-center overflow-hidden rounded-xxs bg-elevated shadow-card">
        <span className="absolute inset-x-0 top-0 h-1.75 border-b border-line-soft bg-surface-2" />
        <Icon aria-hidden="true" className="mt-1.5 size-4 text-subtle" />
      </span>
    );
  }
  return (
    <span
      className={cn(
        "flex min-h-0 min-w-0 flex-1 gap-0.5",
        node.direction === "row" ? "flex-row" : "flex-col"
      )}
    >
      {node.children.map((child, index) => (
        <span
          className="flex min-h-0 min-w-0"
          key={child.kind === "leaf" ? child.windowId : child.id}
          style={{ flex: `${node.weights[index] ?? 1} 1 0` }}
        >
          <ThumbnailNode node={child} />
        </span>
      ))}
    </span>
  );
}

/** A miniature of the desktop: its tab strip, then its active tab's windows by share. */
function DesktopThumbnail({ desktop }: { desktop: DraftDesktop }) {
  const active = desktop.tabs.find(tab => tab.id === desktop.activeTabId) ?? desktop.tabs[0]!;
  return (
    <span className="flex aspect-16/10 w-full flex-col gap-1 rounded-md bg-rail p-1.5">
      <span className="flex h-2 shrink-0 items-center gap-0.5">
        {desktop.tabs.slice(0, 8).map(tab => (
          <span
            className={cn(
              "h-full max-w-8 flex-1 rounded-[2px]",
              tab.id === active.id ? "bg-fg/25" : "bg-fg/8"
            )}
            key={tab.id}
          />
        ))}
      </span>
      <ThumbnailNode node={active.root} />
    </span>
  );
}

function TabRow({ tab, active, lifted }: { tab: DraftTab; active: boolean; lifted?: boolean }) {
  const members = leafIds(tab.root).map(id => WINDOWS[id]!);
  const lead = WINDOWS[leadWindowId(tab)]!;
  const signal = members.find(member => member.signal)?.signal;
  return (
    <button
      className={cn(
        "flex h-7 w-full min-w-0 items-center gap-2 rounded-sm px-1 text-left text-small-body outline-none hover:bg-fg/7 focus-visible:shadow-focus-ring",
        active ? "text-fg" : "text-fg-2",
        lifted && "bg-elevated shadow-pop"
      )}
      type="button"
    >
      <OsTabTiles windows={members} />
      <span className="min-w-0 flex-1 truncate">{lead.title}</span>
      {members.length > 1 ? (
        <span className="text-meta text-subtle tabular-nums">{members.length}</span>
      ) : null}
      {signal ? <StateGlyph size="sm" state={signal} /> : null}
    </button>
  );
}

export interface CardState {
  renaming?: boolean;
  /** The card is being dragged to a new position. */
  lifted?: boolean;
  /** Where a dragged card would land: this card shows as the gap. */
  placeholder?: boolean;
  /** A dragged tab is over this card. */
  dropTarget?: boolean;
  closing?: { phase: DeleteButtonPhase; destinationId?: string };
}

interface DesktopCardProps {
  desktop: DraftDesktop;
  index: number;
  active: boolean;
  canClose: boolean;
  destinations: readonly DraftDesktop[];
  state?: CardState;
}

function DesktopCard({
  desktop,
  index,
  active,
  canClose,
  destinations,
  state = {},
}: DesktopCardProps) {
  const shown = desktop.tabs.slice(0, TAB_ROWS);
  const hidden = desktop.tabs.length - shown.length;
  const closing = state.closing;
  if (state.placeholder) {
    return (
      <li
        className="min-h-72 self-stretch rounded-xl border border-dashed border-line-strong bg-fg/4"
        data-slot="desktop-gap"
      />
    );
  }
  return (
    <li
      className={cn(
        "group/card relative flex min-w-0 flex-col gap-3 rounded-xl bg-card p-3 shadow-card transition-shadow duration-fast",
        active && "ring-2 ring-fg",
        state.dropTarget && "bg-brand-tint ring-2 ring-brand",
        state.lifted && "z-10 rotate-1 shadow-elevated"
      )}
      data-slot="desktop-card"
    >
      <DesktopThumbnail desktop={desktop} />
      <div className="flex min-h-7 items-center gap-2">
        {state.renaming ? (
          <Input
            aria-label="Desktop name"
            autoFocus
            className="h-7 flex-1"
            defaultValue={desktop.name}
          />
        ) : (
          <>
            <GripVertical
              aria-hidden="true"
              className="-ml-1 size-3.5 shrink-0 cursor-grab text-subtle opacity-0 group-hover/card:opacity-100"
            />
            <span className="truncate font-medium text-fg">{desktop.name}</span>
            <span className="shrink-0 text-meta text-muted-foreground">
              {desktop.tabs.length} tab{desktop.tabs.length === 1 ? "" : "s"}
            </span>
            {desktop.needsYou && !active ? (
              <StatusDot label="Needs you" size="sm" tone="attn" />
            ) : null}
          </>
        )}
        <span className="ml-auto flex shrink-0 items-center gap-0.5">
          {state.renaming ? (
            <>
              <Button size="sm" variant="ghost">
                Cancel
              </Button>
              <Button size="sm">Save</Button>
            </>
          ) : (
            <>
              {closing ? null : <Kbd>{`⌃${index + 1}`}</Kbd>}
              <DesktopCardAction label="Rename">
                <Pencil aria-hidden="true" className="size-3.75" />
              </DesktopCardAction>
              {canClose ? (
                <DeleteButton
                  canConfirm={closing?.destinationId !== undefined}
                  label={`Close ${desktop.name}`}
                  phase={closing?.phase}
                />
              ) : null}
            </>
          )}
        </span>
      </div>
      {closing?.phase === "open" ? (
        <label className="flex items-center gap-2 text-small-body text-muted-foreground">
          <span className="shrink-0">
            Move {desktop.tabs.length} tab{desktop.tabs.length === 1 ? "" : "s"} to
          </span>
          <NativeSelect className="flex-1" defaultValue={closing.destinationId ?? ""}>
            {closing.destinationId === undefined ? (
              <NativeSelectOption value="">Choose a desktop</NativeSelectOption>
            ) : null}
            {destinations.map(destination => (
              <NativeSelectOption key={destination.id} value={destination.id}>
                {destination.name}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </label>
      ) : null}
      <ul className="flex flex-col gap-0.5">
        {shown.map(tab => (
          <li key={tab.id}>
            <TabRow active={tab.id === desktop.activeTabId} tab={tab} />
          </li>
        ))}
        {hidden > 0 ? (
          <li className="px-1 pt-1 text-meta text-muted-foreground">+{hidden} more</li>
        ) : null}
        {state.dropTarget ? (
          <li className="mt-1 rounded-sm border border-dashed border-brand py-1 text-center text-meta text-brand">
            Move tab here
          </li>
        ) : null}
      </ul>
    </li>
  );
}

function DesktopCardAction({ label, children }: { label: string; children: ReactNode }) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={<Button aria-label={label} size="icon-sm" type="button" variant="quiet" />}
      >
        {children}
      </TooltipTrigger>
      <TooltipContent side="bottom">{label}</TooltipContent>
    </Tooltip>
  );
}

export interface DesktopsOverviewDraftProps {
  desktops: readonly DraftDesktop[];
  activeDesktopId: string;
  cards?: Record<string, CardState>;
  /** A tab being dragged across the overview. */
  draggedTab?: { tab: DraftTab; left: string; top: string };
}

export function DesktopsOverviewDraft({
  desktops,
  activeDesktopId,
  cards,
  draggedTab,
}: DesktopsOverviewDraftProps) {
  return (
    <div
      aria-label="Desktops"
      className="relative z-40 col-start-2 row-span-2 row-start-2 flex min-h-0 flex-col gap-5 overflow-y-auto rounded-tl-lg bg-overlay-scrim px-8 py-7 backdrop-blur-md"
      data-slot="desktops-overview"
      role="dialog"
    >
      <header className="flex items-center gap-3">
        <div className="flex min-w-0 flex-col">
          <h2 className="text-heading font-medium tracking-tight text-fg">Desktops</h2>
          <p className="text-meta text-fg-2">Each desktop keeps its own tabs</p>
        </div>
        <div className="ml-auto flex items-center gap-2">
          <Button variant="secondary">
            <Plus aria-hidden="true" />
            New desktop
          </Button>
          <Button variant="secondary">Done</Button>
        </div>
      </header>
      <ol className="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] items-start gap-5">
        {desktops.map((desktop, index) => (
          <DesktopCard
            active={desktop.id === activeDesktopId}
            canClose={desktops.length > 1}
            desktop={desktop}
            destinations={desktops.filter(candidate => candidate.id !== desktop.id)}
            index={index}
            key={desktop.id}
            state={cards?.[desktop.id]}
          />
        ))}
      </ol>
      {draggedTab ? (
        <span
          className="pointer-events-none absolute z-50 w-56"
          style={{ left: draggedTab.left, top: draggedTab.top }}
        >
          <TabRow active lifted tab={draggedTab.tab} />
        </span>
      ) : null}
    </div>
  );
}
