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
import type { FileTreeBridge } from "../hooks/use-file-tree-bridge";
import { useFilesExplorer } from "../hooks/use-files-explorer";
import type { FileViewerTarget, ViewerFindSeed } from "../lib/file-viewer-target";
import { ExplorerPaneSwitch } from "./explorer-pane-switch";
import { FilesSearchPane } from "./files-search-pane";
import { GoToFileBox } from "./go-to-file-box";
import { FileTreeUnavailableState, FileViewerMessageState } from "./file-viewer-states";
import { FileTreeNotes } from "./file-tree-notes";
import { FileViewerPane } from "./file-viewer-pane";
import { FilesTreeRowMenu } from "./files-tree-row-menu";
import { OpenRootInMenu } from "./open-in-button";

export interface FilesExplorerProps {
  root: FilesRootRef;
  /** What the viewer shows; a file target is also the tree's selection. */
  target: FileViewerTarget | null;
  /**
   * The operator opened a file: from the tree, Go to file (`line` from `name:42`), or a
   * Search match (`line` plus the Find in file seed).
   */
  onOpenFile: (path: string, line?: number, find?: ViewerFindSeed) => void;
  /** The way out of a root that is gone (the Files app's "Switch to project root"). */
  unavailableAction?: React.ReactNode;
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
  root: rootRef,
  bridge,
  filter,
  onFilterChange,
  showIgnored,
  onShowIgnoredChange,
}: {
  root: FilesRootRef;
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
        {root ? <OpenRootInMenu root={rootRef} /> : null}
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
  root,
  bridge,
  filter,
  selectedPath,
  revealKey,
  onOpenFile,
  resetKey,
}: {
  root: FilesRootRef;
  bridge: FileTreeBridge;
  filter: string;
  selectedPath: string | null;
  revealKey: string;
  onOpenFile: (path: string) => void;
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
            aria-label="Project files"
            className="min-h-0 flex-1"
            data-testid="files-tree"
            surface="sunken"
            paths={bridge.paths}
            loadingPaths={bridge.loadingPaths}
            decorations={bridge.decorations}
            selectedPath={selectedPath}
            revealKey={revealKey}
            onSelect={onOpenFile}
            onExpand={bridge.expand}
            onExpandedChange={bridge.expandedChanged}
            filter={filter}
            filterEmpty={
              <p className="px-3 py-2 text-xs text-muted-foreground">No loaded files match</p>
            }
            empty={<p className="px-3 py-2 text-xs text-muted-foreground">This folder is empty</p>}
            resetKey={resetKey}
            rowMenu={item => <FilesTreeRowMenu root={root} item={item} />}
          />
          <FileTreeNotes bridge={bridge} />
        </>
      );
  }
}

/**
 * The project files explorer (`_uiux.md` S3): a lazy tree on the left and the viewer on the
 * right, over a project checkout or worktree root (the Files app).
 */
export function FilesExplorer({ root, target, onOpenFile, unavailableAction }: FilesExplorerProps) {
  const explorer = useFilesExplorer(root, target);
  const { bridge, pane } = explorer;
  if (bridge.root.status === "unavailable") {
    return <FileTreeUnavailableState action={unavailableAction} />;
  }
  return (
    <div className="relative flex min-h-0 flex-1 flex-col" onKeyDown={explorer.onKeyDown}>
      <ResizablePanelGroup
        orientation="horizontal"
        className="min-h-0 flex-1"
        data-testid="files-explorer"
      >
        <ResizablePanel
          defaultSize={280}
          minSize={180}
          maxSize="60%"
          style={{ overflow: "hidden" }}
        >
          <div className="flex h-full min-h-0 flex-col bg-sunken">
            <ExplorerPaneSwitch
              pane={pane}
              onPaneChange={explorer.setPane}
              onGoToFile={explorer.openGoToFile}
              primaryKey={explorer.primaryKey}
            />
            {/* Both panes stay mounted, so switching keeps the tree and the last search. */}
            <div className={pane === "files" ? "flex min-h-0 flex-1 flex-col" : "hidden"}>
              <TreeHeader
                root={root}
                bridge={bridge}
                filter={explorer.filter}
                onFilterChange={explorer.setFilter}
                showIgnored={explorer.showIgnored}
                onShowIgnoredChange={explorer.setShowIgnored}
              />
              <TreeBody
                root={root}
                bridge={bridge}
                filter={explorer.filter}
                selectedPath={explorer.selectedPath}
                revealKey={explorer.revealKey}
                onOpenFile={path => onOpenFile(path)}
                resetKey={explorer.showIgnored ? "all" : "visible"}
              />
            </div>
            <div className={pane === "search" ? "flex min-h-0 flex-1 flex-col" : "hidden"}>
              <FilesSearchPane
                root={root}
                focusToken={explorer.searchFocus}
                onOpenMatch={onOpenFile}
              />
            </div>
          </div>
        </ResizablePanel>
        <ResizableHandle aria-label="Resize file tree" />
        <ResizablePanel minSize={240} style={{ overflow: "hidden" }}>
          <div className="flex h-full min-h-0 flex-col" data-testid="file-viewer">
            <FileViewerPane
              root={root}
              target={target}
              onRevealInTree={() => {
                explorer.setFilter("");
                explorer.showFiles();
                explorer.revealSelection();
              }}
              onRefreshTree={bridge.refresh}
            />
          </div>
        </ResizablePanel>
      </ResizablePanelGroup>
      {explorer.goToFileOpen ? (
        <GoToFileBox
          root={root}
          recent={explorer.recent}
          onClose={explorer.closeGoToFile}
          onOpenFile={(path, line) => {
            explorer.showFiles();
            onOpenFile(path, line);
          }}
          onRevealFolder={explorer.revealFolder}
        />
      ) : null}
    </div>
  );
}
