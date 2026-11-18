import {
  Button,
  DeleteButton,
  Kbd,
  NativeSelect,
  NativeSelectOption,
  StatusDot,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";
import { GripVertical, Pencil } from "@compozy/ui/icons";
import type { Dispatch, PointerEvent, ReactNode, RefObject, SetStateAction } from "react";

import type { SessionPayload } from "@/systems/session";

import type { DesktopCardClosing } from "../hooks/use-desktop-card-actions";
import type { OsAppId, OsWindow } from "../lib/os-types";
import { DesktopLayoutThumbnail } from "./desktop-layout-thumbnail";
import { DesktopOverviewTabRow } from "./desktop-overview-tab-row";
import type { DesktopOverviewItem } from "../lib/desktops-overview-model";

/** Tab rows a card lists before it counts the rest as "+N more" (US-014.EC-3). */
const DESKTOP_CARD_TAB_ROWS = 4;

function tabCount(count: number): string {
  return `${count} tab${count === 1 ? "" : "s"}`;
}

/** The card's layout thumbnail; pressing it switches to the desktop. */
export function DesktopCardThumbnail({
  desktop,
  active,
  needsYou,
  busy,
  thumbnailRef,
  onSwitch,
}: {
  desktop: DesktopOverviewItem;
  active: boolean;
  needsYou: boolean;
  busy: boolean;
  thumbnailRef: RefObject<HTMLButtonElement | null> | undefined;
  onSwitch: () => void;
}) {
  const apps = new Map<string, OsAppId>();
  for (const tab of desktop.tabs) for (const member of tab.members) apps.set(member.id, member.app);
  return (
    <button
      ref={thumbnailRef}
      aria-label={`${active ? "Current desktop" : "Switch to"} ${desktop.name}${needsYou ? " — needs you" : ""}`}
      className="block w-full rounded-md outline-none focus-visible:shadow-focus-ring disabled:cursor-not-allowed"
      data-slot="desktops-overview-card"
      disabled={busy}
      onClick={onSwitch}
      type="button"
    >
      <DesktopLayoutThumbnail
        activeTabId={desktop.activeTabId}
        appOf={windowId => apps.get(windowId)}
        root={desktop.activeRoot}
        tabIds={desktop.tabs.map(tab => tab.id)}
      />
    </button>
  );
}

/** The name row: name, tab count, needs-you mark, switch chord, rename and the in-place close. */
export function DesktopCardNameRow({
  desktop,
  desktopCount,
  busy,
  needsYou,
  needsDestination,
  initialAction,
  closing,
  setClosing,
  closeKey,
  binRef,
  onStartRename,
  onDelete,
}: {
  desktop: DesktopOverviewItem;
  desktopCount: number;
  busy: boolean;
  needsYou: boolean;
  needsDestination: boolean;
  initialAction: "rename" | "close" | undefined;
  closing: DesktopCardClosing | null;
  setClosing: Dispatch<SetStateAction<DesktopCardClosing | null>>;
  closeKey: number;
  binRef: RefObject<HTMLButtonElement | null>;
  onStartRename: () => void;
  onDelete: (destinationId: string | null) => void;
}) {
  return (
    <div className="flex min-h-7 items-center gap-2">
      <GripVertical
        aria-hidden="true"
        className="-ml-1 size-3.5 shrink-0 cursor-grab text-subtle opacity-0 group-hover/card:opacity-100"
      />
      <span className="truncate font-medium text-fg">{desktop.name}</span>
      <span className="shrink-0 text-meta text-muted-foreground">
        {tabCount(desktop.tabs.length)}
      </span>
      {needsYou ? <StatusDot label="Needs you" size="sm" tone="attn" /> : null}
      <span className="ml-auto flex shrink-0 items-center gap-0.5">
        {closing === null && desktop.switchShortcut ? <Kbd>{desktop.switchShortcut}</Kbd> : null}
        <DesktopCardAction disabled={busy} label="Rename" onClick={onStartRename}>
          <Pencil aria-hidden="true" className="size-3.75" />
        </DesktopCardAction>
        {desktopCount > 1 ? (
          <DeleteButton
            key={closeKey}
            binRef={binRef}
            // Opened from the indicator menu's Close desktop, the confirm takes focus.
            autoFocusCancel={initialAction === "close"}
            phase={closing?.phase ?? "rest"}
            canConfirm={!needsDestination || (closing?.destinationId ?? "") !== ""}
            disabled={busy}
            label={`Close ${desktop.name}`}
            onConfirm={() => onDelete(needsDestination ? (closing?.destinationId ?? null) : null)}
            onPhaseChange={phase => {
              if (phase === "rest") setClosing(null);
              else
                setClosing(previous => ({
                  phase,
                  destinationId: phase === "open" ? "" : (previous?.destinationId ?? ""),
                }));
            }}
          />
        ) : null}
      </span>
    </div>
  );
}

/** While closing a desktop with tabs: where its tabs go, chosen before the check confirms. */
export function DesktopCardCloseDestination({
  desktop,
  desktops,
  busy,
  closing,
  setClosing,
}: {
  desktop: DesktopOverviewItem;
  desktops: readonly DesktopOverviewItem[];
  busy: boolean;
  closing: DesktopCardClosing;
  setClosing: Dispatch<SetStateAction<DesktopCardClosing | null>>;
}) {
  const destinations = desktops.filter(candidate => candidate.id !== desktop.id);
  return (
    <label className="flex items-center gap-2 text-small-body text-muted-foreground">
      <span className="shrink-0">Move {tabCount(desktop.movableTabCount)} to</span>
      <NativeSelect
        className="flex-1"
        disabled={busy}
        value={closing.destinationId}
        onChange={event => setClosing({ phase: "open", destinationId: event.currentTarget.value })}
      >
        {closing.destinationId === "" ? (
          <NativeSelectOption value="">Choose a desktop</NativeSelectOption>
        ) : null}
        {destinations.map(destination => (
          <NativeSelectOption key={destination.id} value={destination.id}>
            {destination.name}
          </NativeSelectOption>
        ))}
      </NativeSelect>
    </label>
  );
}

/** The card's first tabs, "+N more" for the rest, and the drop hint while a drag is over it. */
export function DesktopCardTabList({
  desktop,
  busy,
  dropTarget,
  dragKind,
  sessionOf,
  onChooseTab,
  onTabPointerDown,
}: {
  desktop: DesktopOverviewItem;
  busy: boolean;
  dropTarget: boolean;
  dragKind: "tab" | "window";
  sessionOf: (win: OsWindow) => SessionPayload | undefined;
  onChooseTab: (windowId: string) => void;
  onTabPointerDown: ((tabId: string, event: PointerEvent<HTMLElement>) => void) | undefined;
}) {
  const shown = desktop.tabs.slice(0, DESKTOP_CARD_TAB_ROWS);
  const hidden = desktop.tabs.length - shown.length;
  return (
    <ul aria-label={`Tabs on ${desktop.name}`} className="flex flex-col gap-0.5">
      {shown.map(tab => (
        <li key={tab.id}>
          <DesktopOverviewTabRow
            active={tab.id === desktop.activeTabId}
            disabled={busy}
            members={tab.members}
            sessionOf={sessionOf}
            tabId={tab.id}
            onChoose={() => onChooseTab(tab.leadWindowId)}
            onPointerDown={onTabPointerDown ? event => onTabPointerDown(tab.id, event) : undefined}
          />
        </li>
      ))}
      {hidden > 0 ? (
        <li className="px-1 pt-1 text-meta text-muted-foreground">+{hidden} more</li>
      ) : null}
      {dropTarget ? (
        <li className="mt-1 rounded-sm border border-dashed border-brand py-1 text-center text-meta text-brand-strong">
          Move {dragKind} here
        </li>
      ) : null}
    </ul>
  );
}

function DesktopCardAction({
  label,
  disabled,
  onClick,
  children,
}: {
  label: string;
  disabled: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            aria-label={label}
            disabled={disabled}
            size="icon-sm"
            type="button"
            variant="quiet"
            onClick={onClick}
          />
        }
      >
        {children}
      </TooltipTrigger>
      <TooltipContent side="bottom">{label}</TooltipContent>
    </Tooltip>
  );
}
