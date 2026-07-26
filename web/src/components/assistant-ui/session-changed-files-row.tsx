import { ChevronRight, FileText } from "lucide-react";
import { useRef } from "react";

import { cn } from "@/lib/utils";
import { Eyebrow } from "@agh/ui";
import type { ChangedFileEntry, SessionChangedFilesRow } from "./session-timeline.logic";

// Signal palette as information, never decoration: additions read `--success`,
// deletions `--danger`. The +/- sign carries the meaning too, so the stat never
// depends on color alone.
function DiffStat({ additions, deletions }: { additions: number; deletions: number }) {
  return (
    <span
      data-slot="changed-files-diffstat"
      className="flex shrink-0 items-center gap-2 font-mono text-eyebrow tabular-nums"
    >
      <span className="text-success">+{additions}</span>
      <span className="text-danger">-{deletions}</span>
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
      className="flex min-w-0 items-center gap-2 px-2.5 py-1 text-small-body"
    >
      <FileText aria-hidden="true" className="size-3.5 shrink-0 text-subtle" strokeWidth={1.75} />
      <span className="flex min-w-0 flex-1 items-baseline" title={file.path}>
        {dir ? <span className="truncate text-subtle">{dir}</span> : null}
        <span className="shrink-0 text-fg">{name}</span>
      </span>
      <DiffStat additions={file.additions} deletions={file.deletions} />
    </div>
  );
}

/**
 * The settled-turn "Changed files" roll-up card. Collapsed, it reads
 * `Edited N files +a/-d`; expanded, it lists each modified file with its diff
 * stats. Display-only — AGH exposes no checkpoint/Undo semantics, so this
 * carries no Undo/Review actions (truthful UI). Styling per `analysis/07 §4.2
 * #26`: `--radius-lg` + `border-line` + `bg-canvas-soft` + `<Eyebrow>` header,
 * no `bg-card/45` translucency or custom shadow.
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
  return (
    <div
      data-testid="changed-files-row"
      data-expanded={String(row.expanded)}
      className="my-1 overflow-hidden rounded-md border border-line bg-canvas-soft"
    >
      <button
        ref={ref}
        type="button"
        aria-expanded={row.expanded}
        onClick={() => onToggle(ref.current)}
        className={cn(
          "group flex w-full min-w-0 items-center gap-2 px-2.5 py-1.5 text-left",
          "transition-colors duration-base ease-out hover:bg-hover",
          "focus-visible:outline-none focus-visible:shadow-focus-ring"
        )}
      >
        <ChevronRight
          aria-hidden="true"
          className={cn(
            "size-3 shrink-0 text-subtle transition-transform duration-base ease-out motion-reduce:transition-none",
            row.expanded ? "rotate-90 text-muted" : null
          )}
          strokeWidth={1.75}
        />
        <Eyebrow className="min-w-0 shrink truncate text-muted group-hover:text-fg">
          Edited {fileCount} {fileCount === 1 ? "file" : "files"}
        </Eyebrow>
        <span className="ml-auto shrink-0">
          <DiffStat additions={row.additions} deletions={row.deletions} />
        </span>
      </button>
      {row.expanded ? (
        <div data-testid="changed-files-list" className="border-t border-line py-0.5">
          {row.files.map(file => (
            <ChangedFileRow key={file.path} file={file} />
          ))}
        </div>
      ) : null}
    </div>
  );
}
