import { useQueryClient, type QueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import { notifyUser } from "@/lib/user-feedback";
import {
  desktopShellBridge,
  EDITOR_SETTINGS_PATH,
  reportOpenInEditorResult,
  type OpenInEditorResult,
} from "@/systems/os";
import { workspaceDetailOptions, worktreesListOptions } from "@/systems/workspace";

import { resolveFilePath, type FilePathBase, type FilePathToken } from "../lib/file-path-links";
import { sessionKeys } from "../lib/query-keys";
import { sessionDetailOptions } from "../lib/query-options";
import type { SessionPayload } from "../types";
import { useSessionRuntimeRenderContext } from "./use-session-runtime-render-context";

export interface OpenInEditor {
  /** Resolves `token` at click time, asks the desktop shell to open it, and reports failures. */
  open(token: FilePathToken): Promise<void>;
  /** Cmd on macOS, Ctrl elsewhere — the modifier that opens on click. */
  modifier: "meta" | "ctrl";
  /** Windows desktop: drive paths (`C:\x`) name a file and `\` separates segments. */
  windows: boolean;
}

interface SessionScope {
  sessionId: string;
  workspaceId: string;
}

// An invalidated entry (e.g. after a worktree change) may name a stale `working_dir`, so only
// valid entries count; with none, the click rereads the detail.
function cachedSessionDetail(
  queryClient: QueryClient,
  { sessionId, workspaceId }: SessionScope
): SessionPayload | undefined {
  const cache = queryClient.getQueryCache();
  const detail = cache.find({ queryKey: sessionKeys.detail(workspaceId, sessionId), exact: true });
  const queries = cache.findAll({ queryKey: sessionKeys.byIdRoot(sessionId) });
  if (detail) queries.push(detail);
  const candidates: SessionPayload[] = [];
  for (const query of queries) {
    const data = query.state.data as SessionPayload | undefined;
    if (!query.state.isInvalidated && data !== undefined) candidates.push(data);
  }
  return candidates.find(data => data.working_dir) ?? candidates[0];
}

// Detail-cache writers (rename, live-tail) may replace the payload without `working_dir`, so the
// click reads it fresh whenever the cached detail lacks it (spec Data Models, web cache rule).
async function readSessionDetail(
  queryClient: QueryClient,
  scope: SessionScope
): Promise<SessionPayload | undefined> {
  const cached = cachedSessionDetail(queryClient, scope);
  if (cached?.working_dir) return cached;
  try {
    return await queryClient.fetchQuery({
      ...sessionDetailOptions(scope.workspaceId, scope.sessionId),
      staleTime: 0,
    });
  } catch {
    return cached;
  }
}

async function readOptional<T>(read: () => Promise<T>): Promise<T | undefined> {
  try {
    return await read();
  } catch {
    return undefined;
  }
}

async function readResolutionBase(queryClient: QueryClient, scope: SessionScope) {
  const session = await readSessionDetail(queryClient, scope);
  const worktreeId = session?.worktree_id?.trim() ?? "";
  const [workspace, worktrees] = await Promise.all([
    readOptional(() => queryClient.ensureQueryData(workspaceDetailOptions(scope.workspaceId))),
    worktreeId === ""
      ? Promise.resolve(undefined)
      : readOptional(() => queryClient.ensureQueryData(worktreesListOptions(scope.workspaceId))),
  ]);
  const base: FilePathBase = {
    workingDir: session?.working_dir,
    worktreePath: worktrees?.worktrees.find(entry => entry.id === worktreeId)?.path,
    workspaceRoot: workspace?.workspace.root_dir,
  };
  const roots = [base.worktreePath, base.workspaceRoot]
    .map(root => root?.trim() ?? "")
    .filter((root, index, all) => root !== "" && all.indexOf(root) === index);
  return { base, roots };
}

/**
 * The desktop-only open action for this session's transcript. `null` on browser and remote
 * clients, on desktop shells without the `files` bridge, and outside a session transcript.
 */
export function useOpenInEditor(): OpenInEditor | null {
  const context = useSessionRuntimeRenderContext();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const bridge = desktopShellBridge();
  const files = bridge?.files;
  if (!files || !context?.sessionId || !context.workspaceId) return null;
  const scope = { sessionId: context.sessionId, workspaceId: context.workspaceId };

  return {
    modifier: bridge.platform === "darwin" ? "meta" : "ctrl",
    windows: bridge.platform === "win32",
    async open(token) {
      const openSettings = () => void navigate({ to: EDITOR_SETTINGS_PATH });
      const { base, roots } = await readResolutionBase(queryClient, scope);
      const path = resolveFilePath(token, base, { windows: bridge.platform === "win32" });
      if (path === null) {
        notifyUser({ tone: "error", message: `File not found: ${token.path}` });
        return;
      }
      let result: OpenInEditorResult;
      try {
        result = await files.openInEditor({
          path,
          ...(token.line !== undefined ? { line: token.line } : {}),
          ...(token.column !== undefined ? { column: token.column } : {}),
          roots,
        });
      } catch {
        result = {
          status: "error",
          code: "editor_failed",
          message: "Couldn't open the editor. Check the editor command in Settings.",
        };
      }
      reportOpenInEditorResult(result, path, openSettings);
    },
  };
}
