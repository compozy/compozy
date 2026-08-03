import type { ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import { ArrowRight } from "lucide-react";

import { Button, formatRelativeTime } from "@compozy/ui";

import {
  isOpenWait,
  type LoopNodeLifecycle,
  type LoopNodeWaitView,
} from "../../lib/loop-node-lifecycle";
import { LoopRunSection } from "./loop-run-section";

interface LoopRunWaitingPanelProps {
  nodes: readonly LoopNodeLifecycle[];
  renderNodeActions?: (node: LoopNodeLifecycle) => ReactNode;
}

interface LoopRunAttentionPanelProps {
  nodes: readonly LoopNodeLifecycle[];
  renderNodeActions?: (node: LoopNodeLifecycle) => ReactNode;
  /** Opens the quarantine entry for the node an attention flag names. */
  onOpenQuarantine?: (node: LoopNodeLifecycle) => void;
}

/** `1080` seconds → `18m` — the wait-age form the daemon's `age_seconds` feeds. */
function ageLabel(seconds: number): string {
  if (seconds < 60) return `${Math.max(seconds, 0)}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h`;
  return `${Math.floor(hours / 24)}d`;
}

const WAIT_KIND_SENTENCE: Record<string, string> = {
  timer: "Sleeping on a timer — it resumes by itself.",
  event: "Waiting for a matching event. A lane's wait is its own; nothing else is held up.",
  approval_escalation:
    "The ladder walks its steps until you decide; a decision at any point wins and cancels the rest.",
};

const WAIT_KIND_TITLE: Record<string, string> = {
  timer: "is waiting on a timer",
  event: "is waiting for an event",
  approval_escalation: "is waiting for your decision",
};

/**
 * The Waiting panel (US-021/US-022): one row per open wait cell with the
 * daemon's own age. Every value — kind, resume time, escalation cursor, age —
 * comes from the wait row; nothing is computed from wall-clock guesses here.
 */
export function LoopRunWaitingPanel({ nodes, renderNodeActions }: LoopRunWaitingPanelProps) {
  const waits: { node: LoopNodeLifecycle; wait: LoopNodeWaitView }[] = [];
  for (const node of nodes) {
    for (const wait of node.waits) {
      if (isOpenWait(wait)) waits.push({ node, wait });
    }
  }
  if (waits.length === 0) return null;
  return (
    <LoopRunSection
      data-testid="loop-run-waiting"
      label="Waiting"
      right={
        <span className="font-mono text-mono-id text-subtle">
          {waits.length} {waits.length === 1 ? "wait" : "waits"}
        </span>
      }
    >
      <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
        {waits.map(({ node, wait }, index) => (
          <div
            className={`flex items-start gap-3 px-4 py-3.5 ${index > 0 ? "border-t border-line-soft" : ""}`}
            data-testid={`loop-run-wait-${node.nodeId}-${wait.itemIndex}`}
            key={`${node.nodeId}-${wait.itemIndex}`}
          >
            <span aria-hidden="true" className="mt-1.5 size-2 shrink-0 rounded-full bg-info" />
            <div className="min-w-0 flex-1">
              <div className="text-ws-name font-medium text-fg-strong">
                {node.label} {WAIT_KIND_TITLE[wait.kind] ?? "is waiting"}
              </div>
              <div className="mt-0.75 max-w-[60ch] text-small-body leading-relaxed text-muted">
                {WAIT_KIND_SENTENCE[wait.kind] ?? "This lane is parked until its wait resolves."}
              </div>
              <div className="mt-1.5 font-mono text-pill-group-badge text-faint">
                {[
                  wait.kind,
                  wait.resumeAt ? `resumes ${formatRelativeTime(wait.resumeAt)}` : null,
                  wait.claimState === "intervention_required" ? "needs a decision" : null,
                  `${node.nodeId}[${wait.itemIndex}] · gen ${wait.generation}`,
                ]
                  .filter(Boolean)
                  .join(" · ")}
              </div>
            </div>
            <span className="flex shrink-0 items-center gap-2 pt-0.5">
              <span className="font-mono text-mono-id whitespace-nowrap text-subtle">
                waiting {ageLabel(wait.ageSeconds)}
              </span>
              {renderNodeActions?.(node)}
            </span>
          </div>
        ))}
        <div className="border-t border-line-soft px-4 py-2.5">
          <Link
            className="inline-flex items-center gap-1.25 text-badge font-medium text-muted hover:text-fg-strong"
            data-testid="loop-run-waiting-inventory-link"
            search={{ nodes: "waiting" }}
            to="/loop-runs"
          >
            All waits in the inventory
            <ArrowRight aria-hidden="true" className="size-3" />
          </Link>
        </div>
      </div>
    </LoopRunSection>
  );
}

const ATTENTION_COPY: Record<string, { title: string; body: string }> = {
  silence: {
    title: "has gone quiet",
    body: "No output, no tool call, no heartbeat — and no confirmed death either. Watch flag only: it clears itself on any evidence of life.",
  },
  dependency_quarantined: {
    title: "can't move forward",
    body: "Forward progress needs a lane that is set aside. The run parks here and says so — it never waits silently or fails without a reason.",
  },
  expired_wait: {
    title: "waited past its deadline",
    body: "The wait expired and this loop declares no timeout route, so nothing decided what happens next.",
  },
};

/**
 * The Needs-attention panel (US-007 EC-2 / US-024 AC-4). An attention flag is
 * evidence, not a stop: the run keeps going, and the panel names the flag the
 * daemon wrote plus whatever it recorded as the reason. Flags carry warning
 * tone because they are a state the runtime reports, never decoration.
 */
export function LoopRunAttentionPanel({
  nodes,
  renderNodeActions,
  onOpenQuarantine,
}: LoopRunAttentionPanelProps) {
  if (nodes.length === 0) return null;
  return (
    <LoopRunSection
      data-testid="loop-run-attention"
      label="Needs attention"
      right={<span className="font-mono text-mono-id text-subtle">{nodes.length} flagged</span>}
    >
      <div className="grid gap-3 min-[900px]:grid-cols-2">
        {nodes.map(node => {
          const copy = ATTENTION_COPY[node.attentionFlag];
          return (
            <div
              className="rounded-lg border border-warning/25 bg-warning-tint px-4 py-3.5"
              data-testid={`loop-run-attention-${node.nodeId}`}
              key={node.nodeId}
            >
              <div className="text-ws-name font-medium text-warning">
                {copy ? `${node.label} ${copy.title}` : `${node.label} needs attention`}
              </div>
              <p className="mt-1 max-w-[60ch] text-small-body leading-relaxed text-muted">
                {copy?.body ?? node.attentionReason}
              </p>
              <div className="mt-2 font-mono text-pill-group-badge text-faint">
                {[
                  "node_attention_flagged",
                  node.attentionFlag,
                  node.lastEvidenceAt
                    ? `last evidence ${formatRelativeTime(node.lastEvidenceAt)}`
                    : null,
                  `${node.nodeId} · gen ${node.generation}`,
                ]
                  .filter(Boolean)
                  .join(" · ")}
              </div>
              <div className="mt-3 flex items-center gap-2">
                {node.attentionFlag === "dependency_quarantined" && onOpenQuarantine ? (
                  <Button
                    className="min-h-6"
                    data-testid={`loop-run-attention-open-quarantine-${node.nodeId}`}
                    onClick={() => onOpenQuarantine(node)}
                    size="sm"
                    type="button"
                    variant="outline"
                  >
                    Open quarantine entry
                  </Button>
                ) : null}
                {renderNodeActions?.(node)}
              </div>
            </div>
          );
        })}
      </div>
    </LoopRunSection>
  );
}
