import { SessionInspectorSection, SessionInspectorSectionHead } from "../session-inspector-section";
import { subagentRosterGroups, subagentRosterTitle } from "./subagent-format";
import { SubagentRoster, type SubagentRosterProps } from "./subagent-roster";

export type SessionInspectorSubagentsSectionProps = Omit<
  SubagentRosterProps,
  "hoverSide" | "className"
>;

/**
 * Inspector roster of this session's subagents (nav VC-01/03): failed pinned on
 * top, then live (waiting first), then the collapsed `Previous subagents (N)`.
 * Six rows first, then `Show 12 more` per page. Absent when there are none.
 */
export function SessionInspectorSubagentsSection({
  subagents,
  ...roster
}: SessionInspectorSubagentsSectionProps) {
  if (subagents.length === 0) return null;
  const live = subagentRosterGroups(subagents).live.length;

  return (
    <SessionInspectorSection data-testid="session-inspector-subagents">
      <SessionInspectorSectionHead meta={subagents.length}>
        {subagentRosterTitle(live)}
      </SessionInspectorSectionHead>
      <SubagentRoster subagents={subagents} {...roster} />
    </SessionInspectorSection>
  );
}
