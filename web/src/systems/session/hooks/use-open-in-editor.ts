import { useQueryClient, type QueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import { notifyUser } from "@/lib/user-feedback";
import { desktopShellBridge, type OpenInEditorResult } from "@/systems/os";
import { workspaceDetailOptions, worktreesListOptions } from "@/systems/workspace";

import { resolveFilePath, type FilePathBase, type FilePathToken } from "../lib/file-path-links";
import { sessionKeys } from "../lib/query-keys";
import { sessionDetailOptions } from "../lib/query-options";
import type { SessionPayload } from "../types";
import { useSessionRuntimeRenderContext } from "./use-session-runtime-render-context";

export const EDITOR_SETTINGS_PATH = "/settings/general";

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

function cachedSessionDetail(
  queryClient: QueryClient,
  { sessionId, workspaceId }: SessionScope
): SessionPayload | undefined {
  const scoped = queryClient
    .getQueriesData<SessionPayload>({ queryKey: sessionKeys.byIdRoot(sessionId) })
    .map(([, data]) => data)
    .filter(data => data !== undefined);
  const detail = queryClient.getQueryData<SessionPayload>(
    sessionKeys.detail(workspaceId, sessionId)
  );
  const candidates = detail ? [...scoped, detail] : scoped;
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

/** Maps a non-success bridge result to its toast; `opened` and `cancelled` stay silent. */
export function reportOpenInEditorResult(
  result: OpenInEditorResult,
  path: string,
  openSettings: () => void
): void {
  if (result.status !== "error") return;
  const settingsAction = { label: "Open Settings", onClick: openSettings };
  switch (result.code) {
    case "not_found":
      notifyUser({ tone: "error", message: `File not found: ${path}` });
      return;
    case "not_a_file":
      notifyUser({ tone: "error", message: `Not a file: ${path}` });
      return;
    case "editor_failed":
    case "editor_not_configured":
      notifyUser({ tone: "error", message: result.message, action: settingsAction });
      return;
    default:
      notifyUser({ tone: "error", message: result.message });
  }
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
