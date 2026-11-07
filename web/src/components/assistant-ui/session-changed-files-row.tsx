import { FileText } from "@compozy/ui/icons";

import { CHANGED_FILES_VISIBLE_CAP } from "./session-timeline-changed-files";
import type { ChangedFileEntry, SessionChangedFilesRow } from "./session-timeline.logic";
import { TranscriptDisclosure, cn } from "@compozy/ui";

// Signal palette as information, never decoration: additions read `--success`,
// deletions `--color-destructive`. The +/− sign carries the meaning too, so the stat never
// depends on color alone.
function DiffStat({ additions, deletions }: { additions: number; deletions: number }) {
  return (
    <span
      data-slot="changed-files-diffstat"
      className="flex shrink-0 items-center gap-1.5 font-mono text-transcript-caption tabular-nums"
    >
      <span className="font-medium text-success">+{additions}</span>
      <span className="font-medium text-destructive">−{deletions}</span>
    </span>
  );
}

function splitPath(path: string): { dir: string; name: string } {
  const index = path.lastIndexOf("/");
  if (index < 0) return { dir: "", name: path };
  return { dir: path.slice(0, index + 1), name: path.slice(index + 1) };
}

const ROW_CLASS = "flex min-h-transcript-row min-w-0 items-center gap-2 text-transcript-body";

function ChangedFileRow({
  file,
  onOpen,
}: {
  file: ChangedFileEntry;
  /** Opens Changes at this file (US-005.AC-6); absent for a session without a file root. */
  onOpen?: (path: string) => void;
}) {
  const { dir, name } = splitPath(file.path);
  const body = (
    <>
      <FileText aria-hidden="true" className="size-3 shrink-0 text-faint" />
      <span
        className="flex min-w-0 flex-1 items-baseline font-mono text-transcript-caption"
        title={file.path}
      >
        {dir ? <span className="truncate text-subtle">{dir}</span> : null}
        <span className="shrink-0 text-fg">{name}</span>
      </span>
      <DiffStat additions={file.additions} deletions={file.deletions} />
    </>
  );
  if (!onOpen) {
    return (
      <div data-testid="changed-file-row" className={ROW_CLASS}>
        {body}
      </div>
    );
  }
  return (
    <button
      type="button"
      data-testid="changed-file-row"
      aria-label={`Review changes in ${name}`}
      className={cn(
        ROW_CLASS,
        "w-full cursor-pointer rounded-sm text-left outline-none hover:bg-surface-2 focus-visible:shadow-focus-ring"
      )}
      onClick={() => onOpen(file.path)}
    >
      {body}
    </button>
  );
}

/**
 * The settled-turn changed-files outcome, calm-transcript grammar: one
 * "Edited N files +A −D" line that expands to bare mono file lines behind the
 * detail rail — no card, no fill. A file opens the session's Changes at it
 * (no checkpoint/Undo semantics), capped at 8 lines with a bare "+N more" tail.
 */
export function SessionChangedFilesRowView({
  row,
  onToggle,
  onOpenFile,
}: {
  row: SessionChangedFilesRow;
  onToggle: () => void;
  /** Opens the session's Changes at a file; absent where there is no file root. */
  onOpenFile?: (path: string) => void;
}) {
  const fileCount = row.files.length;
  const visibleFiles = row.files.slice(0, CHANGED_FILES_VISIBLE_CAP);
  const hiddenCount = fileCount - visibleFiles.length;
  const detailsId = `${row.id}:entries`;
  return (
    <div data-testid="changed-files-row" data-expanded={String(row.expanded)} className="min-w-0">
      <TranscriptDisclosure
        expanded={row.expanded}
        onToggle={onToggle}
        aria-controls={detailsId}
        icon={<FileText aria-hidden="true" className="size-3 shrink-0 text-subtle" />}
        label={
          <>
            Edited {fileCount} {fileCount === 1 ? "file" : "files"}
          </>
        }
        trailing={<DiffStat additions={row.additions} deletions={row.deletions} />}
      />
      <div
        id={detailsId}
        data-testid="changed-files-list"
        hidden={!row.expanded}
        aria-hidden={!row.expanded}
        inert={!row.expanded}
        className={
          row.expanded
            ? "mt-0.5 mb-0.5 ml-transcript-detail-indent flex flex-col gap-px border-l border-line pl-transcript-detail-gutter"
            : undefined
        }
      >
        {row.expanded ? (
          <>
            {visibleFiles.map(file => (
              <ChangedFileRow
                key={file.path}
                file={file}
                {...(onOpenFile ? { onOpen: onOpenFile } : {})}
              />
            ))}
            {hiddenCount > 0 ? (
              <span className="min-h-transcript-row content-center text-transcript-meta text-subtle">
                +{hiddenCount} more
              </span>
            ) : null}
          </>
        ) : null}
      </div>
    </div>
  );
}
