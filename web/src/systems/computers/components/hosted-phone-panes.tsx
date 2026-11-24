import {
  Check,
  ChevronRight,
  CircleCheck,
  MessageSquare,
  RefreshCw,
  ShieldAlert,
  TerminalSquare,
  X,
} from "@compozy/ui/icons";

import { Button, cn, Empty, Pill, Spinner, StatusDot } from "@compozy/ui";

import {
  getSessionDisplayTitle,
  pendingInteractionReason,
  toSessionBadge,
  type SessionPayload,
} from "@/systems/session";

export interface HostedPhoneApprovalItem {
  id: string;
  requestId: string;
  turnId: string;
  sessionId: string;
  workspaceId: string;
  sessionTitle: string;
  projectName: string;
  title: string;
  command: string;
  kind: "permission" | "clarify";
}

function formatUpdatedAge(iso: string | undefined): string {
  if (!iso) return "";
  const parsed = Date.parse(iso);
  if (Number.isNaN(parsed)) return "";
  const seconds = Math.max(0, Math.floor((Date.now() - parsed) / 1000));
  if (seconds < 60) return "now";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h`;
  return `${Math.floor(hours / 24)}d`;
}

function sessionStatusTone(
  session: SessionPayload
): "success" | "warning" | "destructive" | "faint" {
  const badge = toSessionBadge(session);
  if (badge === "waiting-for-auth" || badge === "waiting-for-input") return "warning";
  if (session.state === "running" || session.state === "starting") return "success";
  if (session.state === "error") return "destructive";
  return "faint";
}

function sessionStatusLabel(session: SessionPayload): string {
  const badge = toSessionBadge(session);
  if (badge === "waiting-for-auth") return "Needs approval";
  if (badge === "waiting-for-input") return "Needs input";
  if (session.state === "running" || session.state === "starting") return "Running";
  if (session.state === "error") return "Error";
  if (session.state === "stopped") return "Stopped";
  return "Idle";
}

export interface HostedPhoneSessionListProps {
  computerName: string;
  sessions: readonly SessionPayload[];
  workspaceNameById: ReadonlyMap<string, string>;
  loading: boolean;
  error: Error | null;
  onRetry: () => void;
  onOpenSession: (session: SessionPayload) => void;
}

/**
 * Compact phone session list (`< 1024px`, U8): single-column list of the hosted
 * computer's sessions with status dot, workspace name, pending reason, and tap-to-open.
 */
export function HostedPhoneSessionList({
  computerName,
  sessions,
  workspaceNameById,
  loading,
  error,
  onRetry,
  onOpenSession,
}: HostedPhoneSessionListProps) {
  if (loading && sessions.length === 0) {
    return (
      <div className="flex flex-1 items-center justify-center p-6 text-small-body text-muted-foreground">
        <Spinner aria-hidden="true" className="mr-2 size-4" />
        Loading sessions…
      </div>
    );
  }

  if (error && sessions.length === 0) {
    return (
      <div className="flex flex-1 items-center justify-center p-6">
        <Empty
          action={
            <Button onClick={onRetry} size="sm" type="button" variant="secondary">
              <RefreshCw className="size-3.5" />
              Retry
            </Button>
          }
          description={error.message}
          icon={MessageSquare}
          title={`Couldn't load sessions on ${computerName}`}
        />
      </div>
    );
  }

  if (sessions.length === 0) {
    return (
      <div className="flex flex-1 items-center justify-center p-6">
        <Empty
          description={`Start a session on ${computerName} and it will appear here.`}
          icon={MessageSquare}
          title="No sessions yet"
        />
      </div>
    );
  }

  return (
    <div
      className="flex flex-1 flex-col divide-y divide-line-soft overflow-y-auto"
      data-testid="phone-sessions-list"
    >
      {sessions.map(session => {
        const title = getSessionDisplayTitle(session);
        const projectName = session.workspace_id
          ? (workspaceNameById.get(session.workspace_id) ?? session.workspace_path ?? "Project")
          : "User";
        const reason = pendingInteractionReason(session);
        const tone = sessionStatusTone(session);
        const statusText = sessionStatusLabel(session);
        const age = formatUpdatedAge(session.updated_at);

        return (
          <button
            className="flex min-h-16 w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-surface-2 active:bg-surface-2 focus-visible:shadow-focus-ring focus-visible:outline-none"
            data-session-id={session.id}
            key={session.id}
            onClick={() => onOpenSession(session)}
            type="button"
          >
            <StatusDot className="shrink-0" size="md" tone={tone} />
            <div className="flex min-w-0 flex-1 flex-col gap-0.5">
              <div className="flex items-center justify-between gap-2">
                <span className="truncate text-body font-medium text-fg-strong">{title}</span>
                {age ? (
                  <span className="shrink-0 text-eyebrow text-faint tabular-nums">{age}</span>
                ) : null}
              </div>
              <div className="flex items-center gap-1.5 text-eyebrow text-subtle">
                <span className="truncate">{projectName}</span>
                <span aria-hidden="true" className="text-faint">
                  ·
                </span>
                <span
                  className={cn(
                    "shrink-0",
                    tone === "warning" && "font-medium text-attn",
                    tone === "destructive" && "font-medium text-destructive"
                  )}
                >
                  {statusText}
                </span>
              </div>
              {reason ? (
                <p className="mt-0.5 truncate text-small-body text-muted-foreground">{reason}</p>
              ) : null}
            </div>
            <ChevronRight aria-hidden="true" className="size-4 shrink-0 text-faint" />
          </button>
        );
      })}
    </div>
  );
}

export interface HostedPhoneApprovalsPaneProps {
  approvals: readonly HostedPhoneApprovalItem[];
  pendingIds: ReadonlySet<string>;
  onApprove: (item: HostedPhoneApprovalItem) => void;
  onDeny: (item: HostedPhoneApprovalItem) => void;
  onOpenSession: (sessionId: string) => void;
}

/**
 * Compact phone Approvals tab (`< 1024px`, U8): surfaces pending command approvals
 * across sessions on this hosted computer with one-tap Approve / Deny.
 */
export function HostedPhoneApprovalsPane({
  approvals,
  pendingIds,
  onApprove,
  onDeny,
  onOpenSession,
}: HostedPhoneApprovalsPaneProps) {
  if (approvals.length === 0) {
    return (
      <div
        className="flex flex-1 items-center justify-center p-6"
        data-testid="phone-approvals-empty"
      >
        <Empty
          description="Pending tool and command approvals from your sessions will appear here."
          icon={CircleCheck}
          title="No pending approvals"
        />
      </div>
    );
  }

  return (
    <div
      className="flex flex-1 flex-col gap-3 overflow-y-auto p-4"
      data-testid="phone-approvals-list"
    >
      {approvals.map(item => {
        const busy = pendingIds.has(item.id);
        return (
          <div
            className="flex flex-col gap-3 rounded-lg border border-line bg-surface p-3.5"
            data-approval-request-id={item.id}
            key={item.id}
          >
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0">
                <div className="flex items-center gap-1.5">
                  <ShieldAlert aria-hidden="true" className="size-3.5 shrink-0 text-attn" />
                  <span className="truncate text-small-body font-medium text-fg-strong">
                    {item.sessionTitle}
                  </span>
                </div>
                <span className="text-eyebrow text-subtle">{item.projectName}</span>
              </div>
              <Pill size="xs" tone="warning">
                {item.kind === "permission" ? "Approval" : "Question"}
              </Pill>
            </div>

            <div className="rounded-md border border-line-soft bg-code-bg px-3 py-2 font-mono text-mono-id text-fg">
              {item.command || item.title}
            </div>

            {item.kind === "permission" ? (
              <div className="flex items-center gap-2">
                <Button
                  className="flex-1"
                  disabled={busy}
                  onClick={() => onDeny(item)}
                  size="sm"
                  type="button"
                  variant="outline"
                >
                  <X aria-hidden="true" className="size-3.5" />
                  Deny
                </Button>
                <Button
                  className="flex-1"
                  disabled={busy}
                  onClick={() => onApprove(item)}
                  size="sm"
                  type="button"
                >
                  <Check aria-hidden="true" className="size-3.5" />
                  Approve
                </Button>
              </div>
            ) : (
              <Button
                className="w-full"
                onClick={() => onOpenSession(item.sessionId)}
                size="sm"
                type="button"
                variant="secondary"
              >
                <TerminalSquare aria-hidden="true" className="size-3.5" />
                Open session to answer
              </Button>
            )}
          </div>
        );
      })}
    </div>
  );
}
