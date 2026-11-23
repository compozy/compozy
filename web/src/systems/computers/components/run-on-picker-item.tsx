import { RefreshCw, Sparkles, TriangleAlert } from "@compozy/ui/icons";
import type { MouseEvent } from "react";

import { Button, cn, CommandItem, Pill, Spinner, StatusDot } from "@compozy/ui";

import type { ComputerFixture } from "../mocks/remote-computers-fixtures";
import { ComputerLoadMeter } from "./computer-load-meter";
import { ComputerPlatformIcon } from "./computer-platform-icon";

function routeShortLabel(computer: ComputerFixture): string {
  if (computer.status === "connecting") return "Connecting…";
  if (computer.status === "offline") return `Offline · last seen ${computer.lastConnectedLabel}`;
  if (computer.status === "incompatible") return `Incompatible · ${computer.version}`;
  if (computer.status === "needs_pro") return "Needs Pro plan";
  if (computer.status === "needs_attention") return "Needs attention";
  return computer.routeInUse?.label ?? "Connected";
}

function handleInlineAction(event: MouseEvent<HTMLButtonElement>, callback?: () => void) {
  event.preventDefault();
  event.stopPropagation();
  callback?.();
}

export interface RunOnComputerItemProps {
  computer: ComputerFixture;
  selected: boolean;
  reconnecting: boolean;
  highlightReconnect: boolean;
  testIdPrefix: string;
  onSelect: (computerId: string) => void;
  onReconnect?: (computerId: string) => void;
  onUpgrade?: (computerId: string) => void;
}

/** One remote computer row inside `RunOnPicker`: status, load meter, inline Reconnect or Upgrade. */
export function RunOnComputerItem({
  computer,
  selected,
  reconnecting,
  highlightReconnect,
  testIdPrefix,
  onSelect,
  onReconnect,
  onUpgrade,
}: RunOnComputerItemProps) {
  const isOffline = computer.status === "offline";
  const isNeedsPro = computer.status === "needs_pro";
  const isIncompatible = computer.status === "incompatible";
  const isConnecting = computer.status === "connecting";
  const isAttention = computer.status === "needs_attention";
  const isBlocked = isNeedsPro || isIncompatible;
  const showReconnectAlways = isOffline && (highlightReconnect || reconnecting);

  return (
    <CommandItem
      className="group/run-on-item items-center gap-2.5 py-1.5"
      data-checked={selected ? "true" : "false"}
      data-computer-status={computer.status}
      data-testid={`${testIdPrefix}-item-${computer.id}`}
      disabled={isBlocked}
      onSelect={() => {
        if (isBlocked) return;
        onSelect(computer.id);
      }}
      value={`${computer.name} ${computer.status} ${computer.routeInUse?.label ?? ""}`}
    >
      <ComputerPlatformIcon
        className={cn("size-4 shrink-0", (isOffline || isBlocked) && "opacity-60")}
        iconKind={computer.iconKind}
        platform={computer.platform}
        size="sm"
        tone="muted"
      />

      <span
        className={cn(
          "flex min-w-0 flex-1 flex-col gap-0.5",
          (isOffline || isBlocked) && "opacity-70"
        )}
      >
        <span className="flex items-center gap-1.5 text-small-body font-medium text-fg">
          <span className="truncate">{computer.name}</span>
          {isAttention ? (
            <TriangleAlert
              aria-label={computer.attentionReason?.summary ?? "Needs attention"}
              className="size-3 shrink-0 text-warning"
            />
          ) : null}
        </span>
        <span className="flex items-center gap-1.5 truncate text-eyebrow text-subtle">
          {isConnecting || reconnecting ? (
            <Spinner aria-hidden="true" className="size-3 shrink-0" />
          ) : (
            <StatusDot
              size="sm"
              tone={
                computer.status === "connected"
                  ? "success"
                  : isAttention || isIncompatible
                    ? "warning"
                    : isNeedsPro
                      ? "brand"
                      : "faint"
              }
            />
          )}
          <span className="truncate">
            {reconnecting ? "Reconnecting…" : routeShortLabel(computer)}
          </span>
        </span>
      </span>

      <span className="flex shrink-0 items-center gap-1.5">
        {computer.status === "connected" || isAttention ? (
          <ComputerLoadMeter computerName={computer.name} load={computer.load} />
        ) : null}

        {isOffline && onReconnect ? (
          <Button
            className={cn(
              "h-6 px-2 text-badge transition-opacity",
              showReconnectAlways
                ? "opacity-100"
                : "opacity-0 group-hover/run-on-item:opacity-100 group-data-[selected=true]/run-on-item:opacity-100 focus-visible:opacity-100"
            )}
            disabled={reconnecting}
            onClick={event => handleInlineAction(event, () => onReconnect(computer.id))}
            size="xs"
            type="button"
            variant="secondary"
          >
            {reconnecting ? (
              <Spinner aria-hidden="true" className="size-3" />
            ) : (
              <RefreshCw aria-hidden="true" className="size-3" />
            )}
            {reconnecting ? "Reconnecting" : "Reconnect"}
          </Button>
        ) : null}

        {isNeedsPro ? (
          <span className="pointer-events-auto flex items-center gap-1.5 opacity-100">
            <Pill form="tint" size="sm" tone="brand">
              Needs Pro
            </Pill>
            {onUpgrade ? (
              <Button
                className="h-6 px-2 text-badge"
                onClick={event => handleInlineAction(event, () => onUpgrade(computer.id))}
                size="xs"
                type="button"
                variant="secondary"
              >
                <Sparkles aria-hidden="true" className="size-3" />
                Upgrade
              </Button>
            ) : null}
          </span>
        ) : null}
      </span>
    </CommandItem>
  );
}
