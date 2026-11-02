import type { SessionPayload } from "../types";
import type { SessionContextView } from "../lib/session-context";
import { describeSessionContextMeter } from "../lib/session-context-view";
import { SessionContextCompactAction } from "./session-context-compact-action";
import { SessionContextMeterBody } from "./session-context-meter-body";
import { SessionInspectorSection } from "./session-inspector-section";

export function SessionContextMeterSection({
  context,
  session,
}: {
  context: SessionContextView;
  /** The inspected session; its advertised commands decide whether Compact now is offered. */
  session?: SessionPayload;
}) {
  const view = describeSessionContextMeter(context);
  return (
    <SessionInspectorSection data-testid="session-context-meter">
      <SessionContextMeterBody view={view} />
      {session ? <SessionContextCompactAction session={session} /> : null}
    </SessionInspectorSection>
  );
}
