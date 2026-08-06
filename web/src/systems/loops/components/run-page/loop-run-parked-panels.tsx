import type { ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import { ArrowRight } from "lucide-react";

import { Button, formatRelativeTime } from "@compozy/ui";

import {
  isOpenWait,
  type LoopNodeLifecycle,
  type LoopNodeWaitView,
} from "../../lib/loop-node-lifecycle";
import { humanizeLoopNodeId } from "../../lib/loop-run-story-rows";
import { LoopRunSection } from "./loop-run-section";

interface LoopRunWaitingPanelProps {
  nodes: readonly LoopNodeLifecycle[];
  renderNodeActions?: (node: LoopNodeLifecycle) => ReactNode;
}

interface LoopRunAttentionPanelProps {
  nodes: readonly LoopNodeLifecycle[];
  renderNodeActions?: (node: LoopNodeLifecycle) => ReactNode;
  /** Opens the quarantine entry for the quarantined producer node id. */
  onOpenQuarantine?: (nodeId: string) => void;
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

const ATTENTION_TITLE: Record<string, string> = {
  silence: "has gone quiet",
  expired_wait: "waited past its deadline",
};

const ATTENTION_BODY: Record<string, string> = {
  silence:
    "No output, no tool call, no heartbeat — and no confirmed death either. The flag clears itself on any evidence of life.",
  expired_wait:
    "The wait expired and this loop declares no timeout route, so nothing decided what happens next.",
};

interface DependencyAttentionGroup {
  producerNodeId: string;
  consumers: LoopNodeLifecycle[];
}

/** Consumers parked behind the same quarantined producer collapse to one row. */
function groupDependencyAttention(nodes: readonly LoopNodeLifecycle[]): {
  groups: DependencyAttentionGroup[];
  singles: LoopNodeLifecycle[];
} {
  const byProducer = new Map<string, LoopNodeLifecycle[]>();
  const singles: LoopNodeLifecycle[] = [];
  for (const node of nodes) {
    if (node.attentionFlag !== "dependency_quarantined" || !node.attentionProducerNodeId) {
      singles.push(node);
      continue;
    }
    const producer = node.attentionProducerNodeId;
    const members = byProducer.get(producer);
    if (members) {
      members.push(node);
    } else {
      byProducer.set(producer, [node]);
    }
  }
  const groups = [...byProducer.entries()]
    .map(([producerNodeId, consumers]) => ({ producerNodeId, consumers }))
    .sort((left, right) => left.producerNodeId.localeCompare(right.producerNodeId));
  return { groups, singles };
}

function joinLabels(labels: string[]): string {
  if (labels.length <= 1) return labels[0] ?? "";
  return `${labels.slice(0, -1).join(", ")} and ${labels.at(-1)}`;
}

function DependencyAttentionRow({
  group,
  withDivider,
  onOpenQuarantine,
}: {
  group: DependencyAttentionGroup;
  withDivider: boolean;
  onOpenQuarantine?: (nodeId: string) => void;
}) {
  const producerLabel = group.producerNodeId
    ? humanizeLoopNodeId(group.producerNodeId)
    : "a parked step";
  const consumerLabels = joinLabels(group.consumers.map(node => node.label));
  const generation = group.consumers[0]?.generation;
  return (
    <div
      className={`flex items-start gap-3 px-4 py-3.5 ${withDivider ? "border-t border-line-soft" : ""}`}
      data-testid={`loop-run-attention-producer-${group.producerNodeId || "unknown"}`}
    >
      <span aria-hidden="true" className="mt-1.5 size-2 shrink-0 rounded-full bg-warning" />
      <div className="min-w-0 flex-1">
        <div className="text-ws-name font-medium text-warning">{producerLabel} is quarantined</div>
        <p className="mt-0.75 max-w-[62ch] text-small-body leading-relaxed text-muted">
          {consumerLabels
            ? `${consumerLabels} ${group.consumers.length === 1 ? "is" : "are"} parked behind it until it is requeued.`
            : "Downstream steps are parked behind it until it is requeued."}
        </p>
        <div className="mt-1.5 font-mono text-pill-group-badge text-faint">
          {[
            "dependency_quarantined",
            group.producerNodeId || null,
            generation !== undefined ? `gen ${generation}` : null,
          ]
            .filter(Boolean)
            .join(" · ")}
        </div>
      </div>
      {group.producerNodeId && onOpenQuarantine ? (
        <Button
          className="min-h-6 shrink-0"
          data-testid={`loop-run-attention-open-quarantine-${group.producerNodeId}`}
          onClick={() => onOpenQuarantine(group.producerNodeId)}
          size="sm"
          type="button"
          variant="outline"
        >
          Open quarantine entry
        </Button>
      ) : null}
    </div>
  );
}

/**
 * The Needs-attention panel (US-007 EC-2 / US-024 AC-4). An attention flag is
 * evidence, not a stop: the run keeps going. Consumers parked behind one
 * quarantined producer collapse into a single row that routes to the
 * producer's repair record; other flags keep the daemon's own reason.
 */
export function LoopRunAttentionPanel({
  nodes,
  renderNodeActions,
  onOpenQuarantine,
}: LoopRunAttentionPanelProps) {
  if (nodes.length === 0) return null;
  const { groups, singles } = groupDependencyAttention(nodes);
  let rowIndex = 0;
  return (
    <LoopRunSection
      data-testid="loop-run-attention"
      label="Needs attention"
      right={<span className="font-mono text-mono-id text-subtle">{nodes.length} flagged</span>}
    >
      <div className="overflow-hidden rounded-lg border border-warning/25 bg-warning-tint">
        {groups.map(group => (
          <DependencyAttentionRow
            group={group}
            key={`producer-${group.producerNodeId}`}
            onOpenQuarantine={onOpenQuarantine}
            withDivider={rowIndex++ > 0}
          />
        ))}
        {singles.map(node => (
          <div
            className={`flex items-start gap-3 px-4 py-3.5 ${rowIndex++ > 0 ? "border-t border-line-soft" : ""}`}
            data-testid={`loop-run-attention-${node.nodeId}`}
            key={node.nodeId}
          >
            <span aria-hidden="true" className="mt-1.5 size-2 shrink-0 rounded-full bg-warning" />
            <div className="min-w-0 flex-1">
              <div className="text-ws-name font-medium text-warning">
                {node.label} {ATTENTION_TITLE[node.attentionFlag] ?? "needs attention"}
              </div>
              <p className="mt-0.75 max-w-[62ch] text-small-body leading-relaxed text-muted">
                {ATTENTION_BODY[node.attentionFlag] ?? node.attentionReason}
              </p>
              <div className="mt-1.5 font-mono text-pill-group-badge text-faint">
                {[
                  node.attentionFlag,
                  node.lastEvidenceAt
                    ? `last evidence ${formatRelativeTime(node.lastEvidenceAt)}`
                    : null,
                  `${node.nodeId} · gen ${node.generation}`,
                ]
                  .filter(Boolean)
                  .join(" · ")}
              </div>
            </div>
            {renderNodeActions ? (
              <span className="flex shrink-0 items-center gap-2 pt-0.5">
                {renderNodeActions(node)}
              </span>
            ) : null}
          </div>
        ))}
      </div>
    </LoopRunSection>
  );
}
