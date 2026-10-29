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
import { WorkspaceApiError } from "@/systems/workspace/adapters/workspace-api";
import {
  useDeleteWorkspace,
  useSetWorkspaceArchived,
} from "@/systems/workspace/hooks/use-workspaces";
import type { WorkspacePayload } from "@/systems/workspace/types";

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
  const copy = projectActionCopy(action, archived);
  return (
    <>
      <WorkspaceActionMenu workspace={workspace} choose={choose}>
        {children}
      </WorkspaceActionMenu>
      <ConfirmDialog
        open={action !== null}
        onOpenChange={open => {
          if (!open && !pending) setAction(null);
        }}
        title={`${copy.verb} ${workspace.name}?`}
        description={copy.description}
        confirmLabel={`${copy.verb} project`}
        cancelLabel="Cancel"
        tone={copy.tone}
        isPending={pending}
        onConfirm={submit}
        error={
          failure instanceof WorkspaceApiError && failure.code === "workspace_busy"
            ? "Stop active work before archiving this project."
            : failure?.message
        }
        body={<WorkspaceArchiveBlockers blockers={blockers} />}
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

function WorkspaceActionMenu({
  workspace,
  choose,
  children,
}: {
  workspace: WorkspacePayload;
  choose: (action: "archive" | "remove") => void;
  children?: ReactElement;
}) {
  const archived = Boolean(workspace.archived_at);
  const archiveLabel = archived ? "Unarchive project" : "Archive project";
  const ArchiveIcon = archived ? ArchiveRestore : Archive;
  return children ? (
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
  );
}

function WorkspaceArchiveBlockers({ blockers }: { blockers: WorkspaceApiError["blockers"] }) {
  if (blockers.length === 0) return null;
  return (
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
  );
}

function projectActionCopy(action: "archive" | "remove" | null, archived: boolean) {
  if (action === "remove")
    return {
      verb: "Remove",
      tone: "danger" as const,
      description:
        "Stopped session history is deleted and window bindings are removed. Adding this folder again creates a new project ID. Files on disk are untouched.",
    };
  if (archived)
    return {
      verb: "Unarchive",
      tone: "neutral" as const,
      description: "Restore this project to navigation with the same ID, history and layouts.",
    };
  return {
    verb: "Archive",
    tone: "neutral" as const,
    description:
      "Hide this project from navigation and All projects. Sessions, history, layouts, worktrees and configuration are kept.",
  };
}
