import type { FileTreeBridge } from "../hooks/use-file-tree-bridge";

function folderLabel(path: string): string {
  return path === "" ? "This folder" : path;
}

/**
 * Quiet notes under the tree for what the rows themselves cannot carry: capped listings
 * (US-001.EC-3), expanded empty folders (EC-4), and a git status that timed out (EC-7).
 */
export function FileTreeNotes({ bridge }: { bridge: FileTreeBridge }) {
  const statusUnavailable = bridge.root.status === "ready" && bridge.root.statusUnavailable;
  if (bridge.truncated.length === 0 && bridge.empty.length === 0 && !statusUnavailable) return null;
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
      {statusUnavailable ? <li>Git status unavailable</li> : null}
    </ul>
  );
}
