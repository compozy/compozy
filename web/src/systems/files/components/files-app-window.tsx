import {
  Button,
  CodeHighlightPoolProvider,
  Empty,
  SkeletonRegion,
  SkeletonRows,
  useTopbarSlot,
} from "@compozy/ui";
import { Archive, FolderTree, FolderX, Plus } from "@compozy/ui/icons";
import { createCodeHighlightWorker } from "@compozy/ui/lib/code-highlight-worker";

import { useFilesAppWindow } from "../hooks/use-files-app-window";
import { FilesProjectSwitcher, FilesRootPicker } from "./files-app-pickers";
import { FilesExplorer } from "./files-explorer";

export interface FilesAppWindowProps {
  /** The window's project; one Files window per project (US-002.AC-5). */
  workspaceId: string;
  /** Re-roots this window at another project. */
  onSwitchProject: (workspaceId: string) => void;
}

const FILES_GLYPH = <FolderTree />;

/** `web/src/http.go` → `http.go`; a trailing separator never leaves an empty name. */
function fileBaseName(path: string): string {
  return path.split(/[\\/]/).filter(Boolean).at(-1) ?? path;
}

/** The Files OS app (`_uiux.md` S6): project and root pickers over the shared explorer. */
export function FilesAppWindow({ workspaceId, onSwitchProject }: FilesAppWindowProps) {
  const { status, projects, workspaceName, rootOptions, place, target, openFile } =
    useFilesAppWindow(workspaceId);

  // The head keeps the app's identity ("Files"); only the tab names the open file, else the project.
  const named = status === "ready" || status === "archived";
  useTopbarSlot({
    glyph: FILES_GLYPH,
    tabLabel: named ? (target ? fileBaseName(target.path) : workspaceName) : undefined,
    // The project and root follow the app identity in the head row.
    context: named ? (
      <div className="flex min-w-0 items-center gap-1.5" data-testid="files-app-pickers">
        <FilesProjectSwitcher
          workspaceId={workspaceId}
          workspaceName={workspaceName}
          projects={projects}
          onSwitch={onSwitchProject}
        />
        {status === "ready" ? (
          <FilesRootPicker
            options={rootOptions}
            worktreeId={place.worktreeId}
            onSelect={place.selectRoot}
          />
        ) : null}
      </div>
    ) : undefined,
  });

  if (status === "loading") {
    return (
      <SkeletonRegion label="Loading files" className="p-3" data-testid="files-app-loading">
        <SkeletonRows count={8} />
      </SkeletonRegion>
    );
  }
  if (status === "archived") {
    return (
      <Empty
        icon={Archive}
        title="Project archived"
        description="Unarchive it to browse its files again."
        data-testid="files-app-archived"
      />
    );
  }
  if (status === "missing") {
    return (
      <Empty
        icon={FolderX}
        title="Project not found"
        description="It was removed from CompozyOS."
        data-testid="files-app-missing"
      />
    );
  }
  const root = {
    kind: "workspace" as const,
    workspaceId,
    ...(place.worktreeId ? { worktreeId: place.worktreeId } : {}),
  };
  return (
    <CodeHighlightPoolProvider workerFactory={createCodeHighlightWorker}>
      <div className="flex h-full min-h-0 flex-col" data-testid="files-app">
        <FilesExplorer
          key={place.worktreeId ?? ""}
          root={root}
          target={target}
          onOpenFile={openFile}
          unavailableAction={
            <Button
              type="button"
              variant="outline"
              size="sm"
              data-testid="files-switch-to-project-root"
              onClick={() => place.selectRoot(null)}
            >
              Switch to project root
            </Button>
          }
        />
      </div>
    </CodeHighlightPoolProvider>
  );
}

/** The Files window with no project to show (US-002.EC-1). */
export function FilesAppNoProjects({ onAddProject }: { onAddProject: () => void }) {
  useTopbarSlot({ glyph: FILES_GLYPH });
  return (
    <Empty
      icon={FolderTree}
      title="No projects yet"
      description="Add a project to browse its files."
      action={
        <Button type="button" size="sm" onClick={onAddProject} data-testid="files-add-project">
          <Plus aria-hidden />
          Add project
        </Button>
      }
      data-testid="files-app-no-projects"
    />
  );
}
