import { ArrowUpRight, RefreshCw, Sparkles } from "@compozy/ui/icons";
import { useState } from "react";

import {
  Button,
  cn,
  Popover,
  PopoverContent,
  PopoverTrigger,
  Spinner,
  StateGlyph,
  StatusDot,
} from "@compozy/ui";

import {
  DISPLAYED_REMOTE_SCOPES,
  type ComputerFixture,
  type ComputerIconKind,
  type ComputerPlatform,
} from "../mocks/remote-computers-fixtures";
import { ComputerLoadMeter } from "./computer-load-meter";
import { ComputerPlatformIcon } from "./computer-platform-icon";
import { ComputerStatusChip } from "./computer-status-chip";

export interface ComputerMetaLabelProps {
  name: string;
  platform?: ComputerPlatform;
  iconKind?: ComputerIconKind;
  offline?: boolean;
  className?: string;
}

/**
 * Quiet muted provenance label placed on remote session rows, Kanban cards,
 * and cross-computer search results (`_uiux.md` U5). Local sessions omit this
 * label so remote sessions stand out cleanly without visual clutter.
 */
export function ComputerMetaLabel({
  name,
  platform,
  iconKind,
  offline = false,
  className,
}: ComputerMetaLabelProps) {
  return (
    <span
      className={cn(
        "inline-flex min-w-0 items-center gap-1 font-sans text-micro tracking-normal text-muted-foreground",
        offline && "text-faint",
        className
      )}
      data-offline={offline ? "true" : undefined}
      data-slot="computer-meta-label"
      title={offline ? `${name} (Offline)` : name}
    >
      <ComputerPlatformIcon
        className="size-3 shrink-0"
        iconKind={iconKind}
        platform={platform}
        size="sm"
        tone="muted"
      />
      <span className="min-w-0 truncate">{name}</span>
      {offline ? (
        <>
          <span aria-hidden="true" className="text-faint">
            ·
          </span>
          <span className="shrink-0 text-faint">Offline</span>
        </>
      ) : null}
    </span>
  );
}

const DEGRADED_LABELS: Partial<Record<ComputerFixture["status"], string>> = {
  offline: "Offline",
  connecting: "Connecting…",
  needs_pro: "Needs Pro",
  incompatible: "Update required",
};

export interface ComputerChipProps {
  computer: ComputerFixture;
  /** Opens the chip popover initially (for prototype states). */
  defaultOpen?: boolean;
  onOpenDetails?: () => void;
  onReconnect?: () => void;
  onUpgrade?: () => void;
  className?: string;
}

/**
 * Session window header computer chip (`_uiux.md` U6).
 *
 * Calm neutral chrome when healthy (`[icon] devbox`), appending a truthful
 * degraded suffix (`· Offline`, `· Connecting…`, `· Needs Pro`) only when
 * unhealthy. Clicking opens a compact status popover showing active route,
 * host capacity, granted device scopes, and jump to Computer details.
 */
export function ComputerChip({
  computer,
  defaultOpen = false,
  onOpenDetails,
  onReconnect,
  onUpgrade,
  className,
}: ComputerChipProps) {
  const [open, setOpen] = useState(defaultOpen);
  const isOffline = computer.status === "offline";
  const isConnecting = computer.status === "connecting";
  const isNeedsPro = computer.status === "needs_pro";
  const isWarning = computer.status === "needs_attention" || computer.status === "incompatible";

  const degradedSuffix = DEGRADED_LABELS[computer.status];

  return (
    <Popover onOpenChange={setOpen} open={open}>
      <PopoverTrigger
        aria-label={`Session runs on ${computer.name}${degradedSuffix ? `, ${degradedSuffix}` : ""}`}
        className={cn(
          "inline-flex h-6.5 max-w-48 shrink-0 cursor-pointer items-center gap-1.5 rounded-md border border-line bg-surface-2/60 px-2 text-eyebrow text-fg-2 transition-colors duration-base hover:border-line-strong hover:bg-surface-2 hover:text-fg focus-visible:shadow-focus-ring focus-visible:outline-none",
          isOffline && "text-muted-foreground",
          isNeedsPro && "border-brand/30 bg-brand-tint text-brand",
          className
        )}
        data-computer-id={computer.id}
        data-computer-status={computer.status}
        data-slot="computer-chip"
        data-testid={`session-computer-chip-${computer.id}`}
        type="button"
      >
        <ComputerPlatformIcon
          className="size-3.5 shrink-0"
          iconKind={computer.iconKind}
          platform={computer.platform}
          size="sm"
          tone="muted"
        />
        <span className="truncate font-medium">{computer.name}</span>
        {degradedSuffix ? (
          <span className="inline-flex shrink-0 items-center gap-1 text-badge text-muted-foreground">
            <span aria-hidden="true">·</span>
            {isConnecting ? <Spinner aria-hidden="true" className="size-2.5" /> : null}
            <span className={cn(isNeedsPro && "text-brand", isWarning && "text-warning")}>
              {degradedSuffix}
            </span>
          </span>
        ) : (
          <StatusDot size="sm" tone={isWarning ? "warning" : "success"} />
        )}
      </PopoverTrigger>

      <ComputerChipDetails
        computer={computer}
        onOpenDetails={onOpenDetails}
        onReconnect={onReconnect}
        onUpgrade={onUpgrade}
      />
    </Popover>
  );
}

function ComputerChipDetails({
  computer,
  onOpenDetails,
  onReconnect,
  onUpgrade,
}: Pick<ComputerChipProps, "computer" | "onOpenDetails" | "onReconnect" | "onUpgrade">) {
  const isOffline = computer.status === "offline";
  const isNeedsPro = computer.status === "needs_pro";
  return (
    <PopoverContent align="start" className="w-80 p-3.5" sideOffset={6}>
      <div className="flex flex-col gap-3">
        <div className="flex items-start justify-between gap-2">
          <div className="flex min-w-0 items-center gap-2">
            <ComputerPlatformIcon
              iconKind={computer.iconKind}
              platform={computer.platform}
              size="md"
              tone="muted"
            />
            <div className="min-w-0">
              <p className="truncate text-small-body font-medium text-fg">{computer.name}</p>
              <p className="truncate text-eyebrow text-subtle">
                {computer.routeInUse?.label ??
                  (isOffline
                    ? `Last connected ${computer.lastConnectedLabel}`
                    : (computer.routes[0]?.label ?? "No active route"))}
              </p>
            </div>
          </div>
          <ComputerStatusChip status={computer.status} />
        </div>

        <div className="flex items-center justify-between border-t border-line-soft pt-2.5 text-eyebrow">
          <span className="text-subtle">Host capacity</span>
          <ComputerLoadMeter computerName={computer.name} load={computer.load} />
        </div>

        <div className="flex flex-col gap-1.5 border-t border-line-soft pt-2.5">
          <div className="flex items-center justify-between text-eyebrow">
            <span className="text-subtle">Permissions on this device</span>
            <span className="text-badge text-faint tabular-nums">
              {DISPLAYED_REMOTE_SCOPES.filter(s => computer.scopes.includes(s.scope)).length} of{" "}
              {DISPLAYED_REMOTE_SCOPES.length}
            </span>
          </div>
          <ul className="grid grid-cols-2 gap-x-3 gap-y-1">
            {DISPLAYED_REMOTE_SCOPES.map(item => {
              const granted = computer.scopes.includes(item.scope);
              return (
                <li
                  className={cn(
                    "flex items-center gap-1.5 text-eyebrow",
                    granted ? "text-fg-2" : "text-faint"
                  )}
                  key={item.scope}
                >
                  <StateGlyph size="sm" state={granted ? "done" : "stopped"} />
                  <span className="truncate">{item.label}</span>
                </li>
              );
            })}
          </ul>
          {!computer.scopes.includes("terminal:operate") ? (
            <p className="pt-0.5 text-badge text-subtle">
              Terminals are hidden while this device lacks terminal permission on {computer.name}.
            </p>
          ) : null}
        </div>

        <div className="flex items-center justify-end gap-2 border-t border-line-soft pt-2.5">
          {isOffline && onReconnect ? (
            <Button onClick={onReconnect} size="xs" type="button" variant="secondary">
              <RefreshCw aria-hidden="true" data-icon="inline-start" />
              Reconnect
            </Button>
          ) : null}
          {isNeedsPro && onUpgrade ? (
            <Button onClick={onUpgrade} size="xs" type="button" variant="default">
              <Sparkles aria-hidden="true" data-icon="inline-start" />
              Upgrade
            </Button>
          ) : null}
          {onOpenDetails ? (
            <Button onClick={onOpenDetails} size="xs" type="button" variant="quiet">
              Computer details
              <ArrowUpRight aria-hidden="true" data-icon="inline-end" />
            </Button>
          ) : null}
        </div>
      </div>
    </PopoverContent>
  );
}
