import { Home, Plus, ServerOff } from "lucide-react";

import { Button, ConnectionIndicator, Empty, useTopbarSlot } from "@agh/ui";

import { HomeDashboard } from "@/systems/dashboard";
import { useSessionCreate } from "@/systems/session";
import { useDaemonHealth } from "@/systems/status";

/**
 * Thin shell for the home window: identity, the Live pill, and the one primary
 * action live in the window head; the body is the 7-zone home dashboard.
 */
export function DashboardWindow(_props: { windowId: string }) {
  const { connectionStatus } = useDaemonHealth();
  const sessionCreate = useSessionCreate();

  useTopbarSlot({
    glyph: <Home />,
    status: (
      <ConnectionIndicator data-testid="home-connection-indicator" status={connectionStatus} />
    ),
    actions: (
      <Button
        disabled={!sessionCreate.hasActiveWorkspace || sessionCreate.isCreating}
        onClick={() => sessionCreate.openForAgent("")}
        size="sm"
        variant="primary"
      >
        <Plus aria-hidden="true" />
        New session
      </Button>
    ),
  });

  if (connectionStatus === "disconnected") {
    return (
      <div className="flex flex-1 items-center justify-center p-8" data-testid="home-error">
        <Empty
          description="Start the daemon to see what your agents are doing."
          icon={ServerOff}
          title={<ConnectionIndicator status="disconnected" />}
        />
      </div>
    );
  }

  return <HomeDashboard />;
}
