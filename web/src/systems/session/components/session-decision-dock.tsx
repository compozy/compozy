import { useState } from "react";

import { Dock } from "@compozy/ui";

import { useSessionClarifications } from "../hooks/use-session-clarifications";
import { useSessionDecisionMessages } from "../hooks/use-session-transcript-thread-messages";
import { derivePendingPermissions } from "../lib/pending-permissions";
import { ClarificationDock } from "./clarification-dock";
import { PermissionDock } from "./permission-dock";

export interface SessionDecisionDockProps {
  enabled?: boolean;
  sessionId: string;
  workspaceId: string;
}

/**
 * Composer-zone decision host: permission and clarification asks leave the
 * transcript and dock here, one at a time — permissions first, then pending
 * clarifications — with a mono "1/N" counter when decisions queue up. The
 * transcript keeps only one-line receipts.
 */
export function SessionDecisionDock({
  enabled = true,
  sessionId,
  workspaceId,
}: SessionDecisionDockProps) {
  const decisionMessages = useSessionDecisionMessages();
  const clarifications = useSessionClarifications(workspaceId, sessionId, { enabled });
  // Locally-resolved ids hide a decided ask before the durable transcript /
  // pending-list catch up, so the next queued decision surfaces immediately.
  const [resolvedIds, setResolvedIds] = useState<ReadonlySet<string>>(() => new Set());
  const markResolved = (id: string) =>
    setResolvedIds(previous => {
      const next = new Set(previous);
      next.add(id);
      return next;
    });

  const pendingPermissions = derivePendingPermissions(decisionMessages).filter(
    permission => !resolvedIds.has(permission.requestId)
  );
  const pendingClarifications = clarifications.error
    ? []
    : (clarifications.data ?? []).filter(
        clarification => !resolvedIds.has(clarification.request_id)
      );
  const permission = pendingPermissions[0];
  if (permission) {
    const total = pendingPermissions.length + pendingClarifications.length;
    return (
      <PermissionDock
        enabled={enabled}
        key={permission.requestId}
        permission={permission}
        sessionId={sessionId}
        workspaceId={workspaceId}
        countLabel={total > 1 ? `1/${total}` : null}
        onResolved={() => markResolved(permission.requestId)}
      />
    );
  }

  if (clarifications.error) {
    return (
      <Dock role="region" aria-label="Decision status" data-testid="session-decision-dock-error">
        <Dock.Status className="mt-0" role="alert" tone="danger">
          Couldn’t load pending questions.
        </Dock.Status>
      </Dock>
    );
  }

  const total = pendingClarifications.length;
  if (total === 0) {
    return null;
  }
  const countLabel = total > 1 ? `1/${total}` : null;

  const clarification = pendingClarifications[0];
  if (!clarification) {
    return null;
  }
  return (
    <ClarificationDock
      enabled={enabled}
      key={clarification.request_id}
      clarification={clarification}
      sessionId={sessionId}
      workspaceId={workspaceId}
      countLabel={countLabel}
      onResolved={() => markResolved(clarification.request_id)}
    />
  );
}
