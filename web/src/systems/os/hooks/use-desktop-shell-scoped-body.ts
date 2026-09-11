import { useGatewayLoopbackOnly } from "@/systems/gateway";
import {
  useSessionCreateActions,
  useSessionLifecycleActions,
  useSessionListView,
} from "@/systems/session";

import { useProfileAutomationEnablement } from "./use-profile-automation-enablement";

/**
 * Scoped shell actions and signals for the desktop body, in one hook so the
 * body component stays under the component-complexity budget.
 */
type DesktopShellScopedWorkspaceId = NonNullable<
  Parameters<typeof useSessionLifecycleActions>[0]
>["workspaceId"];

export function useDesktopShellScopedBody({
  runtimeWorkspaceId,
}: {
  runtimeWorkspaceId: DesktopShellScopedWorkspaceId;
}) {
  const sessionCreate = useSessionCreateActions();
  const sessionLifecycle = useSessionLifecycleActions({ workspaceId: runtimeWorkspaceId });
  const setAutomationEnabled = useProfileAutomationEnablement();
  // The 403 backstop (US-002): any loopback-only refusal the daemon still
  // returns — stale UI, deep link — lands here as the truthful loopback-only
  // strip, never a generic error toast. It persists until the tier latches
  // `local` again, so a retry cannot silently swallow the explanation.
  const loopbackOnly = useGatewayLoopbackOnly();
  // Scope and order are the operator's, persisted by the daemon; the modal
  // renders them rather than fetching its own.
  const sessionListView = useSessionListView();
  const openNewSession = () => {
    sessionCreate.openForAgent("");
  };
  return { loopbackOnly, openNewSession, sessionLifecycle, sessionListView, setAutomationEnabled };
}
