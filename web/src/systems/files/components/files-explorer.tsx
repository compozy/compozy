import { useState } from "react";

import {
  Button,
  FileTree,
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
  SearchInput,
  SkeletonRows,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";
import { Eye, GitBranch, RefreshCw } from "@compozy/ui/icons";

import type { FilesRootRef } from "../adapters/files-api";
import { useFileTreeBridge, type FileTreeBridge } from "../hooks/use-file-tree-bridge";
import type { FileViewerTarget } from "../lib/file-viewer-target";
import { FileTreeUnavailableState, FileViewerMessageState } from "./file-viewer-states";
import { FileTreeNotes } from "./file-tree-notes";
import { FileViewerPane } from "./file-viewer-pane";

export interface FilesExplorerProps {
  root: FilesRootRef;
  /** What the viewer shows; a file target is also the tree's selection. */
  target: FileViewerTarget | null;
  /** The operator picked a file in the tree. */
  onOpenFile: (path: string) => void;
  /** Accessible name of the tree. */
  treeLabel?: string;
}

function TreeIconAction({
  label,
  pressed,
  onClick,
  children,
  testId,
}: {
  label: string;
  pressed?: boolean;
  onClick: () => void;
  children: React.ReactNode;
  testId: string;
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            type="button"
            variant="quiet"
            size="icon-xs"
            aria-label={label}
            aria-pressed={pressed}
            data-testid={testId}
            onClick={onClick}
          />
        }
      >
        {children}
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}

function TreeHeader({
  bridge,
  filter,
  onFilterChange,
  showIgnored,
  onShowIgnoredChange,
}: {
  bridge: FileTreeBridge;
  filter: string;
  onFilterChange: (next: string) => void;
  showIgnored: boolean;
  onShowIgnoredChange: (next: boolean) => void;
}) {
  const root = bridge.root.status === "ready" ? bridge.root.root : null;
  return (
    <div className="flex shrink-0 flex-col gap-1.5 border-b border-line-soft px-2 py-2">
      <div className="flex min-w-0 items-center gap-1.5">
        <p
          className="min-w-0 flex-1 truncate text-xs font-medium text-fg"
          data-testid="files-root-name"
        >
          {root?.name ?? ""}
          {root?.branch ? (
            <span className="ml-1.5 inline-flex items-center gap-1 font-normal text-muted-foreground">
              <GitBranch aria-hidden className="size-3" />
              {root.branch}
            </span>
          ) : null}
        </p>
        <TreeIconAction
          label="Show ignored"
          testId="files-show-ignored"
          pressed={showIgnored}
          onClick={() => onShowIgnoredChange(!showIgnored)}
        >
          <Eye aria-hidden className="size-3.5" />
        </TreeIconAction>
        <TreeIconAction label="Refresh" testId="files-refresh" onClick={bridge.refresh}>
          <RefreshCw aria-hidden className="size-3.5" />
        </TreeIconAction>
      </div>
      <SearchInput
        aria-label="Filter files"
        placeholder="Filter files…"
        value={filter}
        onChange={onFilterChange}
        data-testid="files-filter"
      />
    </div>
  );
}

function TreeBody({
  bridge,
  filter,
  selectedPath,
  onOpenFile,
  treeLabel,
  resetKey,
}: {
  bridge: FileTreeBridge;
  filter: string;
  selectedPath: string | null;
  onOpenFile: (path: string) => void;
  treeLabel: string;
  resetKey: string;
}) {
  switch (bridge.root.status) {
    case "loading":
      return (
        <div className="p-2" data-testid="files-tree-loading" aria-busy>
          <SkeletonRows count={8} />
        </div>
      );
    case "unavailable":
      return <FileTreeUnavailableState />;
    case "error":
      return (
        <FileViewerMessageState
          testId="files-tree-error"
          title="Couldn't list files"
          description={bridge.root.message}
          onRetry={bridge.root.retry}
        />
      );
    case "ready":
      return (
        <>
          <FileTree
            aria-label={treeLabel}
            className="min-h-0 flex-1"
            data-testid="files-tree"
            paths={bridge.paths}
            loadingPaths={bridge.loadingPaths}
            decorations={bridge.decorations}
            selectedPath={selectedPath}
            onSelect={onOpenFile}
            onExpand={bridge.expand}
            filter={filter}
            filterEmpty={
              <p className="px-3 py-2 text-xs text-muted-foreground">No loaded files match</p>
            }
            empty={<p className="px-3 py-2 text-xs text-muted-foreground">This folder is empty</p>}
            resetKey={resetKey}
          />
          <FileTreeNotes bridge={bridge} />
        </>
      );
  }
}

/**
 * The project files explorer (`_uiux.md` S3): a lazy tree on the left and the viewer on the
 * right. Session Review panel's Files tab and the Files app compose the same view over
 * different roots.
 */
export function FilesExplorer({
  root,
  target,
  onOpenFile,
  treeLabel = "Project files",
}: FilesExplorerProps) {
  const [filter, setFilter] = useState("");
  const [showIgnored, setShowIgnored] = useState(true);
  const selectedPath = target?.kind === "file" ? target.path : null;
  const bridge = useFileTreeBridge(root, { showIgnored, revealPath: selectedPath });
  if (bridge.root.status === "unavailable") return <FileTreeUnavailableState />;
  return (
    <ResizablePanelGroup
      orientation="horizontal"
      className="min-h-0 flex-1"
      data-testid="files-explorer"
    >
      <ResizablePanel defaultSize={280} minSize={180} maxSize="60%" style={{ overflow: "hidden" }}>
        <div className="flex h-full min-h-0 flex-col bg-sunken">
          <TreeHeader
            bridge={bridge}
            filter={filter}
            onFilterChange={setFilter}
            showIgnored={showIgnored}
            onShowIgnoredChange={setShowIgnored}
          />
          <TreeBody
            bridge={bridge}
            filter={filter}
            selectedPath={selectedPath}
            onOpenFile={onOpenFile}
            treeLabel={treeLabel}
            resetKey={showIgnored ? "all" : "visible"}
          />
        </div>
      </ResizablePanel>
      <ResizableHandle aria-label="Resize file tree" />
      <ResizablePanel minSize={240} style={{ overflow: "hidden" }}>
        <div className="flex h-full min-h-0 flex-col" data-testid="file-viewer">
          <FileViewerPane
            root={root}
            target={target}
            onRevealInTree={() => setFilter("")}
            onRefreshTree={bridge.refresh}
          />
        </div>
      </ResizablePanel>
    </ResizablePanelGroup>
  );
}
