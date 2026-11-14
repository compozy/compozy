import { Eraser } from "@compozy/ui/icons";

import { Button, Spinner } from "@compozy/ui";

/**
 * The Archived list's entry point (#1717): a quiet footer verb under the list, never a loud
 * call to action, and only on the Archived view. It names how much the archive holds so the
 * reason to clean up is visible without opening anything.
 */
export function ArchivedCleanupFooter({
  archivedCount,
  progress,
  onCleanUp,
  testIdPrefix = "archived-cleanup",
}: {
  /** Archived sessions in the list's scope; `undefined` until counted. */
  archivedCount?: number;
  /** A cleanup running in the daemon: the verb becomes its progress and reopens the dialog. */
  progress?: { done: number; total: number } | null;
  onCleanUp: () => void;
  testIdPrefix?: string;
}) {
  return (
    <footer
      aria-label="Archive"
      className="flex shrink-0 items-center gap-2 border-t border-line px-2.5 py-1.5"
      data-testid={`${testIdPrefix}-footer`}
    >
      <Button
        data-testid={`${testIdPrefix}-open`}
        disabled={archivedCount === 0 && !progress}
        onClick={onCleanUp}
        size="sm"
        variant="quiet"
      >
        {progress ? (
          <>
            <Spinner className="size-3" />
            <span className="tabular-nums">
              Deleting… {progress.done.toLocaleString()} of {progress.total.toLocaleString()}
            </span>
          </>
        ) : (
          <>
            <Eraser />
            Clean up history…
          </>
        )}
      </Button>
      {archivedCount !== undefined ? (
        <span className="ml-auto pr-1.5 text-micro text-subtle">
          <span className="text-muted-foreground tabular-nums">
            {archivedCount.toLocaleString()}
          </span>{" "}
          archived
        </span>
      ) : null}
    </footer>
  );
}
