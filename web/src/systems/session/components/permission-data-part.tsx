import { Check, RotateCcw, X } from "lucide-react";

import { Receipt, Time } from "@compozy/ui";

import type { PermissionDecision } from "../adapters/session-api";
import { useSessionRuntimeRenderContext } from "../hooks/use-session-runtime-render-context";
import { normalizePermissionDecision, toPermissionRequest } from "../lib/message-parts";
import {
  permissionExpiredReceiptCopy,
  permissionReceiptCopy,
} from "../lib/permission-receipt-copy";
import {
  interactionExpiredByRestart,
  permissionDecisionActor,
  type PermissionDecisionActor,
} from "../lib/session-pending-interactions";
import type { CompozyPermissionData, PermissionRequest, SessionInteractionRecord } from "../types";
import { PermissionWaitingLine } from "./permission-waiting-line";

export interface PermissionReceiptProps {
  permission: PermissionRequest;
  decision: PermissionDecision;
  /**
   * Who settled the ask, from the daemon's resolved interaction row. Defaults to
   * `unknown`, which names nobody: the transcript part alone never proves an actor.
   */
  actor?: PermissionDecisionActor;
  askedAt?: string;
}

/**
 * One-line decision record: outcome glyph + sentence + mono subject. Allowed
 * decisions leave a receipt too — a durable audit line, not just rejections.
 */
export function PermissionReceipt({
  permission,
  decision,
  actor = "unknown",
  askedAt,
}: PermissionReceiptProps) {
  const copy = permissionReceiptCopy(permission, decision, actor);
  return (
    <Receipt
      tone={copy.tone}
      icon={copy.tone === "allowed" ? <Check strokeWidth={2} /> : <X strokeWidth={2} />}
      data-testid={
        copy.tone === "allowed" ? "permission-allowed-receipt" : "permission-rejected-notice"
      }
      data-decision={decision}
      data-actor={actor}
    >
      {copy.prefix}
      {copy.subject ? (
        <>
          {copy.join}
          <code>{copy.subject}</code>
        </>
      ) : null}
      {copy.suffix}
      {askedAt ? (
        <>
          {" "}
          · <Time iso={askedAt} />
        </>
      ) : null}
    </Receipt>
  );
}

export interface PermissionExpiredReceiptProps {
  permission: PermissionRequest;
  interaction: SessionInteractionRecord;
  askedAt?: string;
}

/**
 * Receipt for an ask the daemon settled without a decision: the transcript never records
 * an answer, so the durable interaction row is the evidence. Neutral tone — nobody
 * decided, nothing was refused. The restart is named only when the row's resolution is
 * `failed-by-restart`; any other canceled row reads as a plain cancellation.
 */
export function PermissionExpiredReceipt({
  permission,
  interaction,
  askedAt,
}: PermissionExpiredReceiptProps) {
  const cause = interactionExpiredByRestart(interaction) ? "restart" : "canceled";
  const copy = permissionExpiredReceiptCopy(permission, cause);
  return (
    <Receipt
      tone="neutral"
      icon={cause === "restart" ? <RotateCcw strokeWidth={2} /> : <X strokeWidth={2} />}
      data-testid="permission-expired-receipt"
      data-cause={cause}
      data-resolution={interaction.resolution}
      data-resolved-by={interaction.resolved_by}
    >
      {copy.prefix}
      {copy.subject ? <code>{copy.subject}</code> : null}
      {copy.suffix}
      {askedAt ? (
        <>
          {" "}
          · asked <Time iso={askedAt} />
        </>
      ) : null}
    </Receipt>
  );
}

/**
 * Transcript record for a permission data part. Pending terminal asks leave an
 * honest waiting line here; the composer dock still owns the buttons. Generic
 * pending asks stay dock-only so the transcript does not double the ask. An ask
 * the daemon already settled without a decision (canceled — at a restart or
 * otherwise) is never "waiting": it renders the receipt from the durable
 * interaction row instead. A recorded decision names its actor only from the
 * daemon's resolved row; without one the receipt stays neutral.
 */
export function PermissionDataPart({ data }: { data: CompozyPermissionData }) {
  const renderContext = useSessionRuntimeRenderContext();
  const expiredInteractions = renderContext?.expiredInteractions;
  const permission = toPermissionRequest(data);
  const decision = normalizePermissionDecision(data.decision);
  if (decision !== null) {
    const actor = permissionDecisionActor(renderContext?.resolvedInteractions.get(data.request_id));
    return (
      <PermissionReceipt
        actor={actor}
        askedAt={data.timestamp}
        decision={decision}
        permission={permission}
      />
    );
  }
  const expired = expiredInteractions?.get(data.request_id);
  if (expired) {
    return (
      <PermissionExpiredReceipt
        askedAt={data.timestamp}
        interaction={expired}
        permission={permission}
      />
    );
  }
  return <PermissionWaitingLine askedAt={data.timestamp} permission={permission} />;
}
