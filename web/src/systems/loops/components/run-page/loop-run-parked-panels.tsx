import type { ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import { ArrowRight, Hourglass, ShieldAlert, TriangleAlert } from "lucide-react";

import { Button, formatRelativeTime } from "@compozy/ui";

import {
  isOpenWait,
  type LoopNodeLifecycle,
  type LoopNodeWaitView,
} from "../../lib/loop-node-lifecycle";
import { isLoopRequestKind, LOOP_REQUEST_WAIT_SENTENCE } from "../../lib/loop-request-vocabulary";
import { humanizeLoopNodeId } from "../../lib/loop-run-story-rows";
import { LoopSection } from "../loop-section";

interface LoopRunWaitingPanelProps {
  nodes: readonly LoopNodeLifecycle[];
  renderNodeActions?: (node: LoopNodeLifecycle) => ReactNode;

  requestKinds?: ReadonlyMap<string, string>;
  runId?: string;
}

interface LoopRunAttentionPanelProps {
  nodes: readonly LoopNodeLifecycle[];
  renderNodeActions?: (node: LoopNodeLifecycle) => ReactNode;
  /** Opens the quarantine entry for the quarantined producer node id. */
  onOpenQuarantine?: (nodeId: string) => void;
  runId?: string;
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
  request: "Parked on a person. Answer it in Needs you and the lane resumes.",
};

const WAIT_KIND_TITLE: Record<string, string> = {
  timer: "is waiting on a timer",
  event: "is waiting for an event",
  approval_escalation: "is waiting for your decision",

  request: "is waiting for an answer",
};

function hasLadderStrip(wait: LoopNodeWaitView): boolean {
  return (
    wait.kind === "approval_escalation" &&
    (wait.escalationCursor > 0 || Boolean(wait.nextEscalationAt))
  );
}

function waitTitle(wait: LoopNodeWaitView, requestKinds: ReadonlyMap<string, string>): string {
  if (wait.kind === "request") {
    const kind = requestKinds.get(`${wait.nodeId}:${wait.itemIndex}`);
    if (kind && isLoopRequestKind(kind)) return LOOP_REQUEST_WAIT_SENTENCE[kind];
  }
  return WAIT_KIND_TITLE[wait.kind] ?? "is parked";
}

function waitSentence(wait: LoopNodeWaitView): string {
  if (wait.kind === "approval_escalation") {
    return hasLadderStrip(wait)
      ? "The ladder walks its steps until you decide; a decision at any point wins and cancels the rest."
      : "Waiting for your decision. Deciding at any point resolves it.";
  }
  return WAIT_KIND_SENTENCE[wait.kind] ?? "This lane is parked until its wait resolves.";
}

/** Compact match condition — never an invented event name. */
function expectSummary(value: unknown): string | null {
  if (value == null || value === "") return null;
  if (typeof value === "string") return value;
  if (typeof value === "object") {
    try {
      const text = JSON.stringify(value);
      return text === "{}" || text === "[]" ? null : text;
    } catch {
      return null;
    }
  }
  return String(value);
}

const INVENTORY_LINK_CLASS =
  "inline-flex items-center gap-1.25 text-badge font-medium text-muted hover:text-fg-strong";

/**
 * The Waiting panel (US-021/US-022): one row per open wait cell with the
 * daemon's own age. Every value — kind, resume time, escalation cursor, age —
 * comes from the wait row; nothing is computed from wall-clock guesses here.
 */
export function LoopRunWaitingPanel({
  nodes,
  renderNodeActions,
  requestKinds = new Map(),
  runId,
}: LoopRunWaitingPanelProps) {
  const waits: { node: LoopNodeLifecycle; wait: LoopNodeWaitView }[] = [];
  for (const node of nodes) {
    for (const wait of node.waits) {
      if (isOpenWait(wait)) waits.push({ node, wait });
    }
  }
  if (waits.length === 0) return null;
  return (
    <LoopSection
      className="mb-0"
      data-testid="loop-run-waiting"
      gist={`${waits.length} ${waits.length === 1 ? "wait" : "waits"}`}
      icon={<Hourglass aria-hidden="true" />}
      title="Waiting"
    >
      <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
        {waits.map(({ node, wait }, index) => {
          const showLadder = hasLadderStrip(wait);
          return (
            <div
              className={`flex items-start gap-3 px-4 py-3.5 ${index > 0 ? "border-t border-line-soft" : ""}`}
              data-testid={`loop-run-wait-${node.nodeId}-${wait.itemIndex}`}
              key={`${node.nodeId}-${wait.itemIndex}`}
            >
              <span aria-hidden="true" className="mt-1.5 size-2 shrink-0 rounded-full bg-info" />
              <div className="min-w-0 flex-1">
                <div className="text-ws-name font-medium text-fg-strong">
                  {node.label} {waitTitle(wait, requestKinds)}
                </div>
                <div className="mt-0.75 max-w-[60ch] text-small-body leading-relaxed text-muted">
                  {waitSentence(wait)}
                </div>
                {showLadder ? (
                  <div
                    className="mt-1.5 font-mono text-pill-group-badge text-faint"
                    data-testid={`loop-run-wait-ladder-${node.nodeId}-${wait.itemIndex}`}
                  >
                    {[
                      `step ${wait.escalationCursor} done`,
                      wait.nextEscalationAt
                        ? `next escalation ${formatRelativeTime(wait.nextEscalationAt)}`
                        : null,
                    ]
                      .filter(Boolean)
                      .join(" · ")}
                  </div>
                ) : null}
                <div className="mt-1.5 font-mono text-pill-group-badge text-faint">
                  {[
                    wait.kind,
                    wait.resumeAt ? `resumes ${formatRelativeTime(wait.resumeAt)}` : null,
                    expectSummary(wait.expect),
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
          );
        })}
        <div className="border-t border-line-soft px-4 py-2.5">
          <Link
            className={INVENTORY_LINK_CLASS}
            data-testid="loop-run-waiting-inventory-link"
            search={{ nodes: "waiting", nodes_run: runId }}
            to="/loop-runs"
          >
            All waits in the inventory
            <ArrowRight aria-hidden="true" className="size-3" />
          </Link>
        </div>
      </div>
    </LoopSection>
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

const ATTENTION_CARD_CLASS = "overflow-hidden rounded-lg border border-warning/25 bg-warning-tint";

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
  onOpenQuarantine,
}: {
  group: DependencyAttentionGroup;
  onOpenQuarantine?: (nodeId: string) => void;
}) {
  const producerLabel = group.producerNodeId
    ? humanizeLoopNodeId(group.producerNodeId)
    : "a parked step";
  const consumerLabels = joinLabels(group.consumers.map(node => node.label));
  const generation = group.consumers[0]?.generation;
  return (
    <div
      className="flex items-start gap-3 px-4 py-3.5"
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
          <ShieldAlert aria-hidden="true" className="size-3.5" />
          Open quarantine entry
        </Button>
      ) : null}
    </div>
  );
}

function AttentionFlagRow({
  node,
  renderNodeActions,
}: {
  node: LoopNodeLifecycle;
  renderNodeActions?: (node: LoopNodeLifecycle) => ReactNode;
}) {
  return (
    <div
      className="flex items-start gap-3 px-4 py-3.5"
      data-testid={`loop-run-attention-${node.nodeId}`}
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
            node.lastEvidenceAt ? `last evidence ${formatRelativeTime(node.lastEvidenceAt)}` : null,
            `${node.nodeId} · gen ${node.generation}`,
          ]
            .filter(Boolean)
            .join(" · ")}
        </div>
      </div>
      {renderNodeActions ? (
        <span className="flex shrink-0 items-center gap-2 pt-0.5">{renderNodeActions(node)}</span>
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
  runId,
}: LoopRunAttentionPanelProps) {
  if (nodes.length === 0) return null;
  const { groups, singles } = groupDependencyAttention(nodes);
  const cardCount = groups.length + singles.length;
  const paired = cardCount >= 2;
  return (
    <LoopSection
      className="mb-0"
      data-testid="loop-run-attention"
      gist={`${nodes.length} flagged`}
      icon={<TriangleAlert aria-hidden="true" />}
      title="Needs attention"
    >
      <div className={paired ? "grid grid-cols-1 gap-3 min-[900px]:grid-cols-2" : undefined}>
        {groups.map(group => (
          <div className={ATTENTION_CARD_CLASS} key={`producer-${group.producerNodeId}`}>
            <DependencyAttentionRow group={group} onOpenQuarantine={onOpenQuarantine} />
          </div>
        ))}
        {singles.map(node => (
          <div className={ATTENTION_CARD_CLASS} key={node.nodeId}>
            <AttentionFlagRow node={node} renderNodeActions={renderNodeActions} />
          </div>
        ))}
      </div>
      <div className="mt-2.5">
        <Link
          className={INVENTORY_LINK_CLASS}
          data-testid="loop-run-attention-inventory-link"
          search={{ nodes: "attention", nodes_run: runId }}
          to="/loop-runs"
        >
          All flags in the inventory
          <ArrowRight aria-hidden="true" className="size-3" />
        </Link>
      </div>
    </LoopSection>
  );
}
