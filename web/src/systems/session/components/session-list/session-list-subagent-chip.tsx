import { useState } from "react";

import { useSessionSubagents } from "../../hooks/use-session-subagents";
import { requestSessionInspectorSection } from "../../hooks/use-session-inspector-focus";
import { isSessionRunning } from "../../lib/session-running";
import { sessionSubagentCounts } from "../../lib/session-subagent-summary";
import type { SessionPayload } from "../../types";
import { SubagentChip } from "../subagents/subagent-chip";

export interface SessionListSubagentChipProps {
  session: SessionPayload;
  /** Opens the parent session; the inspector then lands on its Subagents section. */
  onSelect: () => void;
}

/**
 * Parent-row chip fed by the row's `subagent_summary` (S9). The hover preview
 * reads the list route only once the card opens, so a page of parents costs no
 * extra requests; the catalog stream keeps both the summary and an open
 * preview live.
 */
export function SessionListSubagentChip({ session, onSelect }: SessionListSubagentChipProps) {
  const [previewOpen, setPreviewOpen] = useState(false);
  const counts = sessionSubagentCounts(session);
  const roster = useSessionSubagents(
    session.workspace_id ?? "",
    session.id,
    previewOpen && counts !== null
  );
  if (counts === null) return null;
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
