import type { ReactNode } from "react";

import { useSubagentRosterContext } from "@/systems/session/hooks/use-subagent-roster-context";
import { useSubagentRosterSelect } from "@/systems/session/hooks/use-subagent-roster";
import { SubagentCard } from "@/systems/session/components/subagents/subagent-card";
import { SubagentGroup } from "@/systems/session/components/subagents/subagent-group";
import type { SubagentView } from "@/systems/session/components/subagents/types";

import { subagentCardModel } from "./session-timeline-subagents";
import type { SessionSubagentRow } from "./session-timeline.logic";

export interface SessionSubagentRowViewProps {
  row: SessionSubagentRow;
  /** Renders a provider-native card's inner rows; `null` when it has none. */
  renderNested: (subagentId: string) => ReactNode;
  onGroupOpenChange: () => void;
}

/**
 * A delegation in the parent transcript (S1/S2): one card, or the same-turn
 * group. State comes from the parent's roster by id; the part only names it.
 */
export function SessionSubagentRowView({
  row,
  renderNested,
  onGroupOpenChange,
}: SessionSubagentRowViewProps) {
  const { workspaceId, sessionId, onOpen } = useSubagentRosterContext();
  // This row's members only: another card's progress never re-renders it.
  const models = useSubagentRosterSelect(workspaceId, sessionId, roster =>
    row.parts.map(part => subagentCardModel(part, roster))
  );
  const nested = (subagent: SubagentView) => renderNested(subagent.id);
  if (models.length === 1) {
    const [model] = models;
    return (
      <div data-testid="subagent-row" className="flex min-w-0 flex-col">
        <SubagentCard
          subagent={model!.subagent}
          stale={model!.stale}
          onOpen={onOpen}
          nested={nested(model!.subagent)}
        />
      </div>
    );
  }
  return (
    <div data-testid="subagent-row" className="flex min-w-0 flex-col">
      <SubagentGroup
        subagents={models.map(model => model.subagent)}
        stale={models.some(model => model.stale)}
        open={row.expanded}
        onOpenChange={onGroupOpenChange}
        onOpen={onOpen}
        nested={nested}
      />
    </div>
  );
}
