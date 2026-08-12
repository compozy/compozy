import { useWindowLiveDataEnabled } from "../../../hooks/use-window-live-data-enabled";
import {
  useSessionCreateActions,
  useSessionCreateHasActiveWorkspace,
  useSessionCreateIsCreating,
} from "@/systems/session";
import { useDaemonHealth } from "@/systems/status";

export function useDashboardWindowModel(windowId: string) {
  const { connectionStatus } = useDaemonHealth();
  const { openForAgent } = useSessionCreateActions();
  const hasActiveWorkspace = useSessionCreateHasActiveWorkspace();
  const isCreating = useSessionCreateIsCreating();
  const liveEnabled = useWindowLiveDataEnabled(windowId);

  return {
    connectionStatus,
    hasActiveWorkspace,
    isCreating,
    liveEnabled,
    openForAgent,
  };
}
