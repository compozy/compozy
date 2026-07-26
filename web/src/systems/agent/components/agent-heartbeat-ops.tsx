import {
  ActionResultBanner,
  Button,
  MetadataList,
  NativeSelect,
  NativeSelectOption,
  Spinner,
} from "@agh/ui";

import type { AgentHeartbeatStatusPayload, WakeAgentHeartbeatResponse } from "../types";
import { getSessionDisplayTitle, type SessionPayload } from "@/systems/session";

export interface AgentHeartbeatOpsProps {
  status: AgentHeartbeatStatusPayload | undefined;
  statusLoading: boolean;
  statusError: boolean;
  onRetryStatus: () => void;
  activeSessions: SessionPayload[];
  selectedSessionId: string | null;
  onSelectSessionId: (sessionId: string | null) => void;
  onWake: (sessionId: string) => void;
  waking: boolean;
  wakeDecision: WakeAgentHeartbeatResponse["decision"] | undefined;
  wakeError: string | null;
  onNewSession: () => void;
}

export function AgentHeartbeatOps({
  status,
  statusLoading,
  statusError,
  onRetryStatus,
  activeSessions,
  selectedSessionId,
  onSelectSessionId,
  onWake,
  waking,
  wakeDecision,
  wakeError,
  onNewSession,
}: AgentHeartbeatOpsProps) {
  const minInterval = status?.preferences?.min_interval;
  const activeHours = status?.preferences?.active_hours ?? [];
  const recentWakes = status?.wake_events ?? [];
  const recentWake = recentWakes[0];
  const health = selectedSessionId ? status?.session_health : null;
  const hasSelectedActiveSession =
    selectedSessionId !== null && activeSessions.some(session => session.id === selectedSessionId);
  const canWake = Boolean(
    hasSelectedActiveSession &&
    !statusLoading &&
    !statusError &&
    health?.eligible_for_wake &&
    !waking
  );

  if (statusError && !status) {
    return (
      <ActionResultBanner
        tone="danger"
        title="Couldn't load heartbeat status"
        description="Retry to check the wake policy and session eligibility."
        actions={
          <Button type="button" size="sm" variant="ghost" onClick={onRetryStatus}>
            Retry
          </Button>
        }
        data-testid="agent-heartbeat-status-error"
      />
    );
  }

  return (
    <div
      className="flex flex-col gap-4 rounded-md border border-line p-4"
      data-testid="agent-heartbeat-ops"
    >
      {statusError ? (
        <ActionResultBanner
          tone="danger"
          title="Couldn't refresh heartbeat status"
          description="Cached status may be stale. Retry to refresh wake eligibility."
          actions={
            <Button type="button" size="sm" variant="ghost" onClick={onRetryStatus}>
              Retry
            </Button>
          }
          data-testid="agent-heartbeat-status-stale-error"
        />
      ) : null}
      <MetadataList>
        <MetadataList.Row>
          <MetadataList.Term>Enabled</MetadataList.Term>
          <MetadataList.Value>
            {statusLoading ? "…" : status?.enabled ? "Yes" : "No"}
          </MetadataList.Value>
        </MetadataList.Row>
        <MetadataList.Row>
          <MetadataList.Term>Schedule</MetadataList.Term>
          <MetadataList.Value className="text-muted">
            {minInterval
              ? `min interval ${minInterval}${
                  activeHours.length > 0
                    ? ` · ${activeHours.length} active-hour window${activeHours.length === 1 ? "" : "s"}`
                    : ""
                }`
              : "None"}
          </MetadataList.Value>
        </MetadataList.Row>
        <MetadataList.Row>
          <MetadataList.Term>Recent wake</MetadataList.Term>
          <MetadataList.Value className="text-muted">
            {recentWake ? `${recentWake.result} · ${recentWake.reason}` : "None"}
          </MetadataList.Value>
        </MetadataList.Row>
      </MetadataList>

      <div className="flex flex-col gap-2">
        <p className="eyebrow text-muted">Wake target</p>
        {activeSessions.length === 0 ? (
          <div className="flex flex-col gap-2" data-testid="agent-heartbeat-no-session">
            <p className="text-small-body text-muted">
              Wake now needs an active session for this agent.
            </p>
            <Button
              type="button"
              size="sm"
              variant="ghost"
              onClick={onNewSession}
              data-testid="agent-heartbeat-new-session"
            >
              New session
            </Button>
          </div>
        ) : (
          <NativeSelect
            className="w-full"
            value={selectedSessionId ?? ""}
            onChange={event => onSelectSessionId(event.target.value || null)}
            aria-label="Active session for wake"
            data-testid="agent-heartbeat-session-select"
          >
            {activeSessions.length > 1 ? (
              <NativeSelectOption value="">Select an active session</NativeSelectOption>
            ) : null}
            {activeSessions.map(session => (
              <NativeSelectOption key={session.id} value={session.id}>
                {getSessionDisplayTitle(session)}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        )}
        {selectedSessionId && statusLoading ? (
          <p className="text-small-body text-muted" role="status">
            Checking session eligibility…
          </p>
        ) : null}
        {selectedSessionId && !statusLoading && health && !health.eligible_for_wake ? (
          <p className="text-small-body text-warning" data-testid="agent-heartbeat-ineligible">
            This session cannot be woken: {health.ineligibility_reason ?? health.health}.
          </p>
        ) : null}
      </div>

      {wakeDecision ? (
        <ActionResultBanner
          tone={wakeDecision.result === "sent" ? "success" : "warning"}
          title={wakeDecision.result === "sent" ? "Wake sent" : "Wake not sent"}
          description={wakeDecision.reason}
          data-testid="agent-heartbeat-wake-result"
        />
      ) : null}
      {wakeError ? (
        <ActionResultBanner
          tone="danger"
          title="Couldn't wake session"
          description={wakeError}
          data-testid="agent-heartbeat-wake-error"
        />
      ) : null}

      <Button
        type="button"
        size="sm"
        disabled={!canWake}
        onClick={() => {
          if (selectedSessionId) onWake(selectedSessionId);
        }}
        data-testid="agent-heartbeat-wake"
        aria-live="polite"
      >
        {waking ? <Spinner className="size-3" /> : null}
        {waking ? "Waking…" : "Wake now"}
      </Button>
    </div>
  );
}
