import { useNavigate } from "@tanstack/react-router";

import { isRemoteComputerId, type ComputerId } from "@/lib/computer-scope";
import { useCurrentComputerId } from "@/hooks/use-current-computer-id";
import { desktopShellBridge, type OpenInEditorResult } from "../lib/desktop-shell-bridge";
import { EDITOR_SETTINGS_PATH, reportOpenInEditorResult } from "../lib/open-in-editor-report";

/**
 * Opens a local file in the operator's editor through the desktop shell. `undefined` on
 * browser and remote sessions/computers (US-012.AC-2, UT-090) and on shells without the
 * `files` bridge, so the page hides it.
 */
export function useOpenFileInEditor(computerId?: ComputerId): ((path: string) => void) | undefined {
  const navigate = useNavigate();
  const effectiveComputerId = useCurrentComputerId(computerId);
  const files = desktopShellBridge()?.files;
  if (isRemoteComputerId(effectiveComputerId) || !files) return undefined;
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
