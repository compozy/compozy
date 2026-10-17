import { useQueryClient } from "@tanstack/react-query";
import { use } from "react";

import { cancelSubagent } from "@/systems/session/adapters/subagent-api";
import { sessionDetailOptions } from "@/systems/session/lib/query-options";
import { SubagentWaitingBanner } from "@/systems/session/components/subagents/subagent-waiting-banner";
import {
  subagentWaitingBannerRows,
  subagentWakePending,
} from "@/systems/session/components/subagents/subagent-format";
import { SubagentNavigationContext } from "@/systems/session/contexts/session-subagents-context-value";
import {
  useSessionSubagentRoster,
  useSubagentRosterContext,
} from "@/systems/session/hooks/use-subagent-roster-context";

/**
 * The composer's waiting banner (S5, UT-W13): while the parent has no running
 * turn and delegated subagents still work, it names them and offers one Stop
 * that cancels every one of them through the cancel route. The glyph breathes
 * while a settled result waits to wake the parent.
 */
export function SessionSubagentBanner({ parentTurnRunning }: { parentTurnRunning: boolean }) {
  const queryClient = useQueryClient();
  const { workspaceId, sessionId, onOpen } = useSubagentRosterContext();
  const roster = useSessionSubagentRoster();
  const navigation = use(SubagentNavigationContext);
  const waiting = subagentWaitingBannerRows(roster.rows, parentTurnRunning);
  if (waiting.length === 0) return null;
  return (
    <SubagentWaitingBanner
      subagents={waiting}
      wakePending={subagentWakePending(roster.rows)}
      onOpen={onOpen ? subagent => onOpen(subagent, { newWindow: false }) : undefined}
      onShowAll={navigation?.showSubagents}
      onStop={async () => {
        const parent = await queryClient.fetchQuery({
          ...sessionDetailOptions(workspaceId, sessionId),
          staleTime: 0,
        });
        return Promise.all(
          waiting.map(subagent => cancelSubagent(workspaceId, subagent.id, parent.profile_name))
        );
      }}
    />
  );
}
