import { useState } from "react";

import { FileTree, SearchInput } from "@compozy/ui";

import type { SessionChangeFile } from "../../adapters/session-changes-api";
import { sessionChangeDecorations } from "../../lib/session-changes";

/**
 * The changed-files tree beside the diffs (US-007.AC-2): only changed paths, each with its
 * `+N −M` and status, and a "Filter files…" box. Selecting a file scrolls its diff into view.
 */
export function SessionChangedFilesTree({
  files,
  selectedPath,
  onSelect,
  resetKey,
}: {
  files: readonly SessionChangeFile[];
  selectedPath: string | null;
  onSelect: (path: string) => void;
  /** Scope and base: switching either replaces the tree. */
  resetKey: string;
}) {
  const [filter, setFilter] = useState("");
  const paths = files.map(file => file.path);
  return (
    <div
      className="flex h-full min-h-0 flex-col"
      data-testid="changes-tree"
      data-file-count={files.length}
    >
      <div className="shrink-0 border-b border-line-soft p-2">
        <SearchInput
          aria-label="Filter changed files"
          placeholder="Filter files…"
          value={filter}
          onChange={setFilter}
          data-testid="changes-tree-filter"
        />
      </div>
      <FileTree
        aria-label="Changed files"
        className="min-h-0 flex-1"
        paths={paths}
        initialExpansion="open"
        selectedPath={selectedPath}
        onSelect={onSelect}
        decorations={sessionChangeDecorations(files)}
        filter={filter}
        filterEmpty={<p className="px-3 py-2 text-xs text-muted-foreground">No files match</p>}
        resetKey={resetKey}
      />
    </div>
  );
}
