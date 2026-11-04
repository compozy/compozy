import { useNavigate } from "@tanstack/react-router";

import { EDITOR_SETTINGS_PATH, reportOpenInEditorResult } from "@/systems/session";

import { desktopShellBridge, type OpenInEditorResult } from "../../lib/desktop-shell-bridge";

/**
 * Opens a check-in script in the operator's editor through the desktop shell. `undefined` on
 * browser and remote clients and on shells without the `files` bridge, so the page hides it.
 */
export function useOpenScript(): ((path: string) => void) | undefined {
  const navigate = useNavigate();
  const files = desktopShellBridge()?.files;
  if (!files) return undefined;
  return path => {
    const folder = path.slice(0, Math.max(path.lastIndexOf("/"), path.lastIndexOf("\\")));
    void files
      .openInEditor({ path, roots: folder ? [folder] : [] })
      .catch((): OpenInEditorResult => ({
        status: "error",
        code: "editor_failed",
        message: "Couldn't open the editor. Check the editor command in Settings.",
      }))
      .then(result =>
        reportOpenInEditorResult(result, path, () => void navigate({ to: EDITOR_SETTINGS_PATH }))
      );
  };
}
