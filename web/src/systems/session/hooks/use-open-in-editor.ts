import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import { notifyUser } from "@/lib/user-feedback";
import {
  desktopShellBridge,
  EDITOR_SETTINGS_PATH,
  reportOpenInEditorResult,
  type OpenInEditorResult,
} from "@/systems/os";

import { sessionWorkspacePath } from "../adapters/session-workspace-path";
import { readFilePathResolutionBase } from "../lib/file-path-resolution-base";
import { resolveFilePath, type FilePathToken } from "../lib/file-path-links";
import { useSessionRuntimeRenderContext } from "./use-session-runtime-render-context";

export interface OpenInEditor {
  /** Resolves `token` at click time, asks the desktop shell to open it, and reports failures. */
  open(token: FilePathToken): Promise<void>;
  /** Cmd on macOS, Ctrl elsewhere — the modifier that opens on click. */
  modifier: "meta" | "ctrl";
  /** Windows desktop: drive paths (`C:\x`) name a file and `\` separates segments. */
  windows: boolean;
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
  if (!files || !context?.sessionId) return null;
  // A user-scope session reports an empty workspace id; its reads live under `@user` (EC-5).
  const scope = {
    sessionId: context.sessionId,
    workspaceId: sessionWorkspacePath(context.workspaceId),
  };

  return {
    modifier: bridge.platform === "darwin" ? "meta" : "ctrl",
    windows: bridge.platform === "win32",
    async open(token) {
      const openSettings = () => void navigate({ to: EDITOR_SETTINGS_PATH });
      const { base, roots } = await readFilePathResolutionBase(queryClient, scope);
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
