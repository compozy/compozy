import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";

/**
 * Sessions modal view-model: shell coordinator for open-session routing and
 * persisted agent-group collapse. Open/close ownership lives in
 * `useDesktopOverlays` (shell-level), not the desktop store.
 */
export function useOsSessionsModal() {
  const { coordinator, store } = useOsShell();
  const collapsedAgentIds = useDesktop(state => state.railCollapsedAgentIds);

  return { coordinator, store, collapsedAgentIds };
}
