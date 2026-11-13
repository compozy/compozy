import {
  Button,
  DeleteButton,
  Input,
  Kbd,
  NativeSelect,
  NativeSelectOption,
  StatusDot,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  cn,
} from "@compozy/ui";
import { GripVertical, Pencil } from "@compozy/ui/icons";
import { useState, type PointerEvent, type ReactNode, type RefObject } from "react";

import type { SessionPayload } from "@/systems/session";

import { DESKTOP_DROP_TARGET_ATTRIBUTE } from "../hooks/use-os-tab-drag";
import type { OsAppId, OsWindow } from "../lib/os-types";
import { DesktopLayoutThumbnail } from "./desktop-layout-thumbnail";
import { DesktopOverviewCardMenu } from "./desktop-overview-card-menu";
import { DesktopOverviewTabRow } from "./desktop-overview-tab-row";
import type { DesktopOverviewItem } from "./desktops-overview";

/** Tab rows a card lists before it counts the rest as "+N more" (US-014.EC-3). */
export const DESKTOP_CARD_TAB_ROWS = 4;

function tabCount(count: number): string {
  return `${count} tab${count === 1 ? "" : "s"}`;
}

export interface DesktopOverviewCardProps {
  desktop: DesktopOverviewItem;
  desktops: readonly DesktopOverviewItem[];
  index: number;
  active: boolean;
  busy: boolean;
  sessionOf: (win: OsWindow) => SessionPayload | undefined;
  /** A dragged tab is over this card and would move here on release. */
  dropTarget?: boolean;
  /** The floating copy of a card being dragged to a new position. */
  lifted?: boolean;
  thumbnailRef?: RefObject<HTMLButtonElement | null>;
  onSwitch: () => void;
  onChooseTab: (windowId: string) => void;
  onTabPointerDown?: (tabId: string, event: PointerEvent<HTMLElement>) => void;
  onCardPointerDown?: (event: PointerEvent<HTMLElement>) => void;
  onRename: (name: string) => void;
  onReorder: (order: number) => void;
  onDelete: (destinationId: string | null) => void;
  onMoveTab: (leadWindowId: string, destinationDesktopId: string) => void;
}

/** Where a desktop that is being closed sends its tabs; empty until one is chosen. */
interface Closing {
  destinationId: string;
}

/**
 * One desktop in the overview: its layout thumbnail (which switches to it), name and tab count,
 * rename and close, and its tabs. Closing opens the in-place confirm; a desktop with tabs asks
 * where they go before the check is available (US-014.EC-1). The last desktop has no close.
 */
export function DesktopOverviewCard({
  desktop,
  desktops,
  index,
  active,
  busy,
  sessionOf,
  dropTarget = false,
  lifted = false,
  thumbnailRef,
  onSwitch,
  onChooseTab,
  onTabPointerDown,
  onCardPointerDown,
  onRename,
  onReorder,
  onDelete,
  onMoveTab,
}: DesktopOverviewCardProps) {
  const [renaming, setRenaming] = useState(false);
  const [closing, setClosing] = useState<Closing | null>(null);
  const needsYou = desktop.needsYou === true && !active;
  const shown = desktop.tabs.slice(0, DESKTOP_CARD_TAB_ROWS);
  const hidden = desktop.tabs.length - shown.length;
  const destinations = desktops.filter(candidate => candidate.id !== desktop.id);
  const needsDestination = desktop.tabs.length > 0;
  const apps = new Map<string, OsAppId>();
  for (const tab of desktop.tabs) for (const member of tab.members) apps.set(member.id, member.app);

  return (
    <DesktopOverviewCardMenu
      desktop={desktop}
      desktops={desktops}
      disabled={busy || lifted}
      index={index}
      sessionOf={sessionOf}
      onMoveTab={onMoveTab}
      onRename={() => {
        setClosing(null);
        setRenaming(true);
      }}
      onReorder={onReorder}
    >
      <li
        {...{ [DESKTOP_DROP_TARGET_ATTRIBUTE]: lifted ? undefined : desktop.id }}
        aria-current={active ? "true" : undefined}
        aria-label={desktop.name}
        className={cn(
          "group/card relative flex min-w-0 flex-col gap-3 rounded-xl bg-card p-3 shadow-card transition-shadow duration-fast",
          active && "ring-2 ring-fg",
          dropTarget && "bg-brand-tint ring-2 ring-brand",
          lifted && "z-10 rotate-1 shadow-elevated"
        )}
        data-current={active ? "true" : undefined}
        data-card-desktop-id={desktop.id}
        data-drop-target={dropTarget ? "true" : undefined}
        data-slot="desktop-card"
        onPointerDown={onCardPointerDown}
      >
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
        {renaming ? (
          <RenameDesktop
            busy={busy}
            name={desktop.name}
            onCancel={() => setRenaming(false)}
            onRename={name => {
              onRename(name);
              setRenaming(false);
            }}
          />
        ) : (
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
              {closing === null && desktop.switchShortcut ? (
                <Kbd>{desktop.switchShortcut}</Kbd>
              ) : null}
              <DesktopCardAction
                disabled={busy}
                label="Rename"
                onClick={() => {
                  setClosing(null);
                  setRenaming(true);
                }}
              >
                <Pencil aria-hidden="true" className="size-3.75" />
              </DesktopCardAction>
              {desktops.length > 1 ? (
                <DeleteButton
                  canConfirm={!needsDestination || (closing?.destinationId ?? "") !== ""}
                  disabled={busy}
                  label={`Close ${desktop.name}`}
                  onConfirm={() =>
                    onDelete(needsDestination ? (closing?.destinationId ?? null) : null)
                  }
                  onPhaseChange={phase => {
                    if (phase === "open") setClosing({ destinationId: "" });
                    if (phase === "rest") setClosing(null);
                  }}
                />
              ) : null}
            </span>
          </div>
        )}
        {closing !== null && needsDestination ? (
          <label className="flex items-center gap-2 text-small-body text-muted-foreground">
            <span className="shrink-0">Move {tabCount(desktop.tabs.length)} to</span>
            <NativeSelect
              className="flex-1"
              disabled={busy}
              value={closing.destinationId}
              onChange={event => setClosing({ destinationId: event.currentTarget.value })}
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
        ) : null}
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
                onPointerDown={
                  onTabPointerDown ? event => onTabPointerDown(tab.id, event) : undefined
                }
              />
            </li>
          ))}
          {hidden > 0 ? (
            <li className="px-1 pt-1 text-meta text-muted-foreground">+{hidden} more</li>
          ) : null}
          {dropTarget ? (
            <li className="mt-1 rounded-sm border border-dashed border-brand py-1 text-center text-meta text-brand-strong">
              Move tab here
            </li>
          ) : null}
        </ul>
      </li>
    </DesktopOverviewCardMenu>
  );
}

/** Where a dragged card would land: the card shows as this gap. */
export function DesktopOverviewCardGap({ desktopId }: { desktopId: string }) {
  return (
    <li
      aria-hidden="true"
      className="min-h-72 self-stretch rounded-xl border border-dashed border-line-strong bg-fg/4"
      data-card-desktop-id={desktopId}
      data-slot="desktop-gap"
    />
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

function RenameDesktop({
  name: currentName,
  busy,
  onCancel,
  onRename,
}: {
  name: string;
  busy: boolean;
  onCancel: () => void;
  onRename: (name: string) => void;
}) {
  const [name, setName] = useState(currentName);
  const nextName = name.trim();
  const canSave = nextName.length > 0 && nextName !== currentName;
  return (
    <form
      className="flex min-h-7 items-center gap-2"
      onSubmit={event => {
        event.preventDefault();
        if (canSave) onRename(nextName);
      }}
    >
      <Input
        aria-label="Desktop name"
        autoFocus
        className="h-7 flex-1"
        disabled={busy}
        value={name}
        onChange={event => setName(event.currentTarget.value)}
        onKeyDown={event => {
          if (event.key !== "Escape") return;
          // Escape leaves the rename only; the overview stays open.
          event.preventDefault();
          event.stopPropagation();
          onCancel();
        }}
      />
      <span className="flex shrink-0 items-center gap-0.5">
        <Button disabled={busy} size="sm" type="button" variant="ghost" onClick={onCancel}>
          Cancel
        </Button>
        <Button disabled={busy || !canSave} size="sm" type="submit">
          Save
        </Button>
      </span>
    </form>
  );
}
