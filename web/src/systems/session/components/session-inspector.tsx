import { DetailInspector, cn } from "@compozy/ui";
import { deriveSessionContext, type SessionContextView } from "../lib/session-context";
import type { SessionPayload, SessionUsageTurnsResponse } from "../types";
import { SessionContextMeterSection } from "./session-context-meter-section";
import { SessionContextInjectedSection } from "./session-context-injected-section";
import { SessionContextTurnsSection } from "./session-context-turns-section";
import { SessionActivitySection, type SessionActivityView } from "./session-activity-section";
import { SessionInspectorOriginSection } from "./session-inspector-origin-section";
import { SessionInspectorUsageSection } from "./session-inspector-sections";
import type { InspectorUsage } from "./session-inspector-types";

export type { InspectorUsage } from "./session-inspector-types";

export interface SessionInspectorProps {
  context?: SessionContextView;
  usage?: InspectorUsage | null;
  turns?: SessionUsageTurnsResponse;
  turnsUnavailable?: boolean;
  activity?: SessionActivityView;
  /** The inspected session; a continued or forked one gains its Origin rows. */
  session?: SessionPayload;
  injectedDefaultOpen?: boolean;
  turnsDefaultOpen?: boolean;
  drawerOpen?: boolean;
  onDrawerOpenChange?: (open: boolean) => void;
  className?: string;
}

export function SessionInspector({
  context = deriveSessionContext(),
  usage,
  turns,
  turnsUnavailable,
  activity,
  session,
  injectedDefaultOpen,
  turnsDefaultOpen,
  drawerOpen,
  onDrawerOpenChange,
  className,
}: SessionInspectorProps) {
  return (
    <DetailInspector
      title="Context"
      className={cn("min-w-0", className)}
      drawerClassName="rounded-none border-l border-line-strong"
      onOpenChange={onDrawerOpenChange}
      open={drawerOpen}
    >
      <div className="flex flex-col gap-4.5 p-4" data-testid="session-inspector">
        <SessionContextMeterSection context={context} />
        <SessionContextInjectedSection
          injected={context.injected}
          showBars={context.used != null}
          defaultOpen={injectedDefaultOpen}
        />
        <SessionInspectorUsageSection usage={usage} />
        <SessionContextTurnsSection
          data={turns}
          unavailable={turnsUnavailable}
          defaultOpen={turnsDefaultOpen}
        />
        <SessionActivitySection activity={activity} />
        {session ? <SessionInspectorOriginSection session={session} /> : null}
      </div>
    </DetailInspector>
  );
}
