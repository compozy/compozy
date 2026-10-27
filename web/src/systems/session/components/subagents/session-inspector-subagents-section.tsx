import { SessionInspectorSection, SessionInspectorSectionHead } from "../session-inspector-section";
import { subagentRosterGroups, subagentRosterTitle } from "./subagent-format";
import { SubagentRoster, type SubagentRosterProps } from "./subagent-roster";
import { SubagentRosterSkeleton } from "./subagent-roster-skeleton";

export type SessionInspectorSubagentsSectionProps = Omit<
  SubagentRosterProps,
  "hoverSide" | "className"
> & {
  /**
   * The session's summary counts while the list is still loading: the section keeps its place,
   * heading and placeholder rows included, instead of appearing and pushing the rail down.
   */
  pending?: { total: number; live: number };
};

/**
 * Inspector roster of this session's subagents (nav VC-01/03): failed pinned on
 * top, then live (waiting first), then the collapsed `Previous subagents (N)`.
 * Six rows first, then `Show 12 more` per page. Absent when there are none.
 */
export function SessionInspectorSubagentsSection({
  subagents,
  pending,
  ...roster
}: SessionInspectorSubagentsSectionProps) {
  if (subagents.length === 0 && !pending) return null;
  const loading = subagents.length === 0 && pending !== undefined;
  const live = loading ? pending.live : subagentRosterGroups(subagents).live.length;
  const total = loading ? pending.total : subagents.length;

  return (
    <SessionInspectorSection data-testid="session-inspector-subagents">
      <SessionInspectorSectionHead meta={total}>
        {subagentRosterTitle(live)}
      </SessionInspectorSectionHead>
      {loading ? (
        <SubagentRosterSkeleton count={total} />
      ) : (
        <SubagentRoster subagents={subagents} {...roster} />
      )}
    </SessionInspectorSection>
  );
}
