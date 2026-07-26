import { ChevronLeft, ChevronRight, Pencil, Trash2 } from "lucide-react";
import { useState } from "react";
import type * as React from "react";

import {
  Button,
  Input,
  NativeSelect,
  NativeSelectOption,
  Pill,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  cn,
} from "@agh/ui";

import type {
  DesktopOverviewItem,
  DesktopOverviewWindow,
  DesktopsOverviewProps,
  DesktopsOverviewState,
} from "./desktops-overview";

interface IconActionProps {
  label: string;
  disabled?: boolean;
  destructive?: boolean;
  onClick: () => void;
  children: React.ReactNode;
}

function IconAction({ label, disabled, destructive = false, onClick, children }: IconActionProps) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            type="button"
            size="icon-sm"
            className="size-11"
            variant={destructive ? "destructive" : "ghost"}
            aria-label={label}
            disabled={disabled}
            onClick={onClick}
          />
        }
      >
        {children}
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
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
    <li className="flex min-w-0 items-center gap-2 border-t border-line-soft py-2 first:border-t-0">
      <span className="min-w-0 flex-1">
        <span className="block truncate text-form-label font-medium text-fg">{window.title}</span>
        {window.detail ? (
          <span className="block truncate text-form-hint text-subtle">{window.detail}</span>
        ) : null}
      </span>
      <NativeSelect
        size="sm"
        className="[&>select]:h-11"
        value={desktop.id}
        disabled={busy || desktops.length < 2}
        aria-label={`Move ${window.title} to another desktop`}
        onChange={event => {
          const destinationId = event.currentTarget.value;
          if (destinationId !== desktop.id) {
            onMoveWindow(window.id, desktop.id, destinationId);
          }
        }}
      >
        {desktops.map(option => (
          <NativeSelectOption key={option.id} value={option.id}>
            {option.name}
          </NativeSelectOption>
        ))}
      </NativeSelect>
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
            className={cn(
              "flex min-w-0 flex-col gap-3 rounded-lg border border-line bg-canvas-soft p-3",
              active && "bg-row-selected shadow-inset-strong"
            )}
          >
            <div className="flex min-w-0 items-start gap-2">
              <div className="min-w-0 flex-1">
                <div className="flex min-w-0 items-center gap-2">
                  <h3 className="truncate text-item-title font-semibold text-fg-strong">
                    {desktop.name}
                  </h3>
                  {active ? (
                    <Pill size="xs" tone="neutral">
                      Current
                    </Pill>
                  ) : null}
                  {desktop.purpose === "focus" ? (
                    <Pill size="xs" tone="neutral">
                      Focus
                    </Pill>
                  ) : null}
                </div>
                <p className="mt-0.5 text-form-hint text-subtle">
                  {windowCount} window{windowCount === 1 ? "" : "s"}
                </p>
              </div>
              <div className="flex shrink-0 items-center">
                <IconAction
                  label={`Move ${desktop.name} left`}
                  disabled={busy || index === 0}
                  onClick={() => onReorderDesktop(desktop.id, index - 1)}
                >
                  <ChevronLeft aria-hidden="true" />
                </IconAction>
                <IconAction
                  label={`Move ${desktop.name} right`}
                  disabled={busy || index === state.desktops.length - 1}
                  onClick={() => onReorderDesktop(desktop.id, index + 1)}
                >
                  <ChevronRight aria-hidden="true" />
                </IconAction>
                <IconAction
                  label={`Rename ${desktop.name}`}
                  disabled={busy}
                  onClick={() => {
                    setDeletingDesktopId(null);
                    setEditingDesktopId(desktop.id);
                  }}
                >
                  <Pencil aria-hidden="true" />
                </IconAction>
                <IconAction
                  label={`Delete ${desktop.name}`}
                  destructive
                  disabled={busy || state.desktops.length < 2}
                  onClick={() => {
                    setEditingDesktopId(null);
                    setDeletingDesktopId(desktop.id);
                  }}
                >
                  <Trash2 aria-hidden="true" />
                </IconAction>
              </div>
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
