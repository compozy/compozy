import { ChevronDown, Plus, Scale } from "@compozy/ui/icons";
import { useEffect, useRef, useState } from "react";

import {
  Button,
  cn,
  CommandEmpty,
  CommandItem,
  CommandList,
  CommandSelect,
  CommandSelectGroup,
  CommandSelectShell,
  CommandSelectTrigger,
  CommandSeparator,
  Spinner,
  StatusDot,
} from "@compozy/ui";

import type { ComputerFixture, LocalComputerFixture } from "../mocks/remote-computers-fixtures";
import { ComputerLoadMeter } from "./computer-load-meter";
import { ComputerPlatformIcon } from "./computer-platform-icon";
import { RunOnComputerItem } from "./run-on-picker-item";

export type RunOnTarget = "local" | "auto" | (string & {});

export interface AutoResolutionSummary {
  computerId: string;
  computerName: string;
  /** Plain-language explanation of why Auto selected this computer. */
  reason: string;
}

export interface RunOnPickerProps {
  /** Local machine state; when `localRunAgents` is false, "This computer" is omitted. */
  localComputer: LocalComputerFixture;
  /** Enabled remote computers visible in the picker. */
  computers: readonly ComputerFixture[];
  /** Selected target: `"local"`, `"auto"`, or a computer `id`. */
  value: RunOnTarget;
  onChange: (next: RunOnTarget) => void;
  /** Whether load balancing is enabled (required for `Auto`). */
  loadBalancingEnabled?: boolean;
  /** Set to `false` in the Automation job editor where jobs belong to one concrete computer. */
  allowAuto?: boolean;
  /** Resolved target when `Auto` is available; `null` when no computer is eligible for the project. */
  autoResolution?: AutoResolutionSummary | null;
  /** Visual density matching the host surface. */
  variant?: "field" | "compact" | "inline";
  /** Static locked chip after a session is created — preserves exact height with zero layout jump. */
  locked?: boolean;
  disabled?: boolean;
  /** Hides the control completely when only This computer exists (`_uiux.md` U4). */
  hideWhenSingleComputer?: boolean;
  /** Opens the menu initially (for prototype states). */
  defaultOpen?: boolean;
  /** Forces the inline Reconnect pill visible on a specific offline computer (for review states). */
  highlightedReconnectId?: string;
  /** IDs of offline computers currently attempting an inline reconnect. */
  reconnectingIds?: readonly string[];
  onReconnect?: (computerId: string) => void;
  onUpgrade?: (computerId: string) => void;
  onAddComputer?: () => void;
  className?: string;
  triggerId?: string;
  testId?: string;
}

function triggerLabel(
  value: RunOnTarget,
  localComputer: LocalComputerFixture,
  computers: readonly ComputerFixture[],
  autoResolution: AutoResolutionSummary | null | undefined
): {
  kind: "auto" | "local" | "remote";
  name: string;
  suffix?: string;
  degraded?: "offline" | "connecting" | "warning" | "pro";
  computer?: ComputerFixture;
} {
  if (value === "auto") {
    if (autoResolution) {
      return { kind: "auto", name: "Auto", suffix: `→ ${autoResolution.computerName}` };
    }
    return { kind: "auto", name: "Auto", suffix: "No computer ready", degraded: "warning" };
  }
  if (value === "local") {
    return { kind: "local", name: "This computer" };
  }
  const found = computers.find(item => item.id === value);
  if (!found) {
    return {
      kind: "local",
      name: localComputer.localRunAgents ? "This computer" : "Choose computer",
    };
  }
  if (found.status === "offline") {
    return {
      kind: "remote",
      name: found.name,
      suffix: "Offline",
      degraded: "offline",
      computer: found,
    };
  }
  if (found.status === "connecting") {
    return {
      kind: "remote",
      name: found.name,
      suffix: "Connecting…",
      degraded: "connecting",
      computer: found,
    };
  }
  if (found.status === "needs_pro") {
    return {
      kind: "remote",
      name: found.name,
      suffix: "Needs Pro",
      degraded: "pro",
      computer: found,
    };
  }
  return { kind: "remote", name: found.name, computer: found };
}

/**
 * Pure, prop-driven `Run on` computer picker (`_uiux.md` U4).
 *
 * Placed immediately before the Project picker in Session Create, Kanban New Session,
 * and the Automation job editor (`allowAuto={false}`). Supports type-ahead filtering
 * while open, live `Auto → <computer>` preview, host load meters, inline `Reconnect`
 * on offline computers without closing the menu, and non-selectable `Needs Pro` rows.
 */
export function RunOnPicker({
  localComputer,
  computers,
  value,
  onChange,
  loadBalancingEnabled = false,
  allowAuto = true,
  autoResolution = null,
  variant = "field",
  locked = false,
  disabled = false,
  hideWhenSingleComputer = true,
  defaultOpen = false,
  highlightedReconnectId,
  reconnectingIds = [],
  onReconnect,
  onUpgrade,
  onAddComputer,
  className,
  triggerId,
  testId = "run-on-picker",
}: RunOnPickerProps) {
  const [open, setOpen] = useState(defaultOpen);
  const [query, setQuery] = useState("");
  const listRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!open) return;
    const frame = requestAnimationFrame(() => {
      const selectedItem = listRef.current?.querySelector<HTMLElement>('[data-checked="true"]');
      selectedItem?.scrollIntoView({ block: "nearest" });
    });
    return () => cancelAnimationFrame(frame);
  }, [open, value]);

  const enabledComputers = computers.filter(computer => computer.enabled);
  const showAuto = allowAuto && loadBalancingEnabled;

  if (hideWhenSingleComputer && enabledComputers.length === 0 && localComputer.localRunAgents) {
    return null;
  }

  const current = triggerLabel(value, localComputer, enabledComputers, autoResolution);
  const isCompact = variant === "compact";
  const isInline = variant === "inline";

  const triggerInner = (
    <span className="flex min-w-0 flex-1 items-center gap-1.5 text-left">
      {current.kind === "auto" ? (
        <Scale
          aria-hidden="true"
          className={cn(
            "size-3.5 shrink-0",
            current.degraded === "warning" ? "text-warning" : "text-subtle"
          )}
        />
      ) : current.kind === "local" ? (
        <ComputerPlatformIcon
          className="size-3.5 shrink-0"
          iconKind={localComputer.iconKind}
          platform={localComputer.platform}
          size="sm"
          tone="muted"
        />
      ) : (
        <ComputerPlatformIcon
          className="size-3.5 shrink-0"
          iconKind={current.computer?.iconKind}
          platform={current.computer?.platform}
          size="sm"
          tone="muted"
        />
      )}
      {isCompact || isInline ? (
        <span className="shrink-0 text-muted-foreground">Run on</span>
      ) : null}
      <span className="truncate font-medium text-fg">{current.name}</span>
      {current.suffix ? (
        <span
          className={cn(
            "inline-flex shrink-0 items-center gap-1 text-eyebrow",
            current.degraded === "warning"
              ? "text-warning"
              : current.degraded === "pro"
                ? "text-brand"
                : "text-muted-foreground"
          )}
        >
          {current.degraded === "connecting" ? (
            <Spinner aria-hidden="true" className="size-3" />
          ) : null}
          {current.kind !== "auto" ? <span aria-hidden="true">·</span> : null}
          <span>{current.suffix}</span>
        </span>
      ) : null}
    </span>
  );

  if (locked) {
    return (
      <span
        className={cn(
          "inline-flex items-center gap-1.5 rounded-md border border-line bg-surface-2/60 text-small-body text-fg",
          isCompact ? "h-7 px-2.5 text-eyebrow" : "h-input px-3",
          className
        )}
        data-locked="true"
        data-slot="run-on-picker"
        data-testid={testId}
      >
        {triggerInner}
      </span>
    );
  }

  const handleSelect = (next: RunOnTarget) => {
    onChange(next);
    setOpen(false);
    setQuery("");
  };

  return (
    <CommandSelect
      onOpenChange={next => {
        setOpen(next);
        if (!next) setQuery("");
      }}
      open={open}
    >
      <CommandSelectTrigger
        aria-expanded={open}
        aria-haspopup="listbox"
        aria-label={`Run on: ${current.name}${current.suffix ? ` ${current.suffix}` : ""}`}
        className={cn(
          isCompact &&
            "h-7 w-auto min-w-0 gap-1.5 rounded-md border-line bg-surface-2/70 px-2.5 py-0 text-eyebrow hover:bg-surface-2 [&>svg:last-child]:hidden",
          isInline &&
            "inline-flex h-auto w-auto min-w-0 gap-1 rounded-sm border-transparent bg-transparent px-1 py-0.5 text-form-hint shadow-none hover:bg-surface-2 [&>svg:last-child]:hidden",
          className
        )}
        data-slot="run-on-picker-trigger"
        data-testid={testId}
        disabled={disabled}
        id={triggerId}
        onKeyDown={event => {
          if (open || event.key.length !== 1 || event.metaKey || event.ctrlKey || event.altKey) {
            return;
          }
          setQuery(event.key);
          setOpen(true);
        }}
        selected
      >
        {triggerInner}
        {isCompact || isInline ? (
          <ChevronDown aria-hidden="true" className="size-3 shrink-0 text-subtle" />
        ) : null}
      </CommandSelectTrigger>

      <CommandSelectShell
        align="start"
        className={cn(
          "flex max-h-[min(28rem,calc(var(--available-height,24rem)-0.5rem))] w-80 max-w-[min(22rem,calc(100vw-2rem))] flex-col overflow-hidden border border-line-strong bg-popover p-0 shadow-overlay",
          defaultOpen && "opacity-100! animate-none! transition-none!"
        )}
        commandProps={{
          className: "flex max-h-full min-h-0 flex-1 flex-col overflow-hidden bg-popover",
        }}
        inputPlaceholder="Search computers…"
        inputProps={{
          value: query,
          onValueChange: setQuery,
          "data-testid": `${testId}-search`,
        }}
        side="bottom"
      >
        <CommandList
          className="min-h-0 max-h-[min(22.5rem,calc(var(--available-height,24rem)-5.5rem))] flex-1 overflow-y-auto bg-popover"
          ref={listRef}
        >
          <CommandEmpty>No matching computer.</CommandEmpty>

          {showAuto || localComputer.localRunAgents ? (
            <CommandSelectGroup heading="Default">
              {showAuto ? (
                <CommandItem
                  className="items-center gap-2.5 py-1.5"
                  data-checked={value === "auto" ? "true" : "false"}
                  data-testid={`${testId}-item-auto`}
                  onSelect={() => handleSelect("auto")}
                  value={`Auto load balancing ${autoResolution?.computerName ?? ""}`}
                >
                  <Scale
                    aria-hidden="true"
                    className={cn(
                      "size-4 shrink-0",
                      autoResolution ? "text-subtle" : "text-warning"
                    )}
                  />
                  <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span className="flex items-center gap-1.5 text-small-body font-medium text-fg">
                      <span>Auto</span>
                      {autoResolution ? (
                        <span className="font-normal text-muted-foreground">
                          → {autoResolution.computerName}
                        </span>
                      ) : null}
                    </span>
                    <span
                      className={cn(
                        "truncate text-eyebrow",
                        autoResolution ? "text-subtle" : "text-warning"
                      )}
                    >
                      {autoResolution
                        ? "Picks the least busy computer"
                        : "No computer can run this project"}
                    </span>
                  </span>
                </CommandItem>
              ) : null}

              {localComputer.localRunAgents ? (
                <CommandItem
                  className="items-center gap-2.5 py-1.5"
                  data-checked={value === "local" ? "true" : "false"}
                  data-testid={`${testId}-item-local`}
                  onSelect={() => handleSelect("local")}
                  value={`This computer ${localComputer.name} local`}
                >
                  <ComputerPlatformIcon
                    className="size-4 shrink-0"
                    iconKind={localComputer.iconKind}
                    platform={localComputer.platform}
                    size="sm"
                    tone="muted"
                  />
                  <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span className="truncate text-small-body font-medium text-fg">
                      This computer
                    </span>
                    <span className="flex items-center gap-1.5 truncate text-eyebrow text-subtle">
                      <StatusDot size="sm" tone="success" />
                      <span className="truncate">{localComputer.name}</span>
                    </span>
                  </span>
                  <ComputerLoadMeter computerName={localComputer.name} load={localComputer.load} />
                </CommandItem>
              ) : null}
            </CommandSelectGroup>
          ) : null}

          {enabledComputers.length > 0 ? (
            <>
              {showAuto || localComputer.localRunAgents ? <CommandSeparator /> : null}
              <CommandSelectGroup heading="Computers">
                {(() => {
                  const reconnectingSet = new Set(reconnectingIds);
                  return enabledComputers.map(computer => (
                    <RunOnComputerItem
                      computer={computer}
                      highlightReconnect={highlightedReconnectId === computer.id}
                      key={computer.id}
                      onReconnect={onReconnect}
                      onSelect={handleSelect}
                      onUpgrade={onUpgrade}
                      reconnecting={reconnectingSet.has(computer.id)}
                      selected={value === computer.id}
                      testIdPrefix={testId}
                    />
                  ));
                })()}
              </CommandSelectGroup>
            </>
          ) : null}
        </CommandList>

        {onAddComputer ? (
          <div className="shrink-0 border-t border-line-soft bg-popover p-1">
            <Button
              className="h-8 w-full justify-start gap-2 px-2.5 text-small-body font-normal text-muted-foreground hover:text-fg"
              data-testid={`${testId}-add-computer`}
              onClick={() => {
                setOpen(false);
                onAddComputer();
              }}
              size="sm"
              type="button"
              variant="quiet"
            >
              <Plus aria-hidden="true" className="size-3.5 shrink-0" />
              <span>Add computer…</span>
            </Button>
          </div>
        ) : null}
      </CommandSelectShell>
    </CommandSelect>
  );
}
