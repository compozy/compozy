import { Link } from "@tanstack/react-router";
import { Home } from "lucide-react";

import { CatalogEmptyState } from "@/components/catalog-empty-state";
import { useSessionCreateActions } from "@/systems/session";
import { Button } from "@compozy/ui";

export interface HomeFirstRunProps {
  workspaceName?: string | null;
}

/**
 * Zero-inventory Home. Replaces the seven zero-filled zones when the overview
 * reports no work at all, offering the three starts that actually exist:
 * a session (dialog), a task (/tasks/new), and the marketplace.
 */
export function HomeFirstRun({ workspaceName }: HomeFirstRunProps) {
  const sessionCreate = useSessionCreateActions();

  return (
    <CatalogEmptyState
      action={
        <>
          <Button
            data-testid="home-first-run-session"
            onClick={() => sessionCreate.openForAgent("")}
            size="sm"
            type="button"
            variant="neutral"
          >
            Start a session
          </Button>
          <Button
            nativeButton={false}
            render={<Link to="/tasks/new" />}
            size="sm"
            variant="ghost"
            data-testid="home-first-run-task"
          >
            Create a task
          </Button>
          <Button
            nativeButton={false}
            render={<Link to="/marketplace/skills" />}
            size="sm"
            variant="ghost"
            data-testid="home-first-run-marketplace"
          >
            Browse the marketplace
          </Button>
        </>
      }
      data-testid="home-first-run"
      icon={Home}
      title={workspaceName ? `No agent work yet in ${workspaceName}` : "No agent work yet"}
    />
  );
}
