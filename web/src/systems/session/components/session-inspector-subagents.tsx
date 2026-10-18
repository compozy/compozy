import { useEffect, useRef } from "react";

import { useSessionInspectorFocus } from "../hooks/use-session-inspector-focus";
import { useSessionSubagents } from "../hooks/use-session-subagents";
import type { SessionPayload } from "../types";
import { SessionInspectorSubagentsSection } from "./subagents/session-inspector-subagents-section";
import type { SubagentOpenOptions } from "./subagents/subagent-card";
import type { SubagentView } from "./subagents/types";

export interface SessionInspectorSubagentsProps {
  session: SessionPayload;
  /** Drill-in (S1 rule): this window, or a new one with ⌘/Ctrl. */
  onOpen?: (subagent: SubagentView, options: SubagentOpenOptions) => void;
}

/**
 * Mounts the inspector's Subagents roster (S10) from the paged list route: the
 * session's direct children only. The host mounts it only while the session's
 * summary reports subagents; it lands in view when the sidebar chip asked.
 */
export function SessionInspectorSubagents({ session, onOpen }: SessionInspectorSubagentsProps) {
  const roster = useSessionSubagents(session.workspace_id ?? "", session.id);
  const focus = useSessionInspectorFocus(session.id, "subagents");
  const anchor = useRef<HTMLDivElement>(null);
  const ready = roster.subagents.length > 0;
  const { requested, land } = focus;

  useEffect(() => {
    if (!requested || !ready) return;
    anchor.current?.scrollIntoView({ block: "start" });
    land();
  }, [requested, ready, land]);

  return (
    <div ref={anchor}>
      <SessionInspectorSubagentsSection
        subagents={roster.subagents}
        onOpen={onOpen}
        onStop={roster.stop}
      />
    </div>
  );
}
