import { emptyForScope } from "@/systems/profiles";

import type { SessionFolderDialogState } from "../../hooks/use-session-list-folder-layout";
import type { WorkspaceFolderPayload } from "../../types";
import { SessionListBadgeContext } from "../../lib/session-list-row-badge";
import { sessionListGroupTarget, type SessionListGroupSpec } from "../../lib/session-list-groups";
import { sessionListShelfNoun } from "../../lib/session-list-shelf";
import { StaysOnProjectRow } from "../stays-on/stays-on-signals";
import {
  GroupLoadFailure,
  SessionListGroupBody,
  type GroupRowProps,
} from "./session-list-group-body";
import { SessionListGroupHeader, type SessionListGroupArrange } from "./session-list-group-header";
import { SessionListProjectFolders } from "./session-list-project-folders";
import { SessionGroupsSkeleton } from "./session-list-skeleton";

/**
 * One collapsible group: its header (toggle and `+`) and, while open, its
 * body. A group that spans pairs renders its split subgroups through this same
 * section, one level in.
 */
function SessionListGroupSection({
  group,
  path,
  nested = false,
  arrange,
  ...props
}: GroupRowProps & {
  group: SessionListGroupSpec;
  path: readonly number[];
  /** A split subgroup: its header and its parent's together name both axes. */
  nested?: boolean;
  /** The header's part in project folders, where they apply. */
  arrange?: SessionListGroupArrange;
}) {
  const { view, testIdPrefix } = props;
  const collapsed = view.collapsedGroupKeys.has(group.key);
  const staysOnScope = group.kind === "project" ? sessionListGroupTarget(group).workspaceId : null;
  return (
    <section data-collapsed={collapsed ? "true" : undefined} data-group-key={group.key}>
      <SessionListGroupHeader
        group={group}
        collapsed={collapsed}
        onToggle={() => view.toggleGroup(group.key)}
        onNewSession={props.onNewSession}
        testIdPrefix={testIdPrefix}
        arrange={arrange}
      />
      {collapsed || !staysOnScope ? null : (
        <StaysOnProjectRow
          className="px-1 pt-1 pb-1.5 pl-3.5"
          currentSessionId={props.currentSessionId}
          name={group.badge?.name ?? "this project"}
          onOpenSession={props.onSelectSession}
          workspaceId={staysOnScope}
        />
      )}
      {collapsed ? null : (
        <SessionListBadgeContext
          value={nested || group.kind === "project" ? undefined : view.badgeOf}
        >
          <SessionListGroupBody
            {...props}
            group={group}
            path={path}
            nested={nested}
            renderSubgroup={(subgroup, subPath) => (
              <SessionListGroupSection
                key={subgroup.key}
                {...props}
                group={subgroup}
                path={subPath}
                nested
              />
            )}
          />
        </SessionListBadgeContext>
      )}
    </section>
  );
}

/** What a list hosting project folders hands them: where their dialogs open. */
export interface SessionListProjectFoldersHost {
  onProjectFolderDialog: (state: SessionFolderDialogState<WorkspaceFolderPayload>) => void;
}

/**
 * Group-by groups (ADR-004): membership, counts, and the collapsed needs-you or
 * working dot come from server facets, so an off-page group is never missing.
 * Every expanded group reads and pages its own query; a failed group retries in
 * place. Inside a project group rows omit the project badge; inside a profile
 * group they omit the profile tag.
 */
export function SessionListGroups(props: GroupRowProps & SessionListProjectFoldersHost) {
  const { view, testIdPrefix } = props;
  if (view.groupsError) {
    return (
      <GroupLoadFailure
        retryTestId={`${testIdPrefix}-groups-retry`}
        onRetry={view.groupsError.retry}
      />
    );
  }
  if (view.groupsLoading) return <SessionGroupsSkeleton />;
  if (view.projectFolders) {
    const { onProjectFolderDialog, ...rows } = props;
    return (
      <SessionListProjectFolders
        projectFolders={view.projectFolders}
        onFolderDialog={onProjectFolderDialog}
        renderGroup={(group, path, arrange) => (
          <SessionListGroupSection
            key={group.key}
            {...rows}
            group={group}
            path={path}
            arrange={arrange}
          />
        )}
        emptyMessage={`${emptyForScope(sessionListShelfNoun(view.shelf), view.scopeLabel)}.`}
        testIdPrefix={testIdPrefix}
      />
    );
  }
  if (view.groups.length === 0) {
    return (
      <p className="px-3 py-8 text-center text-small-body text-muted-foreground">
        {emptyForScope(sessionListShelfNoun(view.shelf), view.scopeLabel)}.
      </p>
    );
  }
  return (
    <div className="flex flex-col gap-1">
      {view.groups.map((group, index) => (
        <SessionListGroupSection key={group.key} {...props} group={group} path={[index]} />
      ))}
    </div>
  );
}
