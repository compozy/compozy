import { useNavigate } from "@tanstack/react-router";
import { Home, ServerOff } from "@compozy/ui/icons";

import { ConnectionIndicator, Empty, useTopbarSlot } from "@compozy/ui";

import { OsNewSessionButton } from "../../components/os-new-session-button";
import { useDesktop } from "../../hooks/use-desktop";

import { useDashboardWindowModel } from "./hooks/use-dashboard-window-model";
import { ActivityPage, HomeDashboard } from "@/systems/dashboard";

const ACTIVITY_PATH = "/activity";

/**
 * Thin shell for the home window: identity, the Live pill, and the one primary
 * action live in the window head; the body is the 7-zone home dashboard.
 */
export function DashboardWindow({ windowId }: { windowId: string }) {
  const onActivity = useDesktop(
    state => state.windows[windowId]?.route?.pathname === ACTIVITY_PATH
  );
  const navigate = useNavigate();
  if (onActivity) return <ActivityPage onBack={() => void navigate({ to: "/" })} />;
  return <DashboardHome windowId={windowId} />;
}

function DashboardHome({ windowId }: { windowId: string }) {
  const navigate = useNavigate();
  const { connectionStatus, hasActiveWorkspace, isCreating, liveEnabled, openForAgent } =
    useDashboardWindowModel(windowId);

  useTopbarSlot({
    glyph: <Home />,
    status: (
      <ConnectionIndicator data-testid="home-connection-indicator" status={connectionStatus} />
    ),
    actions: (
      <OsNewSessionButton
        canStart={hasActiveWorkspace}
        busy={isCreating}
        onNewSession={() => openForAgent("")}
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

  return (
    <HomeDashboard
      liveEnabled={liveEnabled}
      onOpenActivity={() => void navigate({ to: "/activity" })}
    />
  );
}
