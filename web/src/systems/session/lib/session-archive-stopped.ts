import type {
  ArchiveStoppedScope,
  SessionArchiveStoppedResponse,
} from "../adapters/session-archive-api";
import type { SessionBatchResult } from "./session-batch";

/** How many failed titles a toast names before "and N more". */
const LISTED_FAILURES = 3;

const SKIP_REASONS: Record<string, string> = {
  waiting_on_you: "waiting on you",
  running: "running",
  queued_input: "queued input",
  pinned: "pinned",
  later: "in Later",
  instance_main: "always-on main session",
  caller: "the calling session",
};

/** One key per scope, so each scope's run is in flight on its own. */
export function archiveStoppedScopeKey(scope: ArchiveStoppedScope): string {
  return scope.kind === "all" ? "all" : `project:${scope.workspaceId}`;
}

export function threadCount(count: number): string {
  return `${count} ${count === 1 ? "thread" : "threads"}`;
}

/** "3 skipped: 2 waiting on you, 1 running"; `undefined` when nothing was skipped. */
export function archiveStoppedSkipSummary(
  skipCounts: Readonly<Record<string, number>>
): string | undefined {
  // Largest count first, then by reason, as the CLI prints it.
  const reasons = Object.entries(skipCounts)
    .filter(([, count]) => count > 0)
    .sort(([a, countA], [b, countB]) => countB - countA || a.localeCompare(b));
  const total = reasons.reduce((sum, [, count]) => sum + count, 0);
  if (total === 0) return undefined;
  const parts = reasons.map(([reason, count]) => `${count} ${SKIP_REASONS[reason] ?? reason}`);
  return `${total} skipped: ${parts.join(", ")}`;
}

/** "Alpha, Beta, Gamma and 2 more"; `total` counts titles the capped list left out. */
export function listedTitles(titles: readonly string[], total = titles.length): string {
  const shown = titles.slice(0, LISTED_FAILURES).join(", ");
  const rest = Math.max(total, titles.length) - Math.min(titles.length, LISTED_FAILURES);
  return rest > 0 ? `${shown} and ${rest} more` : shown;
}

export type ArchiveStoppedOutcome =
  | { kind: "empty"; description?: string }
  | { kind: "archived"; title: string; description?: string }
  | { kind: "failed"; title: string; description: string; undoable: boolean };

/** The toast a finished run shows: archived, nothing eligible, or failures. */
export function archiveStoppedOutcome(
  report: SessionArchiveStoppedResponse
): ArchiveStoppedOutcome {
  const skipped = archiveStoppedSkipSummary(report.skip_counts);
  const archived = report.archived.length;
  // `failures` lists at most 100 threads; `failure_count` counts them all.
  const failed = report.failure_count;
  if (failed > 0) {
    const titles = listedTitles(
      report.failures.map(failure => failure.title || failure.root_id),
      failed
    );
    return {
      kind: "failed",
      title:
        archived > 0
          ? `Archived ${threadCount(archived)}, ${failed} failed`
          : `Couldn't archive ${threadCount(failed)}`,
      description: [titles, skipped].filter(Boolean).join(" · "),
      undoable: archived > 0,
    };
  }
  if (archived === 0) return { kind: "empty", description: skipped };
  return { kind: "archived", title: `Archived ${threadCount(archived)}`, description: skipped };
}

/**
 * The session ids Undo restores, root first within each thread, and the thread each belongs to.
 * Only the run's `restore_ids` appear (Rule 6): nothing skipped is touched, and subagents are left
 * to their parent's unarchive, which restores them.
 */
export function archiveStoppedUndoPlan(report: SessionArchiveStoppedResponse): {
  ids: string[];
  threadOf: Map<string, string>;
} {
  const ids: string[] = [];
  const threadOf = new Map<string, string>();
  for (const thread of report.archived) {
    for (const id of thread.restore_ids) {
      ids.push(id);
      threadOf.set(id, thread.root_id);
    }
  }
  return { ids, threadOf };
}

/** The toast Undo shows: every thread restored, or the threads that were not. */
export function archiveStoppedUndoOutcome(
  report: SessionArchiveStoppedResponse,
  results: readonly SessionBatchResult[]
): { ok: boolean; title: string; description?: string } {
  const failedIds = new Set<string>();
  for (const result of results) if (result.status === "failed") failedIds.add(result.id);
  const failedThreads = report.archived.filter(thread =>
    thread.restore_ids.some(id => failedIds.has(id))
  );
  const restored = report.archived.length - failedThreads.length;
  if (failedThreads.length === 0) return { ok: true, title: `Restored ${threadCount(restored)}` };
  return {
    ok: false,
    title:
      restored > 0
        ? `Restored ${threadCount(restored)}, ${failedThreads.length} failed`
        : `Couldn't restore ${threadCount(failedThreads.length)}`,
    description: listedTitles(failedThreads.map(thread => thread.title || thread.root_id)),
  };
}
