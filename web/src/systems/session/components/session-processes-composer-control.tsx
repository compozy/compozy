import { Cpu } from "lucide-react";

import {
  Button,
  InputGroupButton,
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
} from "@compozy/ui";

import { processControlLabel } from "../lib/session-processes";
import { SessionProcessList, type SessionProcessListProps } from "./session-process-list";

export interface SessionProcessesComposerControlProps extends Omit<
  SessionProcessListProps,
  "placeholderCount"
> {
  /** Processes the session left running; the trigger shows only while it is positive. */
  count: number;
  /** Stops every process the session started; offered while the list has rows. */
  onStopAll?: () => void;
  /** The popover opened or closed; the owner reads the list while it is open. */
  onOpenChange?: (open: boolean) => void;
}

/**
 * The composer's processes control: shown only while the session has processes
 * still running, including after it stops or is archived, so a dev server an
 * agent left behind never runs unseen. Its popover lists each process with its
 * ports and a Stop, plus Stop all.
 */
export function SessionProcessesComposerControl({
  count,
  onStopAll,
  onOpenChange,
  ...list
}: SessionProcessesComposerControlProps) {
  if (count <= 0) return null;
  const stoppingAll = list.stopping === "all";
  const canStopAll =
    onStopAll !== undefined &&
    list.processes.length > 0 &&
    !list.missing &&
    list.supported !== false;
  return (
    <Popover onOpenChange={open => onOpenChange?.(open)}>
      <PopoverTrigger
        render={
          <InputGroupButton
            type="button"
            size="tool"
            aria-label={processControlLabel(count)}
            data-testid="composer-processes-button"
            className="shrink-0 data-[popup-open]:bg-surface-2"
          />
        }
      >
        <Cpu aria-hidden="true" className="size-4" />
        <span className="font-mono text-mono-id tabular-nums">{count}</span>
      </PopoverTrigger>
      <PopoverContent
        side="top"
        align="start"
        className="w-96 max-w-[calc(100vw-2rem)] gap-2"
        data-testid="composer-processes-popover"
      >
        <div className="flex items-baseline gap-2 px-0.5">
          <PopoverTitle className="text-transcript-meta font-semibold text-fg-strong">
            Processes
          </PopoverTitle>
          <span className="text-transcript-caption text-faint">Started by this session</span>
          {canStopAll ? (
            <Button
              type="button"
              variant="quiet"
              size="xs"
              className="ml-auto"
              disabled={Boolean(list.stopping)}
              onClick={onStopAll}
              data-testid="composer-processes-stop-all"
            >
              {stoppingAll ? "Stopping…" : "Stop all"}
            </Button>
          ) : null}
        </div>
        <SessionProcessList {...list} placeholderCount={count} />
      </PopoverContent>
    </Popover>
  );
}
