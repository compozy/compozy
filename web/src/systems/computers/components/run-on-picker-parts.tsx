import { Scale } from "@compozy/ui/icons";

import { cn, CommandItem, CommandSelectGroup, Spinner, StatusDot } from "@compozy/ui";

import type { ComputerFixture, LocalComputerFixture } from "../mocks/remote-computers-fixtures";
import { ComputerLoadMeter } from "./computer-load-meter";
import { ComputerPlatformIcon } from "./computer-platform-icon";

export interface RunOnTriggerSummary {
  kind: "auto" | "local" | "remote";
  name: string;
  suffix?: string;
  degraded?: "offline" | "connecting" | "warning" | "pro";
  computer?: ComputerFixture;
}

export interface RunOnAutoSummary {
  computerId: string;
  computerName: string;
  reason: string;
}

function suffixToneClass(degraded: RunOnTriggerSummary["degraded"]): string {
  if (degraded === "warning") return "text-warning";
  if (degraded === "pro") return "text-brand";
  return "text-muted-foreground";
}

function RunOnTriggerIcon({
  current,
  localComputer,
}: {
  current: RunOnTriggerSummary;
  localComputer: LocalComputerFixture;
}) {
  if (current.kind === "auto") {
    return (
      <Scale
        aria-hidden="true"
        className={cn(
          "size-3.5 shrink-0",
          current.degraded === "warning" ? "text-warning" : "text-subtle"
        )}
      />
    );
  }
  const source = current.kind === "local" ? localComputer : current.computer;
  return (
    <ComputerPlatformIcon
      className="size-3.5 shrink-0"
      iconKind={source?.iconKind}
      platform={source?.platform}
      size="sm"
      tone="muted"
    />
  );
}

export function RunOnTriggerInner({
  current,
  localComputer,
  showRunOnLabel,
}: {
  current: RunOnTriggerSummary;
  localComputer: LocalComputerFixture;
  showRunOnLabel: boolean;
}) {
  return (
    <span className="flex min-w-0 flex-1 items-center gap-1.5 text-left">
      <RunOnTriggerIcon current={current} localComputer={localComputer} />
      {showRunOnLabel ? <span className="shrink-0 text-muted-foreground">Run on</span> : null}
      <span className="truncate font-medium text-fg">{current.name}</span>
      {current.suffix ? (
        <span
          className={cn(
            "inline-flex shrink-0 items-center gap-1 text-eyebrow",
            suffixToneClass(current.degraded)
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
}

function autoDescription(autoLoading: boolean, autoResolution: RunOnAutoSummary | null): string {
  if (autoLoading) return "Checking eligible computers…";
  if (autoResolution) return "Picks the least busy computer";
  return "No computer can run this project";
}

function RunOnAutoItem({
  selected,
  autoLoading,
  autoResolution,
  testId,
  onSelect,
}: {
  selected: boolean;
  autoLoading: boolean;
  autoResolution: RunOnAutoSummary | null;
  testId: string;
  onSelect: () => void;
}) {
  const healthy = Boolean(autoResolution || autoLoading);
  return (
    <CommandItem
      className="items-center gap-2.5 py-1.5"
      data-checked={selected ? "true" : "false"}
      data-testid={`${testId}-item-auto`}
      onSelect={onSelect}
      value={`Auto load balancing ${autoResolution?.computerName ?? ""}`}
    >
      <Scale
        aria-hidden="true"
        className={cn("size-4 shrink-0", healthy ? "text-subtle" : "text-warning")}
      />
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="flex items-center gap-1.5 text-small-body font-medium text-fg">
          <span>Auto</span>
          {autoLoading ? <Spinner aria-hidden="true" className="size-3" /> : null}
          {!autoLoading && autoResolution ? (
            <span className="font-normal text-muted-foreground">
              → {autoResolution.computerName}
            </span>
          ) : null}
        </span>
        <span className={cn("truncate text-eyebrow", healthy ? "text-subtle" : "text-warning")}>
          {autoDescription(autoLoading, autoResolution)}
        </span>
      </span>
    </CommandItem>
  );
}

export function RunOnDefaultGroup({
  showAuto,
  localComputer,
  value,
  autoResolution,
  autoLoading,
  testId,
  onSelect,
}: {
  showAuto: boolean;
  localComputer: LocalComputerFixture;
  value: string;
  autoResolution: RunOnAutoSummary | null;
  autoLoading: boolean;
  testId: string;
  onSelect: (next: "auto" | "local") => void;
}) {
  if (!showAuto && !localComputer.localRunAgents) {
    return null;
  }

  return (
    <CommandSelectGroup heading="Default">
      {showAuto ? (
        <RunOnAutoItem
          autoLoading={autoLoading}
          autoResolution={autoResolution}
          onSelect={() => onSelect("auto")}
          selected={value === "auto"}
          testId={testId}
        />
      ) : null}

      {localComputer.localRunAgents ? (
        <CommandItem
          className="items-center gap-2.5 py-1.5"
          data-checked={value === "local" ? "true" : "false"}
          data-testid={`${testId}-item-local`}
          onSelect={() => onSelect("local")}
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
            <span className="truncate text-small-body font-medium text-fg">This computer</span>
            <span className="flex items-center gap-1.5 truncate text-eyebrow text-subtle">
              <StatusDot size="sm" tone="success" />
              <span className="truncate">{localComputer.name}</span>
            </span>
          </span>
          <ComputerLoadMeter computerName={localComputer.name} load={localComputer.load} />
        </CommandItem>
      ) : null}
    </CommandSelectGroup>
  );
}
