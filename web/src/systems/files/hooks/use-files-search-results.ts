import { useVirtualizer } from "@tanstack/react-virtual";
import type { KeyboardEvent } from "react";
import { useRef, useState } from "react";

import type { FileSearchFile, FileSearchMatch } from "../adapters/files-api";

export type SearchResultRow =
  | { kind: "file"; file: FileSearchFile; collapsed: boolean }
  | { kind: "match"; file: FileSearchFile; match: FileSearchMatch; key: string }
  | { kind: "more"; file: FileSearchFile };

type MatchRow = Extract<SearchResultRow, { kind: "match" }>;

const ROW_HEIGHT = 24;

function resultRows(
  files: readonly FileSearchFile[],
  collapsed: ReadonlySet<string>
): SearchResultRow[] {
  const rows: SearchResultRow[] = [];
  for (const file of files) {
    const closed = collapsed.has(file.path);
    rows.push({ kind: "file", file, collapsed: closed });
    if (closed) continue;
    file.matches.forEach((match, index) => {
      rows.push({ kind: "match", file, match, key: `${file.path}:${index}` });
    });
    if (file.truncated_matches) rows.push({ kind: "more", file });
  }
  return rows;
}

/**
 * The Search pane's flattened, virtualized rows: collapsible file groups, the selected match,
 * and ↑/↓ stepping between matches (each step opens the match, US-014.AC-4).
 */
export function useFilesSearchResults(
  files: readonly FileSearchFile[],
  onOpenMatch: (file: FileSearchFile, match: FileSearchMatch) => void
) {
  "use no memo";

  const scrollRef = useRef<HTMLDivElement | null>(null);
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(() => new Set());
  const [selected, setSelected] = useState<string | null>(null);
  const rows = resultRows(files, collapsed);
  // react-doctor-disable-next-line react-hooks-js/incompatible-library -- `use no memo` isolates TanStack Virtual's mutable measurement state in this hook.
  const virtualizer = useVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 12,
    useFlushSync: false,
  });

  const open = (row: MatchRow) => {
    setSelected(row.key);
    onOpenMatch(row.file, row.match);
  };
  const toggle = (path: string) =>
    setCollapsed(current => {
      const next = new Set(current);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });
  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return;
    const matches: { row: MatchRow; index: number }[] = [];
    rows.forEach((row, index) => {
      if (row.kind === "match") matches.push({ row, index });
    });
    if (matches.length === 0) return;
    event.preventDefault();
    const current = matches.findIndex(entry => entry.row.key === selected);
    const delta = event.key === "ArrowDown" ? 1 : -1;
    const nextIndex =
      current < 0
        ? delta > 0
          ? 0
          : matches.length - 1
        : Math.min(Math.max(current + delta, 0), matches.length - 1);
    const next = matches[nextIndex];
    if (!next) return;
    open(next.row);
    virtualizer.scrollToIndex(next.index, { align: "auto" });
  };

  return { scrollRef, rows, selected, virtualizer, open, toggle, onKeyDown };
}
