import { ChevronDown, Monitor, Settings2 } from "@compozy/ui/icons";
import { useState } from "react";

import {
  Button,
  CommandEmpty,
  CommandItem,
  CommandList,
  CommandSelect,
  CommandSelectGroup,
  CommandSelectShell,
  CommandSelectTrigger,
  CommandSeparator,
  StatusDot,
  cn,
} from "@compozy/ui";

import { LOCAL_COMPUTER_ID } from "@/lib/computer-scope";

import type { ComputerFixture, LocalComputerFixture } from "../mocks/remote-computers-types";
import { ComputerPlatformIcon } from "./computer-platform-icon";
import { RunOnComputerItem } from "./run-on-picker-item";

export type ComputerScope = "all" | "local" | (string & {});

export interface ComputerScopeCount {
  /** Facet session count, or `null` when unknown (`S11`). */
  count: number | null;
  needsYou: number;
}

export interface ComputerScopePickerProps {
  local: LocalComputerFixture;
  computers: readonly ComputerFixture[];
  /** Facet counts by computer id (`"local"` for this computer); never derived from loaded rows. */
  countsByComputer: ReadonlyMap<string, ComputerScopeCount>;
  value: ComputerScope;
  onChange: (next: ComputerScope) => void;
  onReconnect?: (computerId: string) => void;
  onUpgrade?: (computerId: string) => void;
  onManageComputers?: () => void;
  reconnectingIds?: readonly string[];
  defaultOpen?: boolean;
  testIdPrefix?: string;
}

/** How many sessions a computer holds from facets, plus an attention dot when any need you. */
function ScopeSessionCount({ entry }: { entry: ComputerScopeCount | undefined }) {
  if (!entry || entry.count === null) return null;
  return (
    <span className="flex shrink-0 items-center gap-1.5">
      {entry.needsYou > 0 ? (
        <StatusDot
          label={`${entry.needsYou} need${entry.needsYou === 1 ? "s" : ""} you`}
          size="sm"
          tone="attn"
        />
      ) : null}
      <span className="font-mono text-micro text-faint tabular-nums">{entry.count}</span>
    </span>
  );
}

function aggregateCount(
  countsByComputer: ReadonlyMap<string, ComputerScopeCount>
): ComputerScopeCount {
  let answered = false;
  let count = 0;
  let needsYou = 0;
  for (const entry of countsByComputer.values()) {
    if (entry.count === null) continue;
    answered = true;
    count += entry.count;
    needsYou += entry.needsYou;
  }
  return { count: answered ? count : null, needsYou };
}

function TriggerLabel({
  value,
  local,
  computers,
}: Pick<ComputerScopePickerProps, "value" | "local" | "computers">) {
  if (value === "all") {
    return (
      <>
        <Monitor aria-hidden="true" className="size-3.5 shrink-0 text-subtle" />
        <span className="truncate font-medium text-fg">All computers</span>
      </>
    );
  }
  const remote = computers.find(computer => computer.id === value);
  const source = remote ?? local;
  const offline = remote?.status === "offline";
  return (
    <>
      <ComputerPlatformIcon
        className="size-3.5 shrink-0"
        iconKind={source.iconKind}
        platform={source.platform}
        size="sm"
        tone="muted"
      />
      <span className="truncate font-medium text-fg">{remote ? remote.name : "This computer"}</span>
      {offline ? <span className="shrink-0 text-muted-foreground">· Offline</span> : null}
    </>
  );
}

/**
 * Per-window computer scope picker beside List | Kanban (`F8`, `C6`, `N6`, `R21`).
 * Hidden when no remote computer is enabled. Reads facet counts as props — no session rows.
 */
export function ComputerScopePicker({
  local,
  computers,
  countsByComputer,
  value,
  onChange,
  onReconnect,
  onUpgrade,
  onManageComputers,
  reconnectingIds,
  defaultOpen = false,
  testIdPrefix = "computer-scope",
}: ComputerScopePickerProps) {
  const [open, setOpen] = useState(defaultOpen);
  const [query, setQuery] = useState("");
  // One computer has nothing to choose between: the control stays out of the way (`N6`).
  if (computers.length === 0) return null;

  const reconnectingSet = new Set(reconnectingIds);
  const allCount = aggregateCount(countsByComputer);
  const activeName =
    value === "all"
      ? "All computers"
      : value === LOCAL_COMPUTER_ID
        ? "This computer"
        : (computers.find(computer => computer.id === value)?.name ?? value);

  const select = (next: ComputerScope) => {
    onChange(next);
    setOpen(false);
    setQuery("");
  };

  return (
    <CommandSelect onOpenChange={setOpen} open={open}>
      <CommandSelectTrigger
        aria-label={`Show sessions from: ${activeName}`}
        className={cn(
          "h-7 w-auto max-w-56 min-w-0 gap-1.5 rounded-md border-line bg-surface-2/70 px-2.5 py-0 text-eyebrow hover:bg-surface-2 [&>svg:last-child]:hidden",
          value !== "all" && "border-brand-dim"
        )}
        data-testid={`${testIdPrefix}-trigger`}
        selected
      >
        <span className="flex min-w-0 items-center gap-1.5">
          <TriggerLabel computers={computers} local={local} value={value} />
        </span>
        <ChevronDown aria-hidden="true" className="size-3 shrink-0 text-subtle" />
      </CommandSelectTrigger>
      <CommandSelectShell
        align="start"
        className={cn(
          "flex max-h-[min(30rem,calc(var(--available-height,24rem)-0.5rem))] w-80 flex-col overflow-hidden border border-line-strong bg-popover p-0 shadow-overlay",
          defaultOpen && "opacity-100! animate-none! transition-none!"
        )}
        commandProps={{
          className: "flex max-h-full min-h-0 flex-1 flex-col overflow-hidden bg-popover",
        }}
        inputPlaceholder="Search computers…"
        inputProps={{
          value: query,
          onValueChange: setQuery,
          "data-testid": `${testIdPrefix}-search`,
        }}
        side="bottom"
      >
        <CommandList className="min-h-0 flex-1 overflow-y-auto bg-popover">
          <CommandEmpty>No matching computer.</CommandEmpty>
          <CommandSelectGroup>
            <CommandItem
              className="items-center gap-2.5 py-1.5"
              data-checked={value === "all" ? "true" : "false"}
              data-testid={`${testIdPrefix}-item-all`}
              onSelect={() => select("all")}
              value="All computers"
            >
              <Monitor aria-hidden="true" className="size-4 shrink-0 text-subtle" />
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                <span className="truncate text-small-body font-medium text-fg">All computers</span>
                <span className="truncate text-eyebrow text-subtle">
                  {computers.length + 1} computers
                </span>
              </span>
              <ScopeSessionCount entry={allCount} />
            </CommandItem>
            <CommandItem
              className="items-center gap-2.5 py-1.5"
              data-checked={value === LOCAL_COMPUTER_ID ? "true" : "false"}
              data-testid={`${testIdPrefix}-item-local`}
              onSelect={() => select(LOCAL_COMPUTER_ID)}
              value={`This computer ${local.name}`}
            >
              <ComputerPlatformIcon
                className="size-4 shrink-0"
                iconKind={local.iconKind}
                platform={local.platform}
                size="sm"
                tone="muted"
              />
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                <span className="truncate text-small-body font-medium text-fg">This computer</span>
                <span className="flex items-center gap-1.5 truncate text-eyebrow text-subtle">
                  <StatusDot size="sm" tone="success" />
                  <span className="truncate">{local.name}</span>
                </span>
              </span>
              <ScopeSessionCount entry={countsByComputer.get(LOCAL_COMPUTER_ID)} />
            </CommandItem>
          </CommandSelectGroup>
          <CommandSeparator />
          <CommandSelectGroup heading="Computers">
            {computers.map(computer => (
              <RunOnComputerItem
                computer={computer}
                highlightReconnect={false}
                key={computer.id}
                onReconnect={onReconnect}
                onSelect={select}
                onUpgrade={onUpgrade}
                reconnecting={reconnectingSet.has(computer.id)}
                selected={value === computer.id}
                testIdPrefix={testIdPrefix}
                trailing={<ScopeSessionCount entry={countsByComputer.get(computer.id)} />}
              />
            ))}
          </CommandSelectGroup>
        </CommandList>
        {onManageComputers ? (
          <div className="shrink-0 border-t border-line-soft bg-popover p-1">
            <Button
              className="h-8 w-full justify-start gap-2 px-2.5 text-small-body font-normal text-muted-foreground hover:text-fg"
              data-testid={`${testIdPrefix}-manage`}
              onClick={() => {
                setOpen(false);
                onManageComputers();
              }}
              size="sm"
              type="button"
              variant="quiet"
            >
              <Settings2 aria-hidden="true" className="size-3.5 shrink-0" />
              <span>Manage computers…</span>
            </Button>
          </div>
        ) : null}
      </CommandSelectShell>
    </CommandSelect>
  );
}
