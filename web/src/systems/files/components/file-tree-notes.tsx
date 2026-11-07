import { Button } from "@compozy/ui";

import type { FileTreeBridge } from "../hooks/use-file-tree-bridge";

function folderLabel(path: string): string {
  return path === "" ? "This folder" : path;
}

/**
 * Quiet notes under the tree for what the rows themselves cannot carry: capped listings
 * (US-001.EC-3), expanded empty folders (EC-4), folders that failed to list (with Retry), and a
 * git status that timed out (EC-7).
 */
export function FileTreeNotes({ bridge }: { bridge: FileTreeBridge }) {
  const statusUnavailable = bridge.root.status === "ready" && bridge.root.statusUnavailable;
  const quiet =
    bridge.truncated.length === 0 && bridge.empty.length === 0 && bridge.failed.length === 0;
  if (quiet && !statusUnavailable) return null;
  return (
    <ul
      className="flex shrink-0 flex-col gap-0.5 border-t border-line-soft px-3 py-1.5 text-xs text-muted-foreground"
      data-testid="files-tree-notes"
    >
      {bridge.truncated.map(path => (
        <li key={`truncated:${path}`}>{folderLabel(path)}: Showing 5,000 of many entries</li>
      ))}
      {bridge.empty.map(path => (
        <li key={`empty:${path}`}>{path}: Empty folder</li>
      ))}
      {bridge.failed.map(failure => (
        <li
          key={`failed:${failure.path}`}
          className="flex items-center gap-1.5 text-destructive"
          data-testid="files-tree-folder-error"
          title={failure.message}
        >
          <span className="min-w-0 truncate">{failure.path}: Couldn't load this folder</span>
          <Button type="button" variant="link" size="xs" onClick={failure.retry}>
            Retry
          </Button>
        </li>
      ))}
      {statusUnavailable ? <li>Git status unavailable</li> : null}
    </ul>
  );
}
