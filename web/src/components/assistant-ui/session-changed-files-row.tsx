import { ChevronDown, FileText } from "lucide-react";
import { useRef } from "react";

import { cn } from "@/lib/utils";
import type { ChangedFileEntry, SessionChangedFilesRow } from "./session-timeline.logic";

// Expanded file lines cap — beyond it a bare "+N more" line closes the list.
const CHANGED_FILES_VISIBLE_CAP = 8;

// Signal palette as information, never decoration: additions read `--success`,
// deletions `--danger`. The +/− sign carries the meaning too, so the stat never
// depends on color alone.
function DiffStat({ additions, deletions }: { additions: number; deletions: number }) {
  return (
    <span
      data-slot="changed-files-diffstat"
      className="flex shrink-0 items-center gap-[5px] font-mono text-[11px] tabular-nums"
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
      className="flex min-h-[22px] min-w-0 items-center gap-2 text-[12px]"
    >
      <FileText aria-hidden="true" className="size-3 shrink-0 text-faint" strokeWidth={1.75} />
      <span className="flex min-w-0 flex-1 items-baseline font-mono text-[11px]" title={file.path}>
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
  onToggle: (button: HTMLElement | null) => void;
}) {
  const ref = useRef<HTMLButtonElement | null>(null);
  const fileCount = row.files.length;
  const visibleFiles = row.files.slice(0, CHANGED_FILES_VISIBLE_CAP);
  const hiddenCount = fileCount - visibleFiles.length;
  return (
    <div data-testid="changed-files-row" data-expanded={String(row.expanded)} className="min-w-0">
      <button
        ref={ref}
        type="button"
        aria-expanded={row.expanded}
        onClick={() => onToggle(ref.current)}
        className={cn(
          "group inline-flex min-h-6 items-center gap-[7px] rounded-md px-1 text-left",
          "text-small-body font-medium text-muted",
          "transition-colors duration-base ease-out hover:bg-hover hover:text-fg",
          "focus-visible:shadow-focus-ring focus-visible:outline-none"
        )}
      >
        <span className="flex size-[18px] shrink-0 items-center justify-center">
          <FileText aria-hidden="true" className="size-3 shrink-0 text-subtle" strokeWidth={1.8} />
        </span>
        <span className="min-w-0 shrink truncate">
          Edited {fileCount} {fileCount === 1 ? "file" : "files"}
        </span>
        <DiffStat additions={row.additions} deletions={row.deletions} />
        <ChevronDown
          aria-hidden="true"
          className={cn(
            "size-3 shrink-0 text-faint transition-transform duration-slow ease-out motion-reduce:transition-none",
            row.expanded ? "rotate-180" : null
          )}
          strokeWidth={1.75}
        />
      </button>
      {row.expanded ? (
        <div
          data-testid="changed-files-list"
          className="mt-0.5 mb-0.5 ml-[25px] flex flex-col gap-px border-l border-line pl-[11px]"
        >
          {visibleFiles.map(file => (
            <ChangedFileRow key={file.path} file={file} />
          ))}
          {hiddenCount > 0 ? (
            <span className="min-h-[22px] content-center text-[11.5px] text-subtle">
              +{hiddenCount} more
            </span>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
