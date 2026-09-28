import { settingsUpdateTracks, settingsUpdateView } from "@/systems/settings";
import type { SettingsUpdateStatus } from "@/systems/settings";

interface GeneralUpdateAttentionInput {
  data?: SettingsUpdateStatus;
  error: unknown;
  isError: boolean;
  isLoading: boolean;
}

/**
 * True when the Updates group carries something to act on — a failed check, a
 * failed refresh, or a track that is not settled — so the page can lead with it.
 */
export function generalUpdateNeedsAttention(props: GeneralUpdateAttentionInput): boolean {
  const view = settingsUpdateView(props);
  if (view.kind === "error") return true;
  if (view.kind !== "snapshot") return false;
  if (view.refreshError) return true;
  return settingsUpdateTracks(view.snapshot).some(
    track => track.tone !== "success" && track.tone !== "neutral"
  );
}
