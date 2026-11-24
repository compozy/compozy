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

const STATUS_TONES = {
  connected: "success",
  connecting: "faint",
  offline: "faint",
  needs_attention: "warning",
  needs_pro: "brand",
  incompatible: "warning",
  disabled: "faint",
} as const;

function ComputerItemDetails({
  computer,
  dimmed,
  reconnecting,
}: {
  computer: ComputerFixture;
  dimmed: boolean;
  reconnecting: boolean;
}) {
  return (
    <span className={cn("flex min-w-0 flex-1 flex-col gap-0.5", dimmed && "opacity-70")}>
      <span className="flex items-center gap-1.5 text-small-body font-medium text-fg">
        <span className="truncate">{computer.name}</span>
        {computer.status === "needs_attention" && (
          <TriangleAlert
            aria-label={computer.attentionReason?.summary ?? "Needs attention"}
            className="size-3 shrink-0 text-warning"
          />
        )}
      </span>
      <span className="flex items-center gap-1.5 truncate text-eyebrow text-subtle">
        {computer.status === "connecting" || reconnecting ? (
          <Spinner aria-hidden="true" className="size-3 shrink-0" />
        ) : (
          <StatusDot size="sm" tone={STATUS_TONES[computer.status]} />
        )}
        <span className="truncate">
          {reconnecting ? "Reconnecting…" : routeShortLabel(computer)}
        </span>
      </span>
    </span>
  );
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

function ComputerItemAction({
  computer,
  highlightReconnect,
  reconnecting,
  onReconnect,
  onUpgrade,
}: Pick<
  RunOnComputerItemProps,
  "computer" | "highlightReconnect" | "reconnecting" | "onReconnect" | "onUpgrade"
>) {
  if (computer.status === "connected" || computer.status === "needs_attention") {
    return <ComputerLoadMeter computerName={computer.name} load={computer.load} />;
  }
  if (computer.status === "offline" && onReconnect) {
    return (
      <Button
        className={cn(
          "h-6 px-2 text-badge transition-opacity",
          highlightReconnect || reconnecting
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
    );
  }
  if (computer.status !== "needs_pro") return null;
  return (
    <span className="pointer-events-auto flex items-center gap-1.5 opacity-100">
      <Pill form="tint" size="sm" tone="brand">
        Needs Pro
      </Pill>
      {onUpgrade && (
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
      )}
    </span>
  );
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
  const isBlocked = computer.status === "needs_pro" || computer.status === "incompatible";
  const dimmed = isBlocked || computer.status === "offline";

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
        className={cn("size-4 shrink-0", dimmed && "opacity-60")}
        iconKind={computer.iconKind}
        platform={computer.platform}
        size="sm"
        tone="muted"
      />

      <ComputerItemDetails computer={computer} dimmed={dimmed} reconnecting={reconnecting} />

      <span className="flex shrink-0 items-center gap-1.5">
        <ComputerItemAction
          computer={computer}
          highlightReconnect={highlightReconnect}
          onReconnect={onReconnect}
          onUpgrade={onUpgrade}
          reconnecting={reconnecting}
        />
      </span>
    </CommandItem>
  );
}
