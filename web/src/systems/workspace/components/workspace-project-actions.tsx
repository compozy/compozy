import { useState, type ReactElement } from "react";
import { Archive, ArchiveRestore, MoreHorizontal, Trash2 } from "lucide-react";
import {
  Button,
  ConfirmDialog,
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@compozy/ui";
import { WorkspaceApiError } from "../adapters/workspace-api";
import { useDeleteWorkspace, useSetWorkspaceArchived } from "../hooks/use-workspaces";
import type { WorkspacePayload } from "../types";

export function WorkspaceProjectActions({
  workspace,
  children,
}: {
  workspace: WorkspacePayload;
  children?: ReactElement;
}) {
  const [action, setAction] = useState<"archive" | "remove" | null>(null);
  const archive = useSetWorkspaceArchived();
  const remove = useDeleteWorkspace();
  const pending = archive.isPending || remove.isPending;
  const failure = action === "remove" ? remove.error : archive.error;
  const blockers = failure instanceof WorkspaceApiError ? failure.blockers : [];
  const archived = Boolean(workspace.archived_at);
  const choose = (next: "archive" | "remove") => {
    archive.reset();
    remove.reset();
    setAction(next);
  };
  const submit = () => {
    const onSuccess = () => setAction(null);
    if (action === "remove") remove.mutate(workspace.id, { onSuccess });
    else archive.mutate({ id: workspace.id, archived: !archived }, { onSuccess });
  };
  const archiveLabel = archived ? "Unarchive project" : "Archive project";
  const ArchiveIcon = archived ? ArchiveRestore : Archive;
  return (
    <>
      {children ? (
        <ContextMenu>
          <ContextMenuTrigger render={children} />
          <ContextMenuContent>
            <ContextMenuItem onClick={() => choose("archive")}>
              <ArchiveIcon />
              {archiveLabel}
            </ContextMenuItem>
            <ContextMenuItem variant="destructive" onClick={() => choose("remove")}>
              <Trash2 />
              Remove project
            </ContextMenuItem>
          </ContextMenuContent>
        </ContextMenu>
      ) : (
        <DropdownMenu>
          <DropdownMenuTrigger
            render={<Button variant="ghost" size="icon" aria-label={`Manage ${workspace.name}`} />}
          >
            <MoreHorizontal />
          </DropdownMenuTrigger>
          <DropdownMenuContent>
            <DropdownMenuItem onClick={() => choose("archive")}>
              <ArchiveIcon />
              {archiveLabel}
            </DropdownMenuItem>
            <DropdownMenuItem variant="destructive" onClick={() => choose("remove")}>
              <Trash2 />
              Remove project
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      )}
      <ConfirmDialog
        open={action !== null}
        onOpenChange={open => {
          if (!open && !pending) setAction(null);
        }}
        title={`${action === "remove" ? "Remove" : archived ? "Unarchive" : "Archive"} ${workspace.name}?`}
        description={
          action === "remove"
            ? "Stopped session history is deleted and window bindings are removed. Adding this folder again creates a new project ID. Files on disk are untouched."
            : archived
              ? "Restore this project to navigation with the same ID, history and layouts."
              : "Hide this project from navigation and All projects. Sessions, history, layouts, worktrees and configuration are kept."
        }
        confirmLabel={action === "remove" ? "Remove project" : archiveLabel}
        cancelLabel="Cancel"
        tone={action === "remove" ? "danger" : "neutral"}
        isPending={pending}
        onConfirm={submit}
        error={
          failure instanceof WorkspaceApiError && failure.code === "workspace_busy"
            ? "Stop active work before archiving this project."
            : failure?.message
        }
        body={
          blockers.length > 0 ? (
            <div role="alert">
              <p>Stop these sessions and Loops, or disable these automations, then try again.</p>
              <ul className="mt-2 space-y-1">
                {blockers.map(blocker => (
                  <li key={`${blocker.kind}:${blocker.id}`}>
                    {blocker.kind}: {blocker.name || blocker.id}
                  </li>
                ))}
              </ul>
            </div>
          ) : undefined
        }
        footNote={
          action === "remove" && !archived ? (
            <Button variant="ghost" disabled={pending} onClick={() => choose("archive")}>
              Archive instead
            </Button>
          ) : undefined
        }
      />
    </>
  );
}
