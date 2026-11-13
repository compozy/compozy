import { Button, Input, cn } from "@compozy/ui";
import { useState, type PointerEvent, type RefObject } from "react";

import type { SessionPayload } from "@/systems/session";

import { useDesktopCardActions } from "../hooks/use-desktop-card-actions";
import { DESKTOP_DROP_TARGET_ATTRIBUTE } from "../hooks/use-os-tab-drag";
import type { OsWindow } from "../lib/os-types";
import {
  DesktopCardCloseDestination,
  DesktopCardNameRow,
  DesktopCardTabList,
  DesktopCardThumbnail,
} from "./desktop-overview-card-sections";
import { DesktopOverviewCardMenu } from "./desktop-overview-card-menu";
import type { DesktopOverviewItem } from "../lib/desktops-overview-model";

export interface DesktopOverviewCardProps {
  desktop: DesktopOverviewItem;
  desktops: readonly DesktopOverviewItem[];
  index: number;
  active: boolean;
  busy: boolean;
  sessionOf: (win: OsWindow) => SessionPayload | undefined;
  /** A dragged tab is over this card and would move here on release. */
  dropTarget?: boolean;
  /** What the drag over this card moves: a whole tab, or one window leaving its split. */
  dragKind?: "tab" | "window";
  /** Opened from the desktop's indicator menu: the card starts renaming or closing. */
  initialAction?: "rename" | "close";
  /** The card applied `initialAction`; the owner consumes it so a remount does not reapply it. */
  onInitialActionApplied?: () => void;
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

/**
 * One desktop in the overview: its layout thumbnail (which switches to it), name and tab count,
 * rename and close, and its tabs. Closing opens the in-place confirm; a desktop with tabs asks
 * where they go before the check is available (US-014.EC-1), unless its only tabs are blank New
 * Tabs. Escape anywhere on the card cancels a pending close. The last desktop has no close.
 */
export function DesktopOverviewCard({
  desktop,
  desktops,
  index,
  active,
  busy,
  sessionOf,
  dropTarget = false,
  dragKind = "tab",
  initialAction,
  onInitialActionApplied,
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
  const { renaming, setRenaming, closing, setClosing, closeKey, binRef, cancelCloseOnEscape } =
    useDesktopCardActions(initialAction, onInitialActionApplied);
  const needsYou = desktop.needsYou === true && !active;
  const needsDestination = desktop.movableTabCount > 0;

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
        className={desktopCardClassName(active, dropTarget, lifted)}
        data-current={active ? "true" : undefined}
        data-card-desktop-id={desktop.id}
        data-drop-target={dropTarget ? "true" : undefined}
        data-slot="desktop-card"
        onKeyDown={cancelCloseOnEscape}
        onPointerDown={onCardPointerDown}
      >
        <DesktopCardThumbnail
          active={active}
          busy={busy}
          desktop={desktop}
          needsYou={needsYou}
          thumbnailRef={thumbnailRef}
          onSwitch={onSwitch}
        />
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
          <DesktopCardNameRow
            binRef={binRef}
            busy={busy}
            closeKey={closeKey}
            closing={closing}
            desktop={desktop}
            desktopCount={desktops.length}
            initialAction={initialAction}
            needsDestination={needsDestination}
            needsYou={needsYou}
            setClosing={setClosing}
            onDelete={onDelete}
            onStartRename={() => {
              setClosing(null);
              setRenaming(true);
            }}
          />
        )}
        {closing !== null && needsDestination ? (
          <DesktopCardCloseDestination
            busy={busy}
            closing={closing}
            desktop={desktop}
            desktops={desktops}
            setClosing={setClosing}
          />
        ) : null}
        <DesktopCardTabList
          busy={busy}
          desktop={desktop}
          dragKind={dragKind}
          dropTarget={dropTarget}
          sessionOf={sessionOf}
          onChooseTab={onChooseTab}
          onTabPointerDown={onTabPointerDown}
        />
      </li>
    </DesktopOverviewCardMenu>
  );
}

function desktopCardClassName(active: boolean, dropTarget: boolean, lifted: boolean): string {
  return cn(
    "group/card relative flex min-w-0 flex-col gap-3 rounded-xl bg-card p-3 shadow-card transition-shadow duration-fast",
    active && "ring-2 ring-fg",
    dropTarget && "bg-brand-tint ring-2 ring-brand",
    lifted && "z-10 rotate-1 shadow-elevated"
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
