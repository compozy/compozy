import {
  ActionResultBanner,
  Button,
  MetadataList,
  NativeSelect,
  NativeSelectOption,
  Spinner,
} from "@compozy/ui";

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

type HeartbeatSessionHealth = NonNullable<AgentHeartbeatStatusPayload["session_health"]>;

function describeSchedule(status: AgentHeartbeatStatusPayload | undefined): string {
  const minInterval = status?.preferences?.min_interval;
  if (!minInterval) return "None";
  const windows = status?.preferences?.active_hours?.length ?? 0;
  if (windows === 0) return `min interval ${minInterval}`;
  return `min interval ${minInterval} · ${windows} active-hour window${windows === 1 ? "" : "s"}`;
}

function describeRecentWake(status: AgentHeartbeatStatusPayload | undefined): string {
  const recentWake = status?.wake_events?.[0];
  return recentWake ? `${recentWake.result} · ${recentWake.reason}` : "None";
}

function HeartbeatStatusErrorBanner({ stale, onRetry }: { stale: boolean; onRetry: () => void }) {
  return (
    <ActionResultBanner
      tone="danger"
      title={stale ? "Couldn't refresh heartbeat status" : "Couldn't load heartbeat status"}
      description={
        stale
          ? "Cached status may be stale. Retry to refresh wake eligibility."
          : "Retry to check the wake policy and session eligibility."
      }
      actions={
        <Button type="button" size="sm" variant="ghost" onClick={onRetry}>
          Retry
        </Button>
      }
      data-testid={stale ? "agent-heartbeat-status-stale-error" : "agent-heartbeat-status-error"}
    />
  );
}

function HeartbeatPolicyList({
  status,
  statusLoading,
}: Pick<AgentHeartbeatOpsProps, "status" | "statusLoading">) {
  const enabledLabel = statusLoading ? "…" : status?.enabled ? "Yes" : "No";
  return (
    <MetadataList>
      <MetadataList.Row>
        <MetadataList.Term>Enabled</MetadataList.Term>
        <MetadataList.Value>{enabledLabel}</MetadataList.Value>
      </MetadataList.Row>
      <MetadataList.Row>
        <MetadataList.Term>Schedule</MetadataList.Term>
        <MetadataList.Value className="text-muted">{describeSchedule(status)}</MetadataList.Value>
      </MetadataList.Row>
      <MetadataList.Row>
        <MetadataList.Term>Recent wake</MetadataList.Term>
        <MetadataList.Value className="text-muted">{describeRecentWake(status)}</MetadataList.Value>
      </MetadataList.Row>
    </MetadataList>
  );
}

function HeartbeatSessionPicker({
  activeSessions,
  selectedSessionId,
  onSelectSessionId,
  onNewSession,
}: Pick<
  AgentHeartbeatOpsProps,
  "activeSessions" | "selectedSessionId" | "onSelectSessionId" | "onNewSession"
>) {
  if (activeSessions.length === 0) {
    return (
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
    );
  }
  return (
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
  );
}

/** The selected session's eligibility line: checking, or why it cannot be woken. */
function HeartbeatEligibilityNote({
  statusLoading,
  health,
}: {
  statusLoading: boolean;
  health: HeartbeatSessionHealth | null;
}) {
  if (statusLoading) {
    return (
      <p className="text-small-body text-muted" role="status">
        Checking session eligibility…
      </p>
    );
  }
  if (!health || health.eligible_for_wake) return null;
  return (
    <p className="text-small-body text-warning" data-testid="agent-heartbeat-ineligible">
      This session cannot be woken: {health.ineligibility_reason ?? health.health}.
    </p>
  );
}

function HeartbeatWakeOutcome({
  wakeDecision,
  wakeError,
}: Pick<AgentHeartbeatOpsProps, "wakeDecision" | "wakeError">) {
  const sent = wakeDecision?.result === "sent";
  return (
    <>
      {wakeDecision ? (
        <ActionResultBanner
          tone={sent ? "success" : "warning"}
          title={sent ? "Wake sent" : "Wake not sent"}
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
    </>
  );
}

function canWakeSelectedSession({
  activeSessions,
  selectedSessionId,
  statusLoading,
  statusError,
  health,
  waking,
}: Pick<
  AgentHeartbeatOpsProps,
  "activeSessions" | "selectedSessionId" | "statusLoading" | "statusError" | "waking"
> & { health: HeartbeatSessionHealth | null }): boolean {
  if (selectedSessionId === null || statusLoading || statusError || waking) return false;
  if (!health?.eligible_for_wake) return false;
  return activeSessions.some(session => session.id === selectedSessionId);
}

export function AgentHeartbeatOps(props: AgentHeartbeatOpsProps) {
  const { status, statusLoading, statusError, onRetryStatus, selectedSessionId, onWake, waking } =
    props;
  const health = selectedSessionId ? (status?.session_health ?? null) : null;
  const canWake = canWakeSelectedSession({ ...props, health });

  if (statusError && !status) {
    return <HeartbeatStatusErrorBanner stale={false} onRetry={onRetryStatus} />;
  }

  return (
    <div
      className="flex flex-col gap-4 rounded-lg bg-card p-4 shadow-card"
      data-testid="agent-heartbeat-ops"
    >
      {statusError ? <HeartbeatStatusErrorBanner stale onRetry={onRetryStatus} /> : null}
      <HeartbeatPolicyList status={status} statusLoading={statusLoading} />

      <div className="flex flex-col gap-2">
        <p className="eyebrow text-muted">Wake target</p>
        <HeartbeatSessionPicker {...props} />
        {selectedSessionId ? (
          <HeartbeatEligibilityNote statusLoading={statusLoading} health={health} />
        ) : null}
      </div>

      <HeartbeatWakeOutcome wakeDecision={props.wakeDecision} wakeError={props.wakeError} />

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
        {waking ? <Spinner className="size-3.5" /> : null}
        {waking ? "Waking…" : "Wake now"}
      </Button>
    </div>
  );
}
