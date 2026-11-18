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
  const { pending, rememberedWorkspace, workspaces } = useActiveWorkspace({
    enabled: workspaceId === null,
  });
  const rebind = (next: string) =>
    void coordinator.userRetarget(windowId, {
      app: "files",
      instanceKey: next,
      route: filesAppRoute(next),
    });
  // A Files window opened without a project (an agent `window_open`, or no projects yet) binds
  // to the active project. The window manager owns the binding, so this effect syncs it there.
  const fallback =
    workspaceId === null ? (rememberedWorkspace?.id ?? workspaces[0]?.id ?? null) : null;
  const bindFallback = useEffectEvent((next: string) => rebind(next));
  useEffect(() => {
    if (fallback !== null) bindFallback(fallback);
  }, [fallback]);
  if (workspaceId !== null) {
    return <FilesAppWindow key={workspaceId} workspaceId={workspaceId} onSwitchProject={rebind} />;
  }
  if (pending || fallback !== null) {
    return (
      <SkeletonRegion label="Loading files" className="p-3" data-testid="files-app-binding">
        <SkeletonRows count={8} />
      </SkeletonRegion>
    );
  }
  return <FilesNoProjectsLocation onProjectAdded={rebind} />;
}

/** No project is registered: offer Add project, and bind the window to the one created (US-002.EC-1). */
function FilesNoProjectsLocation({
  onProjectAdded,
}: {
  onProjectAdded: (workspaceId: string) => void;
}) {
  const [setupOpen, setSetupOpen] = useState(false);
  const setupDefaults = useWorkspaceSetupDefaults(setupOpen);
  return (
    <>
      <FilesAppNoProjects onAddProject={() => setSetupOpen(true)} />
      <WorkspaceSetupDialogBoundary
        defaults={setupDefaults}
        open={setupOpen}
        onOpenChange={setSetupOpen}
        onWorkspaceResolved={onProjectAdded}
      />
    </>
  );
}
