import { ArrowLeft, TerminalSquare } from "@compozy/ui/icons";

import { Button, StatusDot } from "@compozy/ui";

import { SessionThread } from "@/components/assistant-ui/session-thread";
import {
  canPromptSession,
  getSessionDisplayTitle,
  isSessionRunning,
  SessionChatRuntimeProvider,
  SessionPromptRuntimeProvider,
  type SessionPayload,
} from "@/systems/session";

export interface HostedPhoneSessionViewProps {
  session: SessionPayload;
  projectName: string;
  canOperateTerminal: boolean;
  onBack: () => void;
  onOpenTerminal?: () => void;
}

/**
 * Full-screen phone session thread view (`< 1024px`, U8): composes the real
 * `SessionPromptRuntimeProvider`, `SessionChatRuntimeProvider`, and `SessionThread`
 * beneath a compact phone navigation bar.
 */
export function HostedPhoneSessionView({
  session,
  projectName,
  canOperateTerminal,
  onBack,
  onOpenTerminal,
}: HostedPhoneSessionViewProps) {
  const title = getSessionDisplayTitle(session);
  const workspaceId = session.workspace_id?.trim() ?? "";
  const canPrompt = canPromptSession(session) && workspaceId !== "";
  const running = isSessionRunning(session);
  const readOnly = workspaceId === "";

  return (
    <div
      className="flex min-h-0 flex-1 flex-col overflow-hidden bg-background"
      data-testid="phone-session-view"
    >
      <div className="flex h-12 shrink-0 items-center justify-between gap-2 border-b border-line-soft bg-rail px-2">
        <div className="flex min-w-0 items-center gap-1.5">
          <Button
            aria-label="Back to sessions"
            onClick={onBack}
            size="icon-sm"
            type="button"
            variant="ghost"
          >
            <ArrowLeft className="size-4" />
          </Button>
          <StatusDot size="sm" tone={running ? "success" : "faint"} />
          <div className="flex min-w-0 flex-col">
            <span className="truncate text-small-body font-medium text-fg-strong">{title}</span>
            <span className="truncate text-eyebrow text-subtle">{projectName}</span>
          </div>
        </div>

        {canOperateTerminal && onOpenTerminal ? (
          <Button
            aria-label="Open terminal"
            onClick={onOpenTerminal}
            size="icon-sm"
            type="button"
            variant="ghost"
          >
            <TerminalSquare className="size-4" />
          </Button>
        ) : null}
      </div>

      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
        <SessionPromptRuntimeProvider canPrompt={canPrompt} session={session}>
          <SessionChatRuntimeProvider
            liveTailEnabled={!readOnly}
            readOnly={readOnly}
            sessionId={session.id}
            workspaceId={workspaceId}
          >
            <SessionThread
              acpSessionId={session.runtime.acp_session_id}
              agentName={session.agent_name}
              canPrompt={canPrompt}
              failure={session.failure}
              isSessionRunning={running}
              liveDataEnabled={!readOnly}
              messageTrail={false}
              readOnly={readOnly}
              sessionId={session.id}
              sessionState={session.state}
              statusSession={session}
              workspaceId={workspaceId}
            />
          </SessionChatRuntimeProvider>
        </SessionPromptRuntimeProvider>
      </div>
    </div>
  );
}
