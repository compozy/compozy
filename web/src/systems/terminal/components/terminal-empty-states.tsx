import { MonitorX, Moon, RotateCcw, TerminalSquare } from "lucide-react";

import { Button, Empty } from "@compozy/ui";

export interface TerminalEmptyStateProps {
  onOpenTerminal?: () => void;
  onViewJournal?: () => void;
}

/**
 * A project with no terminals.
 *
 * Onboarding is the empty state: one action, no tour, and it is the only accent
 * on the surface — an empty terminal has nothing to take control of.
 */
export function TerminalEmptyState({ onOpenTerminal }: TerminalEmptyStateProps) {
  return (
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
      description="Open one to run commands in this project — agents can watch, and everything that runs is kept in the journal."
      framed
      icon={TerminalSquare}
      title="No terminals yet"
    />
  );
}

export interface TerminalExpiredStateProps extends TerminalEmptyStateProps {
  /**
   * How long it went unwatched, already phrased — `[terminal].detached_ttl`.
   *
   * Configurable, so it is passed in rather than assumed. Without it the state
   * says what happened and leaves out the duration, which is better than
   * printing a default this installation may not be using.
   */
  idleFor?: string;
}

/** Reclaimed after an idle period. The journal outlives the terminal. */
export function TerminalExpiredState({
  idleFor,
  onOpenTerminal,
  onViewJournal,
}: TerminalExpiredStateProps) {
  return (
    <Empty
      action={
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
      }
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
  );
}

/**
 * Gone after a runtime restart.
 *
 * Deliberately distinct from a program's own exit: no exit code is shown,
 * because none exists.
 */
export function TerminalNotFoundState({ onOpenTerminal, onViewJournal }: TerminalEmptyStateProps) {
  return (
    <Empty
      action={
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
      }
      data-testid="terminal-not-found"
      description="CompozyOS restarted, and live terminals don't carry across. Everything that ran is in the journal."
      framed
      icon={RotateCcw}
      title="This terminal didn't survive the restart"
    />
  );
}

/**
 * An execute-only platform.
 *
 * Said before anything can hang, and the interactive option is absent rather
 * than greyed out: a disabled Open would still claim the feature exists here.
 */
export function TerminalExecuteOnlyState({ onViewJournal }: TerminalEmptyStateProps) {
  return (
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
  );
}
