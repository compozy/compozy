import { notifyUser } from "@/lib/user-feedback";

import type { OpenInEditorResult } from "./desktop-shell-bridge";

/** Where the editor command lives, for the toast's Open Settings action. */
export const EDITOR_SETTINGS_PATH = "/settings/general";

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
