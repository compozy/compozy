import { Bell, Check, PenLine, ShieldAlert, TriangleAlert, X } from "lucide-react";

import { Button, Eyebrow, formatRelativeTime } from "@compozy/ui";

import type {
  LoopApprovalFact,
  LoopApprovalRequest,
  LoopGateDecision,
} from "../../lib/loop-events";
import { isTerminalLoopStatus } from "../../lib/loop-formatters";
import type { LoopNodeLifecycle } from "../../lib/loop-node-lifecycle";
import { humanizeLoopNodeId } from "../../lib/loop-node-labels";
import type { LoopRequestView } from "../../lib/loop-request-model";
import type { LoopRunRecord } from "../../types";
import { LoopSection } from "../loop-section";
import {
  LoopRequestQuestionnaire,
  type LoopRequestFocusTarget,
  type LoopRunRequestState,
} from "./requests/loop-request-questionnaire";

interface LoopRunNeedsYouCardProps {
  run: LoopRunRecord;
  /** The `needs_approval` payload; null before the frame replays. */
  request: LoopApprovalRequest | null;

  fallbackFacts: LoopApprovalFact[];
  isPending?: boolean;
  /** When true, the approval decision block renders. */
  showApproval?: boolean;
  /** Quarantined lanes that need an entry point above the fold. */
  quarantinedNodes?: readonly LoopNodeLifecycle[];

  requests?: readonly LoopRequestView[];
  requestFocus?: LoopRequestFocusTarget;
  requestState?: LoopRunRequestState;
  workspaceId?: string;
  onOpenQuarantine?: (nodeId: string) => void;
  onDecision: (decision: LoopGateDecision, gateId: string) => void;
}

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

function nodeIdentity(node: LoopNodeLifecycle): string {
  const item = node.itemIndex === null ? "" : `[${node.itemIndex}]`;
  return `${node.nodeId}${item} · gen ${node.generation}`;
}

function nodeRowKey(node: LoopNodeLifecycle): string {
  return `${node.nodeId}${node.itemIndex === null ? "" : `-${node.itemIndex}`}-g${node.generation}`;
}

const NO_QUARANTINED_NODES: readonly LoopNodeLifecycle[] = [];
const NO_REQUEST_VIEWS: readonly LoopRequestView[] = [];

/**
 * Requeue is a live verb: the daemon rejects it once the run has ended, so a
 * terminal row keeps the reason and the entry but stops asking for one.
 */
function quarantineDetail(attempts: number, runEnded: boolean): string {
  const setAside = attempts > 0 ? `Set aside after ${attempts} attempts.` : "Set aside.";
  return runEnded
    ? `${setAside} This run has ended.`
    : `${setAside} Requeue it from the entry once it is repaired.`;
}

/**
 * The "Needs you" region: a neutral panelbox whose only colour is the warning
 * glyph. Requests present as a one-at-a-time questionnaire; approval decisions
 * and quarantine entries share the same shell.
 */
export function LoopRunNeedsYouCard({
  run,
  request,
  fallbackFacts,
  isPending,
  showApproval = true,
  quarantinedNodes = NO_QUARANTINED_NODES,
  requests = NO_REQUEST_VIEWS,
  requestFocus,
  requestState,
  workspaceId = "",
  onOpenQuarantine,
  onDecision,
}: LoopRunNeedsYouCardProps) {
  if (!showApproval && quarantinedNodes.length === 0 && requests.length === 0) return null;
  const gateId = request?.gateId ?? run.active_gate_id ?? "approve";
  const facts = request?.facts && request.facts.length > 0 ? request.facts : fallbackFacts;
  // Who is asking, and from which round — the "who asks" half of the card's
  // stated anatomy. This line used to read `needs_approval · <gate id>`, and
  // `needs_approval · budget · on_exceeded: <enum>` on the budget gate: two wire
  // enums and a machine id, printed in the default register that task_05
  // requirement 1 forbids them from. The budget policy is not restated either —
  // the usage rail already says it in a sentence (`loop-run-usage.ts:146`).
  const gateName = request?.gateId ?? run.active_gate_id ?? "";
  const micro = [
    gateName === "" ? null : humanizeLoopNodeId(gateName),
    run.generation > 0 ? `round ${run.generation}` : null,
  ]
    .filter((part): part is string => part !== null)
    .join(" · ");
  const gistCount = (showApproval ? 1 : 0) + quarantinedNodes.length + requests.length;
  const runEnded = isTerminalLoopStatus(run.status);
  return (
    <LoopSection
      className="mb-0"
      data-testid="loop-run-needs-you"
      gist={`${gistCount} ${gistCount === 1 ? "item" : "items"}`}
      icon={<Bell aria-hidden="true" />}
      title="Needs you"
    >
      <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
        {requests.length > 0 ? (
          <LoopRequestQuestionnaire
            requestFocus={requestFocus}
            requests={requests}
            requestState={requestState}
            workspaceId={workspaceId}
          />
        ) : null}
        {showApproval ? (
          <div
            className={`flex items-start gap-3 px-4 py-3.5${
              requests.length > 0 ? " border-t border-line-soft" : ""
            }`}
            data-testid="loop-run-needs-approval"
          >
            <TriangleAlert aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-warning" />
            <div className="min-w-0 flex-1">
              <div className="text-ws-name font-medium text-fg-strong">
                {request?.title ?? "Approve to continue this run?"}
              </div>
              <p className="mt-1 max-w-[62ch] text-small-body leading-relaxed text-muted">
                {request?.prompt ??
                  "The run parked and asks you first. Approving continues the round with the same limits; rejecting ends the run."}
              </p>
              {facts.length > 0 ? (
                <div className="mt-3 flex flex-wrap gap-x-5.5 gap-y-2">
                  {facts.map(fact => (
                    <div
                      key={fact.label}
                      className="flex flex-col gap-0.5"
                      data-testid="loop-run-fact"
                    >
                      <Eyebrow className="text-faint">{fact.label}</Eyebrow>
                      <span className="font-mono text-mono-id tabular-nums text-fg">
                        {fact.value}
                      </span>
                    </div>
                  ))}
                </div>
              ) : null}
              <div className="mt-3.5 flex flex-wrap gap-2">
                {GATE_DECISIONS.map(
                  ({ decision, testId, variant, className, icon: Icon, label }) => (
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
                  )
                )}
              </div>
              {micro === "" ? null : (
                // Plain language, so not the mono face the ids and fixed
                // operational values wear.
                <div
                  className="mt-3 text-form-hint text-faint"
                  data-testid="loop-run-needs-approval-origin"
                >
                  {micro}
                </div>
              )}
            </div>
          </div>
        ) : null}
        {quarantinedNodes.map((node, index) => {
          const attempts = node.quarantineEntry?.attemptCount ?? 0;
          const rowKey = nodeRowKey(node);
          return (
            <div
              className={`flex items-start gap-3 px-4 py-3.5 ${
                showApproval || requests.length > 0 || index > 0 ? "border-t border-line-soft" : ""
              }`}
              data-testid={`loop-run-needs-quarantine-${rowKey}`}
              key={rowKey}
            >
              <ShieldAlert aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-warning" />
              <div className="min-w-0 flex-1">
                <div className="text-ws-name font-medium text-fg-strong">
                  {node.label} was quarantined
                </div>
                <p
                  className="mt-0.75 max-w-[62ch] text-small-body leading-relaxed text-muted"
                  data-testid={`loop-run-needs-quarantine-detail-${rowKey}`}
                >
                  {quarantineDetail(attempts, runEnded)}
                </p>
                <div className="mt-1.5 font-mono text-pill-group-badge text-faint">
                  {`node_controls.quarantined true · ${nodeIdentity(node)}`}
                </div>
              </div>
              <span className="flex shrink-0 items-center gap-2 pt-0.5">
                {node.quarantinedAt ? (
                  <span className="font-mono text-mono-id whitespace-nowrap text-subtle">
                    {formatRelativeTime(node.quarantinedAt)}
                  </span>
                ) : null}
                {onOpenQuarantine ? (
                  <Button
                    className="min-h-6 shrink-0"
                    data-testid={`loop-run-needs-open-quarantine-${rowKey}`}
                    onClick={() => onOpenQuarantine(node.nodeId)}
                    size="sm"
                    type="button"
                    variant="outline"
                  >
                    <ShieldAlert aria-hidden="true" className="size-3.5" />
                    Open entry
                  </Button>
                ) : null}
              </span>
            </div>
          );
        })}
      </div>
    </LoopSection>
  );
}
