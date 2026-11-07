import { useQueryClient } from "@tanstack/react-query";

import { desktopShellBridge } from "@/systems/os";
import { runtimeIdentityOptions } from "@/systems/status";

import { readFilePathResolutionBase } from "../lib/file-path-resolution-base";
import type { FilePathToken } from "../lib/file-path-links";
import { openFilePathInViewer } from "../lib/file-path-viewer-route";
import { sessionHasFileRoot } from "../lib/session-file-root";
import {
  requestSessionReviewChanges,
  requestSessionReviewFile,
} from "./use-session-review-panel-state";
import { useOpenInEditor, type OpenInEditor } from "./use-open-in-editor";
import { useSessionRuntimeRenderContext } from "./use-session-runtime-render-context";

export interface FilePathLinks {
  /** Windows desktop: drive paths name a file and `\` separates segments. */
  windows: boolean;
  /** Plain click: opens the session's Review panel on Files. `null` without a file root. */
  openInViewer: ((token: FilePathToken) => void) | null;
  /** Desktop Cmd/Ctrl-click: the editor open action (#1167). `null` off desktop. */
  editor: OpenInEditor | null;
  /**
   * A turn's changed-file row: opens Changes focused on the file (US-005.AC-6), or the viewer
   * when it lies outside the root. `null` without a file root.
   */
  openChange: ((path: string) => void) | null;
}

/** A changed-file row's path as a link token: tool paths are absolute or session-relative. */
function changedFileToken(path: string): FilePathToken {
  return { raw: path, path, relative: !path.startsWith("/") && !path.startsWith("~/") };
}

/**
 * The transcript's path affordances for this session (US-005): the viewer on every client
 * with a session root, plus Cmd/Ctrl-click to the editor in the desktop app. `null` when
 * neither applies, so paths render as plain code.
 */
export function useFilePathLinks(): FilePathLinks | null {
  const context = useSessionRuntimeRenderContext();
  const queryClient = useQueryClient();
  const editor = useOpenInEditor();
  const hasRoot = context !== null && sessionHasFileRoot(context.workspaceId);
  const windows = desktopShellBridge()?.platform === "win32";
  if (!context || (!hasRoot && !editor)) return null;
  const scope = { sessionId: context.sessionId, workspaceId: context.workspaceId };
  const resolveRoot = async (token: FilePathToken) => {
    const { base } = await readFilePathResolutionBase(queryClient, scope);
    const root = base.worktreePath ?? base.workspaceRoot;
    if (!root) return null;
    // Only a `~/` path needs the daemon user's home directory.
    const home = token.path.startsWith("~/")
      ? await queryClient.ensureQueryData(runtimeIdentityOptions()).then(
          identity => identity.daemon.user_home_dir,
          () => undefined
        )
      : undefined;
    return { root, workingDir: base.workingDir, home };
  };
  return {
    windows,
    editor,
    openInViewer: hasRoot
      ? token =>
          void openFilePathInViewer(
            token,
            {
              sessionId: scope.sessionId,
              resolveRoot: () => resolveRoot(token),
              open: target => requestSessionReviewFile(scope.sessionId, target),
            },
            { windows }
          )
      : null,
    openChange: hasRoot
      ? path => {
          const token = changedFileToken(path);
          void openFilePathInViewer(
            token,
            {
              sessionId: scope.sessionId,
              resolveRoot: () => resolveRoot(token),
              open: target =>
                target.kind === "file"
                  ? requestSessionReviewChanges(scope.sessionId, { path: target.path })
                  : requestSessionReviewFile(scope.sessionId, target),
            },
            { windows }
          );
        }
      : null,
  };
}
