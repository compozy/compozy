import {
  cn,
  Skeleton,
  SkeletonRegion,
  SkeletonText,
  Table,
  TableBody,
  TableCell,
  TableRow,
} from "@compozy/ui";

import { TerminalJournalColumns } from "./terminal-journal-columns";

/** Rows a loading journal stands in with: the command's width and the folder line under it. */
const JOURNAL_ROWS = [
  { id: "a", command: "w-2/3", folder: "w-1/2" },
  { id: "b", command: "w-1/2", folder: "w-2/5" },
  { id: "c", command: "w-3/4", folder: "w-1/2" },
  { id: "d", command: "w-2/5", folder: "w-1/3" },
  { id: "e", command: "w-3/5", folder: "w-2/5" },
  { id: "f", command: "w-1/2", folder: "w-1/2" },
] as const;

/**
 * The journal's table while its rows are read: the real column heads in the table's own frame,
 * with each cell of a row as blocks in the live row's two-line anatomy (command over folder,
 * outcome over code). One busy status for the table; the toolbar above it stays live.
 */
export function TerminalJournalTableSkeleton() {
  return (
    <SkeletonRegion className="flex min-h-0 min-w-0 flex-1" label="Loading the journal">
      <div className="min-h-0 min-w-0 flex-1 overflow-y-auto">
        <Table aria-hidden="true" className="table-fixed" overflowX="hidden">
          <TerminalJournalColumns />
          <TableBody>
            {JOURNAL_ROWS.map(row => (
              <TableRow className="animate-skeleton-reveal hover:bg-transparent" key={row.id}>
                <TableCell>
                  <SkeletonText className="w-10 font-mono text-transcript-meta" />
                </TableCell>
                <TableCell>
                  <span className="flex min-w-0 items-center gap-1.5">
                    <Skeleton className="size-avatar-sm flex-none rounded-full" />
                    <SkeletonText className="w-16 text-form-input" />
                  </span>
                </TableCell>
                <TableCell className="min-w-0 whitespace-normal">
                  <SkeletonText className={cn("font-mono text-code-block", row.command)} />
                  <SkeletonText className={cn("mt-0.75 font-mono text-eyebrow", row.folder)} />
                </TableCell>
                <TableCell>
                  {/* The line box the live pill makes (22px), so the row is as tall as a real one. */}
                  <div className="flex h-5.5 items-center">
                    <Skeleton className="h-pill-sm w-16 rounded-pill" />
                  </div>
                  <SkeletonText className="mt-0.75 w-10 font-mono text-eyebrow" />
                </TableCell>
                <TableCell>
                  <SkeletonText className="w-16 text-form-input" />
                </TableCell>
                <TableCell>
                  <span className="inline-flex">
                    <Skeleton className="h-pill-xs w-16 rounded-pill" />
                  </span>
                </TableCell>
                <TableCell />
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </SkeletonRegion>
  );
}
