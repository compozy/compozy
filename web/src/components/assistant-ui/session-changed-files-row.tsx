import { FileText } from "lucide-react";

import { CHANGED_FILES_VISIBLE_CAP } from "./session-timeline-changed-files";
import type { ChangedFileEntry, SessionChangedFilesRow } from "./session-timeline.logic";
import { TranscriptDisclosure } from "./transcript-disclosure";

// Signal palette as information, never decoration: additions read `--success`,
// deletions `--danger`. The +/− sign carries the meaning too, so the stat never
// depends on color alone.
function DiffStat({ additions, deletions }: { additions: number; deletions: number }) {
  return (
    <span
      data-slot="changed-files-diffstat"
      className="flex shrink-0 items-center gap-1.5 font-mono text-transcript-caption tabular-nums"
    >
      <span className="font-medium text-success">+{additions}</span>
      <span className="font-medium text-danger">−{deletions}</span>
    </span>
  );
}

function splitPath(path: string): { dir: string; name: string } {
  const index = path.lastIndexOf("/");
  if (index < 0) return { dir: "", name: path };
  return { dir: path.slice(0, index + 1), name: path.slice(index + 1) };
}

function ChangedFileRow({ file }: { file: ChangedFileEntry }) {
  const { dir, name } = splitPath(file.path);
  return (
    <div
      data-testid="changed-file-row"
      className="flex min-h-transcript-row min-w-0 items-center gap-2 text-transcript-body"
    >
      <FileText aria-hidden="true" className="size-3 shrink-0 text-faint" strokeWidth={1.75} />
      <span
        className="flex min-w-0 flex-1 items-baseline font-mono text-transcript-caption"
        title={file.path}
      >
        {dir ? <span className="truncate text-subtle">{dir}</span> : null}
        <span className="shrink-0 text-fg">{name}</span>
      </span>
      <DiffStat additions={file.additions} deletions={file.deletions} />
    </div>
  );
}

/**
 * The settled-turn changed-files outcome, calm-transcript grammar: one
 * "Edited N files +A −D" line that expands to bare mono file lines behind the
 * detail rail — no card, no fill. Display-only (CompozyOS exposes no
 * checkpoint/Undo semantics), capped at 8 lines with a bare "+N more" tail.
 */
export function SessionChangedFilesRowView({
  row,
  onToggle,
}: {
  row: SessionChangedFilesRow;
  onToggle: () => void;
}) {
  const fileCount = row.files.length;
  const visibleFiles = row.files.slice(0, CHANGED_FILES_VISIBLE_CAP);
  const hiddenCount = fileCount - visibleFiles.length;
  return (
    <div data-testid="changed-files-row" data-expanded={String(row.expanded)} className="min-w-0">
      <TranscriptDisclosure
        expanded={row.expanded}
        onToggle={onToggle}
        icon={
          <FileText aria-hidden="true" className="size-3 shrink-0 text-subtle" strokeWidth={1.8} />
        }
        label={
          <>
            Edited {fileCount} {fileCount === 1 ? "file" : "files"}
          </>
        }
        trailing={<DiffStat additions={row.additions} deletions={row.deletions} />}
      />
      {row.expanded ? (
        <div
          data-testid="changed-files-list"
          className="mt-0.5 mb-0.5 ml-transcript-detail-indent flex flex-col gap-px border-l border-line pl-transcript-detail-gutter"
        >
          {visibleFiles.map(file => (
            <ChangedFileRow key={file.path} file={file} />
          ))}
          {hiddenCount > 0 ? (
            <span className="min-h-transcript-row content-center text-transcript-meta text-subtle">
              +{hiddenCount} more
            </span>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
