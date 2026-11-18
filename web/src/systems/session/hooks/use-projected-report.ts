import { useQuery } from "@tanstack/react-query";

import { agentInstanceProjectedReportsOptions, useAgentInstances } from "@/systems/agent";

import { reportPayload, withProjection } from "../lib/session-report";
import type { SessionReportData } from "../types";

const NO_INSTANCE = { id: "", profile_name: "" };

/** Check-in reports whose picks and tool calls the daemon keeps live. */
function isProjected(data: SessionReportData) {
  return data.kind === "activity" || (data.kind === "proposal" && reportPayload(data).picksTracked);
}

/**
 * A transcript report's instance, and the report with the daemon's live projection laid over it
 * from one shared read of the instance's kept reports. Older reports, and ones past the daemon's
 * keep, stand as published.
 */
export function useProjectedReport(published: SessionReportData) {
  const instances = useAgentInstances();
  const instance = instances.find(candidate => candidate.id === published.instance_id);
  const { data: projected } = useQuery({
    ...agentInstanceProjectedReportsOptions(instance ?? NO_INSTANCE),
    enabled: Boolean(instance) && isProjected(published),
    select: reports => reports.find(report => report.id === published.report_id) ?? null,
  });
  return { data: withProjection(published, projected), instance, instances };
}
