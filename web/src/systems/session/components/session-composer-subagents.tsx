import { useState } from "react";

import { useSessionSubagents } from "../hooks/use-session-subagents";
import { sessionSubagentCounts } from "../lib/session-subagent-summary";
import type { SessionPayload } from "../types";
import { SubagentComposerControl } from "./subagents/subagent-composer-control";
import type { SubagentOpenOptions } from "./subagents/subagent-card";
import { subagentCounts, type SubagentCounts } from "./subagents/subagent-format";
import type { SubagentView } from "./subagents/types";

const NO_SUBAGENTS: SubagentCounts = { live: 0, total: 0, failed: 0, attention: 0 };

export interface SessionComposerSubagentsProps {
  session: SessionPayload;
  /** Drill-in (S1 rule): this window, or a new one with ⌘/Ctrl. */
  onOpen?: (subagent: SubagentView, options: SubagentOpenOptions) => void;
}

/**
 * Feeds the composer's subagents control. The open session's stream roster
 * costs no request; the list route is read only once the session's summary
 * reports subagents or the operator opens the popover.
 */
export function SessionComposerSubagents({ session, onOpen }: SessionComposerSubagentsProps) {
  const [open, setOpen] = useState(false);
  const summary = sessionSubagentCounts(session);
  const roster = useSessionSubagents(session.workspace_id ?? "", session.id, {
    coldRead: open || summary !== null,
  });
  // The whole roster is the answer, even an empty one; the summary stands in until every row is in.
  const counts = roster.complete ? subagentCounts(roster.subagents) : (summary ?? NO_SUBAGENTS);

  return (
    <SubagentComposerControl
      subagents={roster.subagents}
      counts={counts}
      // Until the roster answers, the popover holds rows' place rather than claim nothing was delegated.
      loading={!roster.complete && !roster.failed}
      failed={roster.failed}
      onRetry={roster.retry}
      onOpen={onOpen}
      onStop={roster.stop}
      onOpenChange={setOpen}
    />
  );
}
