import { USER_SCOPE_SELECTOR, useActiveWorkspace } from "@/systems/workspace";

import { ALL_PROJECTS, useArchivedCleanup } from "../../hooks/use-archived-cleanup";
import type { SessionListViewModel } from "../../hooks/use-session-list-view";
import type { CleanupScope } from "../../lib/archived-cleanup-model";
import { ArchivedCleanupDialog } from "./archived-cleanup-dialog";
import { ArchivedCleanupFooter } from "./archived-cleanup-footer";

const USER_SCOPE_NAME = "User";

/**
 * The Archived list's "Clean up history" (#1717): the footer verb and its dialog, bound to the
 * daemon. It needs one profile and a settled scope: under All profiles there is nothing to show.
 */
export function ArchivedCleanup({
  view,
  testIdPrefix,
}: {
  view: Pick<
    SessionListViewModel,
    "archivedCount" | "allProjects" | "focusedWorkspaceId" | "focusedScopeName" | "scopeLabel"
  >;
  testIdPrefix: string;
}) {
  const profile = view.scopeLabel;
  const scope: CleanupScope | null = view.allProjects
    ? { allWorkspaces: true }
    : view.focusedWorkspaceId
      ? { workspaceId: view.focusedWorkspaceId }
      : null;
  if (!profile || !scope) return null;
  return (
    <ConnectedArchivedCleanup
      archivedCount={view.archivedCount}
      profile={profile}
      scope={scope}
      scopeName={view.allProjects ? ALL_PROJECTS : view.focusedScopeName}
      testIdPrefix={testIdPrefix}
    />
  );
}

function ConnectedArchivedCleanup({
  profile,
  scope,
  scopeName,
  archivedCount,
  testIdPrefix,
}: {
  profile: string;
  scope: CleanupScope;
  scopeName: string;
  archivedCount?: number;
  testIdPrefix: string;
}) {
  const { registeredWorkspaces } = useActiveWorkspace();
  const names = new Map(registeredWorkspaces.map(workspace => [workspace.id, workspace.name]));
  const cleanup = useArchivedCleanup({
    profile,
    scope,
    projectName: id =>
      id === "" || id === USER_SCOPE_SELECTOR ? USER_SCOPE_NAME : (names.get(id) ?? id),
  });
  return (
    <>
      <ArchivedCleanupFooter
        archivedCount={archivedCount}
        onCleanUp={() => cleanup.setOpen(true)}
        progress={cleanup.progress}
        testIdPrefix={`${testIdPrefix}-archived-cleanup`}
      />
      <ArchivedCleanupDialog
        acrossProjects={"allWorkspaces" in scope}
        counting={cleanup.counting}
        counts={cleanup.counts}
        impact={cleanup.impact}
        onConfirm={cleanup.confirm}
        onOpenChange={cleanup.setOpen}
        onOptionsChange={cleanup.setOptions}
        onRetry={cleanup.retry}
        onStop={cleanup.stop}
        open={cleanup.open}
        options={cleanup.options}
        phase={cleanup.phase}
        previewError={cleanup.previewError}
        scopeName={scopeName}
        starting={cleanup.starting}
      />
    </>
  );
}
