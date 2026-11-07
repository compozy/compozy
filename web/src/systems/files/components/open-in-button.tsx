import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState, type ReactNode } from "react";
import {
  Button,
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@compozy/ui";
import { ChevronDown } from "@compozy/ui/icons";
import {
  desktopShellBridge,
  lastOpenTarget,
  rememberOpenTarget,
  reportOpenInEditorResult,
  EDITOR_SETTINGS_PATH,
} from "@/systems/os";
import { workspaceDetailOptions, worktreesListOptions } from "@/systems/workspace";
import { sessionDetailOptions } from "@/systems/session";
import { openTargetsOptions } from "../lib/query-options";
import type { FilesRootRef } from "../adapters/files-api";

export function OpenInButton({
  root,
  path,
  line,
  children,
}: {
  root?: FilesRootRef;
  path?: string;
  line?: number;
  children?: ReactNode;
}) {
  const files = desktopShellBridge()?.files;
  if (!files?.listOpenTargets) return null;
  if (!root || !path) return <>{children}</>;
  if (!files.openWith) return null;
  return <OpenTargetButton root={root} path={path} line={line} />;
}

function OpenTargetButton({
  root,
  path,
  line,
}: {
  root: FilesRootRef;
  path: string;
  line?: number;
}) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [selected, setSelected] = useState(lastOpenTarget);
  const [busy, setBusy] = useState(false);
  const { data: targets = [] } = useQuery(openTargetsOptions());
  const current = targets.find(target => target.id === selected) ?? targets[0];
  async function open(id: string) {
    if (busy) return;
    setBusy(true);
    try {
      const workspace = await queryClient.ensureQueryData(workspaceDetailOptions(root.workspaceId));
      let worktreeId = root.kind === "workspace" ? root.worktreeId : undefined;
      if (root.kind === "session")
        worktreeId =
          (await queryClient.fetchQuery(sessionDetailOptions(root.workspaceId, root.sessionId)))
            .worktree_id ?? undefined;
      const worktrees = worktreeId
        ? await queryClient.ensureQueryData(worktreesListOptions(root.workspaceId))
        : undefined;
      const worktree = worktrees?.worktrees.find(item => item.id === worktreeId);
      if (worktreeId && !worktree) throw new Error("The worktree is unavailable.");
      const base = worktree?.path ?? workspace.workspace.root_dir;
      const absolute = /^(\/|[A-Za-z]:[\\/]|~\/)/.test(path)
        ? path
        : `${base.replace(/[\\/]$/, "")}/${path}`;
      const result = await desktopShellBridge()!.files!.openWith!({
        target: id,
        path: absolute,
        line: line ?? 1,
        column: 1,
        roots: [base, workspace.workspace.root_dir],
      });
      if (result.status === "opened") {
        rememberOpenTarget(id);
        setSelected(id);
      }
      reportOpenInEditorResult(result, absolute, () => void navigate({ to: EDITOR_SETTINGS_PATH }));
    } catch {
      reportOpenInEditorResult(
        { status: "error", code: "editor_failed", message: "Couldn't open the file. Try again." },
        path,
        () => void navigate({ to: EDITOR_SETTINGS_PATH })
      );
    } finally {
      setBusy(false);
    }
  }
  if (!current) return null;
  return (
    <div className="flex items-center">
      <Button size="sm" variant="outline" disabled={busy} onClick={() => void open(current.id)}>
        Open in {current.label}
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button size="sm" variant="outline" aria-label="Choose application" disabled={busy} />
          }
        >
          <ChevronDown />
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          {targets.map(target => (
            <DropdownMenuItem key={target.id} onClick={() => void open(target.id)}>
              {target.label}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
