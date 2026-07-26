import { Check, PenLine, X } from "lucide-react";

import { Button, Eyebrow, StatusCard } from "@agh/ui";

import type {
  LoopApprovalFact,
  LoopApprovalRequest,
  LoopGateDecision,
} from "../../lib/loop-events";
import type { LoopRunRecord } from "../../types";
import { LoopRunSection } from "./loop-run-section";

interface LoopRunNeedsYouCardProps {
  run: LoopRunRecord;
  /** The `needs_approval` payload; null before the frame replays. */
  request: LoopApprovalRequest | null;
  /** Usage-snapshot facts standing in when the payload carries none (§3). */
  fallbackFacts: LoopApprovalFact[];
  isPending?: boolean;
  onDecision: (decision: LoopGateDecision, gateId: string) => void;
}

/** The closed ADR-017 §9.8 verdict set, rendered as the card's action buttons in order. */
const GATE_DECISIONS = [
  {
    decision: "approve",
    testId: "loop-approval-approve",
    variant: "primary",
    className: undefined,
    icon: Check,
    label: "Approve & resume",
  },
  {
    decision: "request_changes",
    testId: "loop-approval-request-changes",
    variant: "outline",
    className: undefined,
    icon: PenLine,
    label: "Request changes",
  },
  {
    decision: "reject",
    testId: "loop-approval-reject",
    variant: "outline",
    className: "text-danger hover:border-danger/40 hover:text-danger",
    icon: X,
    label: "Reject & halt",
  },
] as const;

/**
 * The "Needs you" card (§2/§7): a warning StatusCard at the top of the main column
 * with the approver's prompt, the decision facts, and the closed decision set —
 * Approve & resume / Request changes / Reject & halt — wired with the gate id.
 */
export function LoopRunNeedsYouCard({
  run,
  request,
  fallbackFacts,
  isPending,
  onDecision,
}: LoopRunNeedsYouCardProps) {
  const gateId = request?.gateId ?? run.active_gate_id ?? "approve";
  const facts = request?.facts && request.facts.length > 0 ? request.facts : fallbackFacts;
  const micro =
    gateId === "budget"
      ? `needs_approval · ${gateId} · on_exceeded: ${run.budget_on_exceeded}`
      : `needs_approval · ${gateId}`;
  return (
    <LoopRunSection label="Needs you" data-testid="loop-run-needs-you">
      <StatusCard
        className="gap-0 border border-warning/25 bg-warning-tint px-4 py-3.5"
        tone="warning"
      >
        <div className="text-ws-name font-medium text-warning">
          {request?.title ?? "Approve to continue this run?"}
        </div>
        <StatusCard.Body className="mt-1 max-w-[62ch] leading-relaxed">
          {request?.prompt ??
            "The run parked and asks you first. Approving continues the round with the same limits; rejecting ends the run."}
        </StatusCard.Body>
        {facts.length > 0 ? (
          <div className="mt-3 flex flex-wrap gap-x-5.5 gap-y-2">
            {facts.map(fact => (
              <div key={fact.label} className="flex flex-col gap-0.5" data-testid="loop-run-fact">
                <Eyebrow className="text-faint">{fact.label}</Eyebrow>
                <span className="font-mono text-mono-id tabular-nums text-fg">{fact.value}</span>
              </div>
            ))}
          </div>
        ) : null}
        <StatusCard.Action className="mt-3.5">
          {GATE_DECISIONS.map(({ decision, testId, variant, className, icon: Icon, label }) => (
            <Button
              key={decision}
              className={className}
              data-testid={testId}
              disabled={isPending}
              onClick={() => onDecision(decision, gateId)}
              size="sm"
              type="button"
              variant={variant}
            >
              <Icon aria-hidden="true" className="size-3.5" />
              {label}
            </Button>
          ))}
        </StatusCard.Action>
        <div className="mt-3 font-mono text-pill-group-badge text-faint">{micro}</div>
      </StatusCard>
    </LoopRunSection>
  );
}
