import { ChevronLeft, ChevronRight, Pencil, Trash2 } from "lucide-react";
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
  DropdownMenuTrigger,
  Input,
  NativeSelect,
  NativeSelectOption,
  TopbarOverflowIcon,
  cn,
} from "@compozy/ui";

import type {
  DesktopOverviewItem,
  DesktopOverviewWindow,
  DesktopsOverviewProps,
  DesktopsOverviewState,
} from "./desktops-overview";

interface DesktopActionsProps {
  desktop: DesktopOverviewItem;
  index: number;
  count: number;
  busy: boolean;
  onRename: () => void;
  onDelete: () => void;
  onReorder: (order: number) => void;
}

/** One overflow menu per card instead of four always-visible icon buttons. */
function DesktopActions({
  desktop,
  index,
  count,
  busy,
  onRename,
  onDelete,
  onReorder,
}: DesktopActionsProps) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={`Actions for ${desktop.name}`}
        disabled={busy}
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

function WindowRow({
  window,
  desktop,
  desktops,
  busy,
  onMoveWindow,
}: {
  window: DesktopOverviewWindow;
  desktop: DesktopOverviewItem;
  desktops: readonly DesktopOverviewItem[];
  busy: boolean;
  onMoveWindow: DesktopsOverviewProps["onMoveWindow"];
}) {
  return (
    <li className="group/desk-window flex min-w-0 items-center gap-2 border-t border-line-soft py-2 first:border-t-0">
      <span className="min-w-0 flex-1">
        <span className="block truncate text-form-label font-medium text-fg">{window.title}</span>
        {window.detail ? (
          <span className="block truncate text-form-hint text-subtle">{window.detail}</span>
        ) : null}
      </span>
      {desktops.length > 1 ? (
        <DropdownMenu>
          <DropdownMenuTrigger
            aria-label={`Move ${window.title} to another desktop`}
            disabled={busy}
            className={cn(
              "opacity-0 transition-opacity duration-base group-hover/desk-window:opacity-100",
              "group-focus-within/desk-window:opacity-100 aria-expanded:opacity-100 disabled:opacity-0"
            )}
            render={<Button type="button" variant="ghost" size="icon-sm" />}
          >
            <TopbarOverflowIcon aria-hidden="true" className="size-3" />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuGroup>
              <DropdownMenuLabel>Move to</DropdownMenuLabel>
              {desktops.map(option =>
                option.id === desktop.id ? null : (
                  <DropdownMenuItem
                    key={option.id}
                    onClick={() => onMoveWindow(window.id, desktop.id, option.id)}
                  >
                    {option.name}
                  </DropdownMenuItem>
                )
              )}
            </DropdownMenuGroup>
          </DropdownMenuContent>
        </DropdownMenu>
      ) : null}
    </li>
  );
}

type DesktopsOverviewGridProps = Pick<
  DesktopsOverviewProps,
  | "onOpenChange"
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

/** Interactive ready-state grid used only after the authoritative snapshot resolves. */
export function DesktopsOverviewGrid({
  state,
  busy,
  initialFocusDesktopId,
  initialFocusRef,
  onOpenChange,
  onSwitchDesktop,
  onRenameDesktop,
  onReorderDesktop,
  onDeleteDesktop,
  onMoveWindow,
}: DesktopsOverviewGridProps) {
  const [editingDesktopId, setEditingDesktopId] = useState<string | null>(null);
  const [deletingDesktopId, setDeletingDesktopId] = useState<string | null>(null);

  return (
    <ol className="grid grid-cols-1 gap-4 pb-8 md:grid-cols-2 xl:grid-cols-3">
      {state.desktops.map((desktop, index) => {
        const active = desktop.id === state.activeDesktopId;
        const destinations = state.desktops.filter(candidate => candidate.id !== desktop.id);
        const windowCount = desktop.windows.length;

        return (
          <li
            key={desktop.id}
            data-current={active ? "true" : undefined}
            aria-current={active ? "true" : undefined}
            className={cn(
              "flex min-w-0 flex-col gap-3 rounded-lg border border-line bg-canvas-soft p-3",
              active && "bg-row-selected"
            )}
          >
            <div className="flex min-w-0 items-start gap-2">
              <div className="min-w-0 flex-1">
                <h3 className="truncate text-item-title font-semibold text-fg-strong">
                  {desktop.name}
                </h3>
                <p className="mt-0.5 text-form-hint text-subtle">
                  {windowCount} window{windowCount === 1 ? "" : "s"}
                </p>
              </div>
              <DesktopActions
                desktop={desktop}
                index={index}
                count={state.desktops.length}
                busy={busy}
                onReorder={order => onReorderDesktop(desktop.id, order)}
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

            <Button
              ref={desktop.id === initialFocusDesktopId ? initialFocusRef : undefined}
              type="button"
              variant="ghost"
              disabled={busy}
              aria-label={`${active ? "Current desktop" : "Switch to"} ${desktop.name}`}
              className={cn(
                "h-auto w-full flex-col items-stretch gap-2 rounded-md border border-line-soft bg-canvas-tint p-2 text-left whitespace-normal",
                "tracking-normal hover:bg-row-hover focus-visible:shadow-focus-ring"
              )}
              onClick={() => {
                if (!active) onSwitchDesktop(desktop.id);
                onOpenChange(false);
              }}
            >
              <span
                aria-hidden="true"
                className="flex h-workspace-thumb w-full items-stretch overflow-hidden rounded-sm bg-canvas"
              >
                {desktop.thumbnail}
              </span>
            </Button>

            <ul aria-label={`Windows on ${desktop.name}`} className="max-h-36 overflow-y-auto">
              {desktop.windows.map(window => (
                <WindowRow
                  key={window.id}
                  window={window}
                  desktop={desktop}
                  desktops={state.desktops}
                  busy={busy}
                  onMoveWindow={onMoveWindow}
                />
              ))}
              {desktop.windows.length === 0 ? (
                <li className="py-2 text-form-hint text-subtle">No windows</li>
              ) : null}
            </ul>

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
    </ol>
  );
}
