import { ChevronLeft, ChevronRight, Pencil, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import type * as React from "react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
  Input,
  Kbd,
  NativeSelect,
  NativeSelectOption,
  StatusDot,
  TopbarOverflowIcon,
  cn,
} from "@compozy/ui";

import type {
  DesktopOverviewItem,
  DesktopsOverviewProps,
  DesktopsOverviewState,
} from "./desktops-overview";

interface DesktopActionsProps {
  desktop: DesktopOverviewItem;
  desktops: readonly DesktopOverviewItem[];
  index: number;
  busy: boolean;
  onRename: () => void;
  onDelete: () => void;
  onReorder: (order: number) => void;
  onMoveWindow: DesktopsOverviewProps["onMoveWindow"];
}

/** One overflow menu per card: rename, reorder, move any of its windows, delete. */
function DesktopActions({
  desktop,
  desktops,
  index,
  busy,
  onRename,
  onDelete,
  onReorder,
  onMoveWindow,
}: DesktopActionsProps) {
  const count = desktops.length;
  const destinations = desktops.filter(candidate => candidate.id !== desktop.id);
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={`Actions for ${desktop.name}`}
        disabled={busy}
        // Quiet until the card is hovered or focused; always shown without hover.
        className={cn(
          "opacity-0 transition-opacity duration-fast group-focus-within/oc:opacity-100 group-hover/oc:opacity-100",
          "aria-expanded:opacity-100 [@media(hover:none)]:opacity-100"
        )}
        render={<Button type="button" variant="ghost" size="icon-sm" />}
      >
        <TopbarOverflowIcon aria-hidden="true" className="size-3" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onClick={onRename}>
          <Pencil aria-hidden="true" />
          Rename
        </DropdownMenuItem>
        <DropdownMenuItem disabled={index === 0} onClick={() => onReorder(index - 1)}>
          <ChevronLeft aria-hidden="true" />
          Move left
        </DropdownMenuItem>
        <DropdownMenuItem disabled={index === count - 1} onClick={() => onReorder(index + 1)}>
          <ChevronRight aria-hidden="true" />
          Move right
        </DropdownMenuItem>
        {desktop.windows.length > 0 && destinations.length > 0 ? (
          <DropdownMenuSub>
            <DropdownMenuSubTrigger>Move a window</DropdownMenuSubTrigger>
            <DropdownMenuSubContent>
              {desktop.windows.map(window => (
                <DropdownMenuSub key={window.id}>
                  <DropdownMenuSubTrigger>{window.title}</DropdownMenuSubTrigger>
                  <DropdownMenuSubContent>
                    <DropdownMenuGroup>
                      <DropdownMenuLabel>Move to</DropdownMenuLabel>
                      {destinations.map(option => (
                        <DropdownMenuItem
                          key={option.id}
                          onClick={() => onMoveWindow(window.id, desktop.id, option.id)}
                        >
                          {option.name}
                        </DropdownMenuItem>
                      ))}
                    </DropdownMenuGroup>
                  </DropdownMenuSubContent>
                </DropdownMenuSub>
              ))}
            </DropdownMenuSubContent>
          </DropdownMenuSub>
        ) : null}
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" disabled={count < 2} onClick={onDelete}>
          <Trash2 aria-hidden="true" />
          Delete…
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function RenameDesktop({
  desktop,
  busy,
  onCancel,
  onRename,
}: {
  desktop: DesktopOverviewItem;
  busy: boolean;
  onCancel: () => void;
  onRename: (name: string) => void;
}) {
  const [name, setName] = useState(desktop.name);
  const nextName = name.trim();
  const canSave = nextName.length > 0 && nextName !== desktop.name;

  return (
    <form
      className="flex flex-wrap items-center gap-2"
      onSubmit={event => {
        event.preventDefault();
        if (canSave) onRename(nextName);
      }}
    >
      <label className="sr-only" htmlFor={`desktop-name-${desktop.id}`}>
        Desktop name
      </label>
      <Input
        autoFocus
        className="h-11 min-w-32 flex-1"
        id={`desktop-name-${desktop.id}`}
        value={name}
        disabled={busy}
        onChange={event => setName(event.currentTarget.value)}
        onKeyDown={event => {
          if (event.key === "Escape") onCancel();
        }}
      />
      <Button type="submit" size="cta-lg" disabled={busy || !canSave}>
        Save name
      </Button>
      <Button type="button" size="cta-lg" variant="ghost" disabled={busy} onClick={onCancel}>
        Cancel
      </Button>
    </form>
  );
}

function DeleteDesktop({
  desktop,
  destinations,
  busy,
  onCancel,
  onDelete,
}: {
  desktop: DesktopOverviewItem;
  destinations: readonly DesktopOverviewItem[];
  busy: boolean;
  onCancel: () => void;
  onDelete: (destinationId: string | null) => void;
}) {
  const [destinationId, setDestinationId] = useState(destinations[0]?.id ?? "");
  const needsTransfer = desktop.windows.length > 0;
  const canDelete = !needsTransfer || destinationId.length > 0;

  return (
    <form
      aria-label={`Delete ${desktop.name}`}
      className="flex flex-col gap-3 rounded-md border border-line bg-danger-tint p-3"
      onSubmit={event => {
        event.preventDefault();
        if (canDelete) onDelete(needsTransfer ? destinationId : null);
      }}
    >
      <div className="flex flex-col gap-1">
        <p className="text-small-body font-medium text-fg-strong">Delete {desktop.name}?</p>
        <p className="text-form-hint text-muted">
          {needsTransfer
            ? `Move ${desktop.windows.length} window${desktop.windows.length === 1 ? "" : "s"} before deleting this desktop.`
            : "This desktop has no windows."}
        </p>
      </div>
      {needsTransfer ? (
        <label className="flex flex-col gap-1.5 text-form-label text-muted">
          Move windows to
          <NativeSelect
            className="w-full [&>select]:h-11"
            value={destinationId}
            disabled={busy}
            onChange={event => setDestinationId(event.currentTarget.value)}
          >
            {destinations.map(destination => (
              <NativeSelectOption key={destination.id} value={destination.id}>
                {destination.name}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </label>
      ) : null}
      <div className="flex justify-end gap-2">
        <Button type="button" size="cta-lg" variant="ghost" disabled={busy} onClick={onCancel}>
          Cancel
        </Button>
        <Button type="submit" size="cta-lg" variant="destructive" disabled={busy || !canDelete}>
          Delete desktop
        </Button>
      </div>
    </form>
  );
}

type DesktopsOverviewGridProps = Pick<
  DesktopsOverviewProps,
  | "onOpenChange"
  | "onCreateDesktop"
  | "onSwitchDesktop"
  | "onRenameDesktop"
  | "onReorderDesktop"
  | "onDeleteDesktop"
  | "onMoveWindow"
> & {
  state: Extract<DesktopsOverviewState, { status: "ready" }>;
  busy: boolean;
  initialFocusDesktopId: string | null;
  initialFocusRef: React.RefObject<HTMLButtonElement | null>;
};

const CARD_SELECTOR = '[data-slot="desktops-overview-card"]:not(:disabled)';
const DEFAULT_ASPECT = 16 / 10;

/** Arrows move across the card grid (Up/Down by one row), Home/End jump; Enter is native. */
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

const CARD_CLASS =
  "group/oc relative flex min-w-0 flex-col gap-2.5 rounded-lg p-2 pb-3 transition-colors duration-fast hover:bg-surface-2";
const CARD_BUTTON_CLASS =
  "flex w-full flex-col gap-2.5 rounded-md text-left outline-none focus-visible:shadow-focus-ring disabled:cursor-not-allowed";

/** Interactive ready-state grid used only after the authoritative snapshot resolves. */
export function DesktopsOverviewGrid({
  state,
  busy,
  initialFocusDesktopId,
  initialFocusRef,
  onOpenChange,
  onCreateDesktop,
  onSwitchDesktop,
  onRenameDesktop,
  onReorderDesktop,
  onDeleteDesktop,
  onMoveWindow,
}: DesktopsOverviewGridProps) {
  const [editingDesktopId, setEditingDesktopId] = useState<string | null>(null);
  const [deletingDesktopId, setDeletingDesktopId] = useState<string | null>(null);
  const aspectRatio = state.desktops[0]?.aspectRatio ?? DEFAULT_ASPECT;

  return (
    <ol
      aria-label="Desktops"
      className="grid grid-cols-[repeat(auto-fill,minmax(300px,1fr))] gap-5 pb-8"
      onKeyDown={moveCardFocus}
    >
      {state.desktops.map((desktop, index) => {
        const active = desktop.id === state.activeDesktopId;
        const destinations = state.desktops.filter(candidate => candidate.id !== desktop.id);
        const windowCount = desktop.windows.length;
        const needsYou = desktop.needsYou === true && !active;

        return (
          <li
            key={desktop.id}
            data-current={active ? "true" : undefined}
            aria-current={active ? "true" : undefined}
            className={CARD_CLASS}
          >
            <button
              ref={desktop.id === initialFocusDesktopId ? initialFocusRef : undefined}
              type="button"
              data-slot="desktops-overview-card"
              disabled={busy}
              aria-label={`${active ? "Current desktop" : "Switch to"} ${desktop.name}${needsYou ? " — needs you" : ""}`}
              className={CARD_BUTTON_CLASS}
              onClick={() => {
                if (!active) onSwitchDesktop(desktop.id);
                onOpenChange(false);
              }}
            >
              <span
                aria-hidden="true"
                className={cn(
                  "relative block w-full overflow-hidden rounded-lg bg-sunken shadow-card",
                  active && "shadow-none ring-2 ring-fg"
                )}
                style={{ aspectRatio: desktop.aspectRatio ?? aspectRatio }}
              >
                {desktop.thumbnail}
              </span>
              <span className="flex min-w-0 items-center gap-2 px-1 pr-9">
                <span className="truncate font-medium text-fg">{desktop.name}</span>
                <span className="shrink-0 text-meta text-muted">
                  {windowCount} window{windowCount === 1 ? "" : "s"}
                </span>
                {needsYou ? <StatusDot tone="accent" size="sm" label="Needs you" /> : null}
                {desktop.switchShortcut ? (
                  <Kbd className="ml-auto">{desktop.switchShortcut}</Kbd>
                ) : null}
              </span>
            </button>
            <div className="absolute right-2 bottom-2.5">
              <DesktopActions
                desktop={desktop}
                desktops={state.desktops}
                index={index}
                busy={busy}
                onReorder={order => onReorderDesktop(desktop.id, order)}
                onMoveWindow={onMoveWindow}
                onRename={() => {
                  setDeletingDesktopId(null);
                  setEditingDesktopId(desktop.id);
                }}
                onDelete={() => {
                  setEditingDesktopId(null);
                  setDeletingDesktopId(desktop.id);
                }}
              />
            </div>

            {editingDesktopId === desktop.id ? (
              <RenameDesktop
                desktop={desktop}
                busy={busy}
                onCancel={() => setEditingDesktopId(null)}
                onRename={name => {
                  onRenameDesktop(desktop.id, name);
                  setEditingDesktopId(null);
                }}
              />
            ) : null}

            {deletingDesktopId === desktop.id ? (
              <DeleteDesktop
                desktop={desktop}
                destinations={destinations}
                busy={busy}
                onCancel={() => setDeletingDesktopId(null)}
                onDelete={destinationId => {
                  onDeleteDesktop(desktop.id, destinationId);
                  setDeletingDesktopId(null);
                }}
              />
            ) : null}
          </li>
        );
      })}
      <li className={CARD_CLASS}>
        <button
          type="button"
          data-slot="desktops-overview-card"
          disabled={busy}
          className={CARD_BUTTON_CLASS}
          onClick={onCreateDesktop}
        >
          <span
            aria-hidden="true"
            className="grid w-full place-items-center rounded-lg text-muted ring-[1.5px] ring-line-strong ring-inset"
            style={{ aspectRatio }}
          >
            <Plus className="size-4" />
          </span>
          <span className="px-1 font-medium text-fg">New desktop</span>
        </button>
      </li>
    </ol>
  );
}
