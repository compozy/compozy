import { emptyForScope } from "@/systems/profiles";

import { SessionListBadgeContext } from "../../lib/session-list-row-badge";
import type { SessionListGroupSpec } from "../../lib/session-list-groups";
import {
  GroupLoadFailure,
  SessionListGroupBody,
  type GroupRowProps,
} from "./session-list-group-body";
import { SessionListGroupHeader } from "./session-list-group-header";
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
  ...props
}: GroupRowProps & {
  group: SessionListGroupSpec;
  path: readonly number[];
  /** A split subgroup: its header and its parent's together name both axes. */
  nested?: boolean;
}) {
  const { view, testIdPrefix } = props;
  const collapsed = view.collapsedGroupKeys.has(group.key);
  return (
    <section data-collapsed={collapsed ? "true" : undefined} data-group-key={group.key}>
      <SessionListGroupHeader
        group={group}
        collapsed={collapsed}
        onToggle={() => view.toggleGroup(group.key)}
        onNewSession={props.onNewSession}
        testIdPrefix={testIdPrefix}
      />
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

/**
 * Group-by groups (ADR-004): membership, counts, and the collapsed needs-you or
 * working dot come from server facets, so an off-page group is never missing.
 * Every expanded group reads and pages its own query; a failed group retries in
 * place. Inside a project group rows omit the project badge; inside a profile
 * group they omit the profile tag.
 */
export function SessionListGroups(props: GroupRowProps) {
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
  if (view.groups.length === 0) {
    return (
      <p className="px-3 py-8 text-center text-small-body text-muted">
        {emptyForScope(view.archived ? "archived sessions" : "sessions", view.scopeLabel)}.
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
