import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import {
  desktopShellBridge,
  lastOpenTarget,
  rememberOpenTarget,
  reportOpenInEditorResult,
  EDITOR_SETTINGS_PATH,
  type OpenTarget,
} from "@/systems/os";
import { sessionDetailOptions } from "@/systems/session";
import { workspaceDetailOptions, worktreesListOptions } from "@/systems/workspace";

import type { FilesRootRef } from "../adapters/files-api";
import { filesKeys } from "../lib/query-keys";
import { openTargetsOptions, primaryOpenTarget } from "../lib/query-options";

export interface OpenInRequest {
  /** Root-relative (or absolute, for an outside link) path; empty for the root itself. */
  path: string;
  line?: number;
  directory?: boolean;
}

export interface OpenIn {
  targets: OpenTarget[];
  /** The split button's primary: the last-used target, else the first editor, else reveal. */
  primary: OpenTarget | undefined;
  busy: boolean;
  open(target: OpenTarget, request: OpenInRequest): Promise<void>;
  /** Re-detects the targets ("Refresh list"). */
  refresh(): void;
  openSettings(): void;
}

async function rootPaths(queryClient: ReturnType<typeof useQueryClient>, root: FilesRootRef) {
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
  return {
    base: worktree?.path ?? workspace.workspace.root_dir,
    project: workspace.workspace.root_dir,
  };
}

function absolutePath(base: string, path: string): string {
  if (/^(\/|[A-Za-z]:[\\/]|~\/)/.test(path)) return path;
  const trimmed = base.replace(/[\\/]$/, "");
  const relative = path.replace(/\/$/, "");
  return relative === "" ? trimmed : `${trimmed}/${relative}`;
}

/**
 * The desktop Open in targets for one root (US-006). The renderer sends a target id and a
 * path; the shell owns detection, confirmation, and launching.
 */
export function useOpenIn(root: FilesRootRef): OpenIn {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [selected, setSelected] = useState(lastOpenTarget);
  const [busy, setBusy] = useState(false);
  const { data: targets = [] } = useQuery(openTargetsOptions());
  const refresh = () => void queryClient.invalidateQueries({ queryKey: filesKeys.openTargets() });
  const openSettings = () => void navigate({ to: EDITOR_SETTINGS_PATH });

  async function open(target: OpenTarget, request: OpenInRequest) {
    const openWith = desktopShellBridge()?.files?.openWith;
    if (busy || !openWith) return;
    setBusy(true);
    let shown = request.path;
    try {
      const { base, project } = await rootPaths(queryClient, root);
      const path = absolutePath(base, request.path);
      shown = path;
      const withLine = !request.directory && target.supportsLine;
      const result = await openWith({
        target: target.id,
        path,
        ...(withLine ? { line: request.line ?? 1, column: 1 } : {}),
        roots: [base, project],
      });
      if (result.status === "opened") {
        rememberOpenTarget(target.id);
        setSelected(target.id);
      }
      // The editor went away since detection (US-006.EC-1): the list must not offer it again.
      if (result.status === "error" && result.code === "target_unavailable") refresh();
      reportOpenInEditorResult(result, path, openSettings);
    } catch {
      reportOpenInEditorResult(
        { status: "error", code: "editor_failed", message: "Couldn't open the file. Try again." },
        shown,
        openSettings
      );
    } finally {
      setBusy(false);
    }
  }

  return {
    targets,
    primary: primaryOpenTarget(targets, selected),
    busy,
    open,
    refresh,
    openSettings,
  };
}
