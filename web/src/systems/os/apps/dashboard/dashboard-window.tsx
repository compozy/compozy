import { Home, Plus, ServerOff } from "lucide-react";

import { Button, ConnectionIndicator, Empty, useTopbarSlot } from "@compozy/ui";

import { useDashboardWindowModel } from "./hooks/use-dashboard-window-model";
import { HomeDashboard } from "@/systems/dashboard";

/**
 * Thin shell for the home window: identity, the Live pill, and the one primary
 * action live in the window head; the body is the 7-zone home dashboard.
 */
export function DashboardWindow({ windowId }: { windowId: string }) {
  const { connectionStatus, hasActiveWorkspace, isCreating, liveEnabled, openForAgent } =
    useDashboardWindowModel(windowId);

  useTopbarSlot({
    glyph: <Home />,
    status: (
      <ConnectionIndicator data-testid="home-connection-indicator" status={connectionStatus} />
    ),
    actions: (
      <Button
        disabled={!hasActiveWorkspace || isCreating}
        onClick={() => openForAgent("")}
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
          description="Start CompozyOS to see what your agents are doing."
          icon={ServerOff}
          title={<ConnectionIndicator status="disconnected" />}
        />
      </div>
    );
  }

  return <HomeDashboard liveEnabled={liveEnabled} />;
}
