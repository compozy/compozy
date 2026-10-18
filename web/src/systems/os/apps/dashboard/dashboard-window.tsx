import { Home, ServerOff } from "lucide-react";

import { ConnectionIndicator, Empty, useTopbarSlot } from "@compozy/ui";

import { OsNewSessionButton } from "../../components/os-new-session-button";

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
    // Global scope has no project to start in, so the action stays disabled and
    // says why (the shared shell control).
    actions: (
      <OsNewSessionButton
        hasProject={hasActiveWorkspace}
        busy={isCreating}
        onNewSession={() => openForAgent("")}
        disabledTestId="home-new-session-disabled"
      />
    ),
  });

  if (connectionStatus === "disconnected") {
    return (
      <div className="flex flex-1 items-center justify-center p-8" data-testid="home-error">
        <Empty
          description="Start CompozyOS on this computer to see your agents."
          icon={ServerOff}
          title="CompozyOS isn't running"
        />
      </div>
    );
  }

  return <HomeDashboard liveEnabled={liveEnabled} />;
}
