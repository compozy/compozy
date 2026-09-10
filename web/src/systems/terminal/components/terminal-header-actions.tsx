import { CircleStop, Ellipsis, Plus, ScrollText } from "lucide-react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Separator,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";

import type { TerminalHeaderProps } from "./terminal-header";

/**
 * At most two trailing actions, set off from the chips by a hairline.
 *
 * Stop and Wait stay available; closing belongs to the OS window chrome.
 */
export function TerminalHeaderActions({
  isPipe,
  recording,
  onStop,
  onSignal,
  onWait,
  onStopRecording,
}: TerminalHeaderActionsProps) {
  if (isPipe) {
    return <TerminalPipeHeaderActions onSignal={onSignal} onWait={onWait} />;
  }
  // Stopping the recording is ghost text; danger stays on the rec dot.
  const quietAction =
    recording && onStopRecording ? (
      <Button
        data-testid="terminal-stop-recording"
        onClick={onStopRecording}
        size="sm"
        type="button"
        variant="ghost"
      >
        Stop recording
      </Button>
    ) : onStop ? (
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              aria-label="Stop"
              data-testid="terminal-stop"
              onClick={onStop}
              size="icon-sm"
              type="button"
              variant="ghost"
            />
          }
        >
          <CircleStop aria-hidden="true" className="size-3.5 text-danger" />
        </TooltipTrigger>
        <TooltipContent side="bottom">Stop</TooltipContent>
      </Tooltip>
    ) : null;
  if (!quietAction) return null;
  return (
    <>
      <TerminalHeaderRule />
      {quietAction}
    </>
  );
}

/**
 * Window-level verbs: another terminal, and the journal. They belong to the
 * window rather than to the active terminal, so they trail everything else.
 */
export function TerminalWindowVerbs({
  onNewTerminal,
  onViewJournal,
}: Pick<TerminalHeaderProps, "onNewTerminal" | "onViewJournal">) {
  if (!onNewTerminal && !onViewJournal) return null;
  return (
    <>
      <TerminalHeaderRule />
      {onNewTerminal ? (
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                aria-label="New terminal"
                data-testid="terminal-new"
                onClick={onNewTerminal}
                size="icon-sm"
                type="button"
                variant="ghost"
              />
            }
          >
            <Plus aria-hidden="true" className="size-3.5" />
          </TooltipTrigger>
          <TooltipContent side="bottom">New terminal</TooltipContent>
        </Tooltip>
      ) : null}
      {onViewJournal ? (
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                aria-label="Journal"
                data-testid="terminal-journal-toggle"
                onClick={onViewJournal}
                size="icon-sm"
                type="button"
                variant="ghost"
              />
            }
          >
            <ScrollText aria-hidden="true" className="size-3.5" />
          </TooltipTrigger>
          <TooltipContent side="bottom">Journal</TooltipContent>
        </Tooltip>
      ) : null}
    </>
  );
}

/** Keeps pipe supervision actions available while window chrome owns close confirmation. */
function TerminalPipeHeaderActions({
  onSignal,
  onWait,
}: Pick<TerminalHeaderActionsProps, "onSignal" | "onWait">) {
  const wait = onWait ? (
    <Button data-testid="terminal-wait" onClick={onWait} size="sm" type="button" variant="ghost">
      Wait
    </Button>
  ) : null;
  const overflow = onSignal ? (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            aria-label="More actions"
            data-testid="terminal-pipe-overflow"
            size="icon-sm"
            type="button"
            variant="ghost"
          />
        }
      >
        <Ellipsis aria-hidden="true" className="size-3.5" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem data-testid="terminal-signal" onClick={onSignal}>
          Signal
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  ) : null;
  if (!wait && !overflow) return null;
  return (
    <>
      <TerminalHeaderRule />
      {wait}
      {overflow}
    </>
  );
}

function TerminalHeaderRule() {
  return <Separator className="h-3.5 self-center" orientation="vertical" />;
}

type TerminalHeaderActionsProps = Pick<
  TerminalHeaderProps,
  "recording" | "onStop" | "onSignal" | "onWait" | "onStopRecording"
> & { isPipe: boolean };
