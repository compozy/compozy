import { useState } from "react";

import { useSessionSubagents } from "../../hooks/use-session-subagents";
import { requestSessionInspectorSection } from "../../hooks/use-session-inspector-focus";
import { isSessionRunning } from "../../lib/session-running";
import type { SessionSubagentCounts } from "../../lib/session-subagent-summary";
import type { SessionPayload } from "../../types";
import { SubagentChip } from "../subagents/subagent-chip";

export interface SessionListSubagentChipProps {
  session: SessionPayload;
  /** The row's `subagent_summary` counts, read once by the row. */
  counts: SessionSubagentCounts;
  /** Opens the parent session; the inspector then lands on its Subagents section. */
  onSelect: () => void;
}

/**
 * Parent-row chip fed by the row's `subagent_summary` (S9). The hover preview
 * reads only once the card opens, so a page of parents costs no extra requests:
 * from the parent's live roster when its session is open, else one list page.
 */
export function SessionListSubagentChip({
  session,
  counts,
  onSelect,
}: SessionListSubagentChipProps) {
  const [previewOpen, setPreviewOpen] = useState(false);
  const roster = useSessionSubagents(session.workspace_id ?? "", session.id, {
    enabled: previewOpen,
    allPages: false,
  });
  return (
    <SubagentChip
      counts={counts}
      parentTurnRunning={isSessionRunning(session)}
      preview={roster.subagents.length > 0 ? roster.subagents : undefined}
      onPreviewOpenChange={setPreviewOpen}
      onOpen={() => {
        requestSessionInspectorSection(session.id, "subagents");
        onSelect();
      }}
    />
  );
}
