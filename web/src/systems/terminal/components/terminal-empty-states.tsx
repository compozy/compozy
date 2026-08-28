import { MonitorX, Moon, RotateCcw, TerminalSquare } from "lucide-react";
import type { ReactNode } from "react";

import { Button, Empty } from "@compozy/ui";

export interface TerminalEmptyStateProps {
  onOpenTerminal?: () => void;
}

function TerminalEmptyFrame({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-0 flex-1 items-center justify-center px-6 py-10">{children}</div>
  );
}

/**
 * A project with no terminals.
 *
 * Onboarding is the empty state: one action, no tour, and it is the only accent
 * on the surface — an empty terminal has nothing to take control of.
 */
export function TerminalEmptyState({ onOpenTerminal }: TerminalEmptyStateProps) {
  return (
    <TerminalEmptyFrame>
      <Empty
        action={
          onOpenTerminal ? (
            <Button
              data-testid="terminal-empty-open"
              onClick={onOpenTerminal}
              size="sm"
              type="button"
            >
              Open a terminal
            </Button>
          ) : undefined
        }
        data-testid="terminal-empty"
        framed
        icon={TerminalSquare}
        title="No terminals yet"
      />
    </TerminalEmptyFrame>
  );
}

export interface TerminalExpiredStateProps {
  onOpenTerminal?: () => void;
  onViewJournal?: () => void;
  /**
   * How long it went unwatched, already phrased — `[terminal].detached_ttl`.
   *
   * Configurable, so it is passed in rather than assumed. Without it the state
   * says what happened and leaves out the duration, which is better than
   * printing a default this installation may not be using.
   */
  idleFor?: string;
}

function terminalHistoryActions({
  onOpenTerminal,
  onViewJournal,
}: Pick<TerminalExpiredStateProps, "onOpenTerminal" | "onViewJournal">): ReactNode | undefined {
  if (!onOpenTerminal && !onViewJournal) return undefined;
  return (
    <>
      {onViewJournal ? (
        <Button onClick={onViewJournal} size="sm" type="button" variant="outline">
          View journal
        </Button>
      ) : null}
      {onOpenTerminal ? (
        <Button onClick={onOpenTerminal} size="sm" type="button" variant="ghost">
          Open a new terminal
        </Button>
      ) : null}
    </>
  );
}

/** Reclaimed after an idle period. The journal outlives the terminal. */
export function TerminalExpiredState({
  idleFor,
  onOpenTerminal,
  onViewJournal,
}: TerminalExpiredStateProps) {
  return (
    <TerminalEmptyFrame>
      <Empty
        action={terminalHistoryActions({ onOpenTerminal, onViewJournal })}
        cause={`terminal_expired · 410 · reclaimed${idleFor ? ` after ${idleFor}` : ""} without viewers`}
        data-testid="terminal-expired"
        description={
          idleFor
            ? `Nobody was watching for ${idleFor}, so it was closed. Its command history is still in the journal.`
            : "Nobody was watching for a while, so it was closed. Its command history is still in the journal."
        }
        framed
        icon={Moon}
        title="This terminal was cleaned up"
      />
    </TerminalEmptyFrame>
  );
}

/**
 * Gone — closed, expired, or never at this address.
 *
 * Distinct from a program's own exit: no exit code is shown, because none
 * exists. The copy does not invent a restart the daemon did not report.
 */
export function TerminalNotFoundState({
  onOpenTerminal,
  onViewJournal,
}: {
  onOpenTerminal?: () => void;
  onViewJournal?: () => void;
}) {
  return (
    <TerminalEmptyFrame>
      <Empty
        action={terminalHistoryActions({ onOpenTerminal, onViewJournal })}
        data-testid="terminal-not-found"
        description="It may have been closed, or this address doesn't match a live terminal. Everything that ran is in the journal."
        framed
        icon={RotateCcw}
        title="This terminal isn't here"
      />
    </TerminalEmptyFrame>
  );
}

/**
 * An execute-only platform.
 *
 * Said before anything can hang, and the interactive option is absent rather
 * than greyed out: a disabled Open would still claim the feature exists here.
 */
export function TerminalExecuteOnlyState({ onViewJournal }: { onViewJournal?: () => void }) {
  return (
    <TerminalEmptyFrame>
      <Empty
        action={
          onViewJournal ? (
            <Button onClick={onViewJournal} size="sm" type="button" variant="outline">
              View journal
            </Button>
          ) : undefined
        }
        cause="terminal_interactive_unavailable · exec available"
        data-testid="terminal-execute-only"
        description="On this platform, agents can still run commands and everything lands in the journal — there's just no live screen to type into."
        framed
        icon={MonitorX}
        title="Interactive terminals aren't available here yet"
      />
    </TerminalEmptyFrame>
  );
}
