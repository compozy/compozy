import { Check, X } from "lucide-react";

import { Receipt, Time } from "@compozy/ui";

import type { PermissionDecision } from "../adapters/session-api";
import { normalizePermissionDecision, toPermissionRequest } from "../lib/message-parts";
import { permissionReceiptCopy } from "../lib/permission-receipt-copy";
import type { CompozyPermissionData, PermissionRequest } from "../types";
import { PermissionWaitingLine } from "./permission-waiting-line";

export interface PermissionReceiptProps {
  permission: PermissionRequest;
  decision: PermissionDecision;
  askedAt?: string;
}

/**
 * One-line decision record: outcome glyph + sentence + mono subject. Allowed
 * decisions leave a receipt too — a durable audit line, not just rejections.
 */
export function PermissionReceipt({ permission, decision, askedAt }: PermissionReceiptProps) {
  const copy = permissionReceiptCopy(permission, decision);
  return (
    <Receipt
      tone={copy.tone}
      icon={copy.tone === "allowed" ? <Check strokeWidth={2} /> : <X strokeWidth={2} />}
      data-testid={
        copy.tone === "allowed" ? "permission-allowed-receipt" : "permission-rejected-notice"
      }
      data-decision={decision}
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

/**
 * Transcript record for a permission data part. Pending terminal asks leave an
 * honest waiting line here; the composer dock still owns the buttons. Generic
 * pending asks stay dock-only so the transcript does not double the ask.
 */
export function PermissionDataPart({ data }: { data: CompozyPermissionData }) {
  const permission = toPermissionRequest(data);
  const decision = normalizePermissionDecision(data.decision);
  if (decision === null) {
    return <PermissionWaitingLine askedAt={data.timestamp} permission={permission} />;
  }
  return <PermissionReceipt askedAt={data.timestamp} decision={decision} permission={permission} />;
}
