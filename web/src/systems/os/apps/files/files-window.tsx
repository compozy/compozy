import { useEffect, useEffectEvent, useState } from "react";

import { SkeletonRegion, SkeletonRows } from "@compozy/ui";

import { FilesAppNoProjects, FilesAppWindow } from "@/systems/files";
import { useActiveWorkspace } from "@/systems/workspace";

import { WorkspaceSetupDialogBoundary } from "../../components/desktop-shell-dialogs";
import { useDesktop } from "../../hooks/use-desktop";
import { useOsShell } from "../../hooks/use-os-shell";
import { useWorkspaceSetupDefaults } from "../../hooks/use-workspace-setup-defaults";
import { FILES_APP_PATH, filesAppRoute, matchFilesInstance } from "../../lib/app-catalog";

const BARE_FILES_ROUTE = { pathname: FILES_APP_PATH, search: {} } as const;

/** Files app controller: one window per project, keyed by the route's workspace id. */
export function FilesWindow({ windowId }: { windowId: string }) {
  const route = useDesktop(state => state.windows[windowId]?.route ?? BARE_FILES_ROUTE);
  const { coordinator } = useOsShell();
  const workspaceId = matchFilesInstance(route.pathname);
  const rebind = (next: string) =>
    void coordinator.userRetarget(windowId, {
      app: "files",
      instanceKey: next,
      route: filesAppRoute(next),
    });
  if (workspaceId !== null) {
    return <FilesAppWindow key={workspaceId} workspaceId={workspaceId} onSwitchProject={rebind} />;
  }
  return <FilesUnboundLocation onBind={rebind} />;
}

/**
 * A Files window opened without a project (an agent `window_open`, or no projects yet): it binds
 * to the active project, or offers Add project when none is registered (US-002.EC-1).
 */
function FilesUnboundLocation({ onBind }: { onBind: (workspaceId: string) => void }) {
  const { pending, rememberedWorkspace, workspaces } = useActiveWorkspace();
  const [setupOpen, setSetupOpen] = useState(false);
  const setupDefaults = useWorkspaceSetupDefaults(setupOpen);
  const fallback = rememberedWorkspace?.id ?? workspaces[0]?.id ?? null;
  const bind = useEffectEvent((workspaceId: string) => onBind(workspaceId));
  useEffect(() => {
    if (fallback !== null) bind(fallback);
  }, [fallback]);
  if (pending || fallback !== null) {
    return (
      <SkeletonRegion label="Loading files" className="p-3" data-testid="files-app-binding">
        <SkeletonRows count={8} />
      </SkeletonRegion>
    );
  }
  return (
    <>
      <FilesAppNoProjects onAddProject={() => setSetupOpen(true)} />
      <WorkspaceSetupDialogBoundary
        defaults={setupDefaults}
        open={setupOpen}
        onOpenChange={setSetupOpen}
        onWorkspaceResolved={onBind}
      />
    </>
  );
}
