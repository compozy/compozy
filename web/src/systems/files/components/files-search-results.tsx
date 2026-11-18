import { ChevronDown, ChevronRight, File } from "@compozy/ui/icons";

import { cn } from "@/lib/utils";

import type { FileSearchFile, FileSearchMatch } from "../adapters/files-api";
import { useFilesSearchResults } from "../hooks/use-files-search-results";
import { splitPath } from "../lib/go-to-file-query";
import { searchExcerpt } from "../lib/search-excerpt";
import { HighlightedText } from "./highlighted-text";

function FileRow({
  file,
  collapsed,
  onToggle,
}: {
  file: FileSearchFile;
  collapsed: boolean;
  onToggle: () => void;
}) {
  const { name, directory } = splitPath(file.path);
  const Chevron = collapsed ? ChevronRight : ChevronDown;
  return (
    <button
      type="button"
      tabIndex={-1}
      aria-expanded={!collapsed}
      onClick={onToggle}
      data-testid="files-search-file"
      data-path={file.path}
      className="flex h-full w-full min-w-0 items-center gap-1.5 px-2 text-left text-xs hover:bg-hover"
    >
      <Chevron aria-hidden className="size-3 shrink-0 text-muted-foreground" />
      <File aria-hidden className="size-3.5 shrink-0 text-muted-foreground" />
      <span className="shrink-0 truncate font-medium text-fg">{name}</span>
      <span className="min-w-0 flex-1 truncate text-muted-foreground">{directory}</span>
      <span className="shrink-0 rounded-pill bg-surface-2 px-1.5 text-meta text-muted-foreground tabular-nums">
        {file.truncated_matches ? "100+" : file.matches.length}
      </span>
    </button>
  );
}

function MatchRow({
  match,
  selected,
  onOpen,
}: {
  match: FileSearchMatch;
  selected: boolean;
  onOpen: () => void;
}) {
  const excerpt = searchExcerpt(match);
  return (
    <div
      role="option"
      aria-selected={selected}
      tabIndex={-1}
      onClick={onOpen}
      data-testid="files-search-match"
      data-line={match.line}
      className={cn(
        "flex h-full min-w-0 cursor-default items-center gap-2 pr-2 pl-7 font-mono text-xs text-fg-2 hover:bg-hover",
        selected && "bg-selected text-fg hover:bg-selected"
      )}
    >
      <span className="w-8 shrink-0 text-right text-faint tabular-nums">{match.line}</span>
      <span className="min-w-0 flex-1 truncate whitespace-pre">
        {excerpt.leading ? <span className="text-faint">…</span> : null}
        <HighlightedText text={excerpt.text} ranges={excerpt.ranges} />
        {excerpt.trailing ? <span className="text-faint">…</span> : null}
      </span>
    </div>
  );
}

export interface FilesSearchResultsProps {
  files: readonly FileSearchFile[];
  /** Dimmed while a newer search runs. */
  dimmed: boolean;
  onOpenMatch: (file: FileSearchFile, match: FileSearchMatch) => void;
}

/**
 * The Search pane's virtualized results (`_uiux.md` S12): file header rows that collapse,
 * then one row per matching line. ↑/↓ move between matches and open each one.
 */
export function FilesSearchResults({ files, dimmed, onOpenMatch }: FilesSearchResultsProps) {
  const { scrollRef, rows, selected, virtualizer, open, toggle, onKeyDown } = useFilesSearchResults(
    files,
    onOpenMatch
  );
  return (
    <div
      ref={scrollRef}
      role="listbox"
      aria-label="Search results"
      tabIndex={0}
      onKeyDown={onKeyDown}
      data-testid="files-search-results"
      className={cn(
        "min-h-0 flex-1 overflow-auto outline-none focus-visible:shadow-focus-inset",
        dimmed && "opacity-60"
      )}
    >
      <div className="relative w-full" style={{ height: virtualizer.getTotalSize() }}>
        {virtualizer.getVirtualItems().map(item => {
          const row = rows[item.index];
          if (!row) return null;
          return (
            <div
              key={item.key}
              className="absolute inset-x-0 top-0"
              style={{ height: item.size, transform: `translateY(${item.start}px)` }}
            >
              {row.kind === "file" ? (
                <FileRow
                  file={row.file}
                  collapsed={row.collapsed}
                  onToggle={() => toggle(row.file.path)}
                />
              ) : row.kind === "match" ? (
                <MatchRow
                  match={row.match}
                  selected={row.key === selected}
                  onOpen={() => open(row)}
                />
              ) : (
                <p className="flex h-full items-center pl-17 text-xs text-muted-foreground">
                  100+ in this file
                </p>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
