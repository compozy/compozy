import { resetSettingsRestartStore } from "@/systems/settings/stores/use-settings-restart-store";
import { sessionStore } from "@/systems/session/stores/session-store";
import { clearActiveWorkspaceSelection } from "@/systems/workspace";

/**
 * Resets the module-level client stores views read outside React state (workspace selection,
 * session drafts, pending settings restart), so a story or prototype canvas never inherits the
 * previous one's edits. Storybook and the prototype gallery share it.
 */
export function resetClientStores() {
  clearActiveWorkspaceSelection();
  sessionStore.trigger.allDraftsDiscarded();
  resetSettingsRestartStore();
}
