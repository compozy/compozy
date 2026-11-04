import { useScriptApprovals } from "@/systems/agent";

import type { DesktopOverlay } from "./use-desktop-overlays";
import { useMenubarAttentionSelection } from "./use-menubar-attention-selection";

/** What the bell acts on: notification rows to focus, and scripts that wait for approval. */
export function useMenubarAttention(
  onOverlayOpenChange: (overlay: DesktopOverlay, open: boolean) => void
) {
  return {
    focusAttentionRow: useMenubarAttentionSelection(onOverlayOpenChange),
    scriptApprovals: useScriptApprovals(),
  };
}
