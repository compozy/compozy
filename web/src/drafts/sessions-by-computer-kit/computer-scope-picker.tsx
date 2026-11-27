/**
 * The window's computer scope, beside List | Kanban: every computer, or one of them. It reads
 * like Run on (same items, status and route words) but filters what the window shows.
 */
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

import { ComputerPlatformIcon } from "@/systems/computers/components/computer-platform-icon";
import { RunOnComputerItem } from "@/systems/computers/components/run-on-picker-item";
import type {
  ComputerFixture,
  LocalComputerFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import { sessionBadgeSignal } from "@/systems/session/lib/session-badge";
import type { SessionPayload } from "@/systems/session/types";

export type ComputerScope = "all" | "local" | (string & {});

export interface ComputerScopePickerProps {
  local: LocalComputerFixture;
  computers: readonly ComputerFixture[];
  /** Sessions on each computer, by computer id (`local` for this computer). */
  sessionsByComputer: ReadonlyMap<string, readonly SessionPayload[]>;
  value: ComputerScope;
  onChange: (next: ComputerScope) => void;
  defaultOpen?: boolean;
}

/** How many sessions a computer holds, and a dot when one of them needs you. */
function SessionCount({ sessions }: { sessions: readonly SessionPayload[] }) {
  const needsYou = sessions.filter(
    session => sessionBadgeSignal(session.badge).attention === "needs-you"
  ).length;
  return (
    <span className="flex shrink-0 items-center gap-1.5">
      {needsYou > 0 ? <StatusDot label={`${needsYou} need you`} size="sm" tone="attn" /> : null}
      <span className="font-mono text-micro text-faint tabular-nums">{sessions.length}</span>
    </span>
  );
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

export function ComputerScopePicker({
  local,
  computers,
  sessionsByComputer,
  value,
  onChange,
  defaultOpen = false,
}: ComputerScopePickerProps) {
  const [open, setOpen] = useState(defaultOpen);
  const [query, setQuery] = useState("");
  // One computer has nothing to choose between: the control stays out of the way.
  if (computers.length === 0) return null;
  const all = [...sessionsByComputer.values()].flat();
  const select = (next: ComputerScope) => {
    onChange(next);
    setOpen(false);
    setQuery("");
  };
  return (
    <CommandSelect onOpenChange={setOpen} open={open}>
      <CommandSelectTrigger
        aria-label={`Show sessions from: ${value === "all" ? "All computers" : value}`}
        className={cn(
          "h-7 w-auto max-w-56 min-w-0 gap-1.5 rounded-md border-line bg-surface-2/70 px-2.5 py-0 text-eyebrow hover:bg-surface-2 [&>svg:last-child]:hidden",
          value !== "all" && "border-brand-dim"
        )}
        data-testid="computer-scope-trigger"
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
        inputProps={{ value: query, onValueChange: setQuery }}
        side="bottom"
      >
        <CommandList className="min-h-0 flex-1 overflow-y-auto bg-popover">
          <CommandEmpty>No matching computer.</CommandEmpty>
          <CommandSelectGroup>
            <CommandItem
              className="items-center gap-2.5 py-1.5"
              data-checked={value === "all" ? "true" : "false"}
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
              <SessionCount sessions={all} />
            </CommandItem>
            <CommandItem
              className="items-center gap-2.5 py-1.5"
              data-checked={value === "local" ? "true" : "false"}
              onSelect={() => select("local")}
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
              <SessionCount sessions={sessionsByComputer.get("local") ?? []} />
            </CommandItem>
          </CommandSelectGroup>
          <CommandSeparator />
          <CommandSelectGroup heading="Computers">
            {computers.map(computer => (
              <RunOnComputerItem
                computer={computer}
                highlightReconnect={false}
                key={computer.id}
                onReconnect={() => undefined}
                onSelect={select}
                reconnecting={false}
                selected={value === computer.id}
                testIdPrefix="computer-scope"
                trailing={<SessionCount sessions={sessionsByComputer.get(computer.id) ?? []} />}
              />
            ))}
          </CommandSelectGroup>
        </CommandList>
        <div className="shrink-0 border-t border-line-soft bg-popover p-1">
          <Button
            className="h-8 w-full justify-start gap-2 px-2.5 text-small-body font-normal text-muted-foreground hover:text-fg"
            onClick={() => setOpen(false)}
            size="sm"
            type="button"
            variant="quiet"
          >
            <Settings2 aria-hidden="true" className="size-3.5 shrink-0" />
            <span>Manage computers…</span>
          </Button>
        </div>
      </CommandSelectShell>
    </CommandSelect>
  );
}
