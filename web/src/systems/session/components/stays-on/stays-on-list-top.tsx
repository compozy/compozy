import { Eyebrow } from "@compozy/ui";

import { GLOBAL_SCOPE, useAgentInstances } from "@/systems/agent";

import type { SessionListGroupBy } from "../../lib/session-list-preferences";
import { instancesInScope } from "../../lib/stays-on-scope";
import { StaysOnGlobalRow } from "./stays-on-signals";
import { StaysOnRow, type StaysOnRowProps } from "./stays-on-row";

/** Flat mode's one row: Global first, then the focused project's. */
function FlatRow({
  workspaceId,
  ...props
}: Omit<StaysOnRowProps, "instances" | "label"> & { workspaceId: string | null }) {
  const instances = useAgentInstances();
  const onlyGlobal = workspaceId === GLOBAL_SCOPE;
  const shown = workspaceId
    ? [
        ...instancesInScope(instances, GLOBAL_SCOPE),
        ...(onlyGlobal ? [] : instancesInScope(instances, workspaceId)),
      ]
    : instances;
  return (
    <StaysOnRow
      {...props}
      instances={shown}
      label={onlyGlobal ? <Eyebrow className="pr-1.5 text-faint">Global</Eyebrow> : undefined}
      scopeName={onlyGlobal ? "Global" : "this list"}
    />
  );
}

/** The row above a session list: Global when projects carry their own, everyone when flat. */
export function StaysOnListTop({
  groupBy,
  workspaceId,
  ...props
}: Omit<StaysOnRowProps, "instances" | "label" | "scopeName"> & {
  groupBy: SessionListGroupBy;
  workspaceId: string | null;
}) {
  if (groupBy === "project") return <StaysOnGlobalRow {...props} className="px-3 pb-2" />;
  return <FlatRow {...props} className="px-3 pb-2" workspaceId={workspaceId} />;
}
