import { Home, Plus, ServerOff } from "lucide-react";

import {
  Button,
  ConnectionIndicator,
  Empty,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  useTopbarSlot,
} from "@compozy/ui";

import { useDashboardWindowModel } from "./hooks/use-dashboard-window-model";
import { HomeDashboard } from "@/systems/dashboard";

/**
 * Thin shell for the home window: identity, the Live pill, and the one primary
 * action live in the window head; the body is the 7-zone home dashboard.
 */
export function DashboardWindow({ windowId }: { windowId: string }) {
  const { connectionStatus, hasActiveWorkspace, isCreating, liveEnabled, openForAgent } =
    useDashboardWindowModel(windowId);

  const newSession = (
    <Button
      disabled={!hasActiveWorkspace || isCreating}
      onClick={() => openForAgent("")}
      variant="primary"
    >
      <Plus aria-hidden="true" />
      New session
    </Button>
  );

  useTopbarSlot({
    glyph: <Home />,
    status: (
      <ConnectionIndicator data-testid="home-connection-indicator" status={connectionStatus} />
    ),
    // Global scope has no project to start in, so the action stays disabled and
    // says why; a disabled button cannot take focus, so the wrapper does.
    actions: hasActiveWorkspace ? (
      newSession
    ) : (
      <Tooltip>
        <TooltipTrigger
          render={
            <span
              aria-label="New session — pick a project to start a session"
              className="inline-flex rounded-pill outline-none focus-visible:shadow-focus-ring"
              data-testid="home-new-session-disabled"
              tabIndex={0}
            />
          }
        >
          {newSession}
        </TooltipTrigger>
        <TooltipContent>Pick a project to start a session</TooltipContent>
      </Tooltip>
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
