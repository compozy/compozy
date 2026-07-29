import { Check, X } from "lucide-react";

import { Receipt } from "@compozy/ui";

import type { PermissionDecision } from "../adapters/session-api";
import { normalizePermissionDecision, toPermissionRequest } from "../lib/message-parts";
import type { CompozyPermissionData, PermissionRequest } from "../types";

interface DecisionReceiptMeta {
  tone: "allowed" | "rejected";
  verb: string;
  scope: string;
}

const DECISION_RECEIPTS: Record<PermissionDecision, DecisionReceiptMeta> = {
  "allow-once": { tone: "allowed", verb: "Allowed", scope: " once" },
  "allow-always": { tone: "allowed", verb: "Allowed", scope: " for this session" },
  "reject-once": { tone: "rejected", verb: "Rejected", scope: "" },
  "reject-always": { tone: "rejected", verb: "Rejected", scope: " for this session" },
};

function permissionReceiptSubject(permission: PermissionRequest): string | null {
  const command = permission.toolInput.command;
  if (typeof command === "string" && command.trim().length > 0) {
    return command.trim();
  }
  const resource = permission.resource.trim();
  return resource.length > 0 ? resource : null;
}

export interface PermissionReceiptProps {
  permission: PermissionRequest;
  decision: PermissionDecision;
}

/**
 * One-line decision record: outcome glyph + sentence + mono subject. Allowed
 * decisions leave a receipt too — a durable audit line, not just rejections.
 */
export function PermissionReceipt({ permission, decision }: PermissionReceiptProps) {
  const meta = DECISION_RECEIPTS[decision];
  const subject = permissionReceiptSubject(permission);
  return (
    <Receipt
      tone={meta.tone}
      icon={meta.tone === "allowed" ? <Check strokeWidth={2} /> : <X strokeWidth={2} />}
      data-testid={
        meta.tone === "allowed" ? "permission-allowed-receipt" : "permission-rejected-notice"
      }
      data-decision={decision}
    >
      {meta.verb} <code>{permission.toolName}</code>
      {meta.scope}
      {subject ? (
        <>
          {" "}
          — <code>{subject}</code>
        </>
      ) : null}
    </Receipt>
  );
}

/**
 * Transcript record for a permission data part. Pending asks live on the
 * composer decision dock — the transcript keeps only one-line receipts, for
 * both allowed and rejected outcomes.
 */
export function PermissionDataPart({ data }: { data: CompozyPermissionData }) {
  const decision = normalizePermissionDecision(data.decision);
  if (decision === null) {
    return null;
  }
  return <PermissionReceipt permission={toPermissionRequest(data)} decision={decision} />;
}
