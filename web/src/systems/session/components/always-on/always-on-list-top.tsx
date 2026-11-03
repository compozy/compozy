import { Eyebrow } from "@compozy/ui";

import { GLOBAL_SCOPE, useAgentInstances } from "@/systems/agent";

import type { SessionListGroupBy } from "../../lib/session-list-preferences";
import { instancesInScope } from "../../lib/always-on-scope";
import { AlwaysOnGlobalRow } from "./always-on-signals";
import { AlwaysOnRow, type AlwaysOnRowProps } from "./always-on-row";

/** Flat mode's one row: Global first, then the focused project's. */
function FlatRow({
  workspaceId,
  ...props
}: Omit<AlwaysOnRowProps, "instances" | "label"> & { workspaceId: string | null }) {
  const instances = useAgentInstances();
  const onlyGlobal = workspaceId === GLOBAL_SCOPE;
  const shown = workspaceId
    ? [
        ...instancesInScope(instances, GLOBAL_SCOPE),
        ...(onlyGlobal ? [] : instancesInScope(instances, workspaceId)),
      ]
    : instances;
  return (
    <AlwaysOnRow
      {...props}
      instances={shown}
      label={onlyGlobal ? <Eyebrow className="pr-1.5 text-faint">Global</Eyebrow> : undefined}
      scopeName={onlyGlobal ? "Global" : "this list"}
    />
  );
}

/** The row above a session list: Global when projects carry their own, everyone when flat. */
export function AlwaysOnListTop({
  groupBy,
  workspaceId,
  ...props
}: Omit<AlwaysOnRowProps, "instances" | "label" | "scopeName"> & {
  groupBy: SessionListGroupBy;
  workspaceId: string | null;
}) {
  if (groupBy === "project") return <AlwaysOnGlobalRow {...props} className="px-3 pb-2" />;
  return <FlatRow {...props} className="px-3 pb-2" workspaceId={workspaceId} />;
}
