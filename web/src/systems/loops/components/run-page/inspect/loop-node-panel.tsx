import type { ReactNode } from "react";
import { ArrowUpRight, CircleSlash } from "lucide-react";

import { Panel, Pill, PropertyRow, Time } from "@compozy/ui";
import { Link } from "@tanstack/react-router";

import type { LoopNodeLink, LoopNodePanelModel } from "../../../lib/loop-node-panel-view";
import { LoopNodeStateChip } from "../loop-node-state-chip";

interface LoopNodePanelProps {
  panel: LoopNodePanelModel;
  /** Node verbs the daemon authorises for this state. Absent, never disabled. */
  actions?: ReactNode;
}

/**
 * One node, opened from the graph or the roster.
 *
 * Everything here is what the operator came for and could not get from a status
 * word: how many attempts it took, what class of failure they were, when the
 * next one lands, who cancelled it and why — and the two links that let them
 * leave for the session or the execution record without losing their place.
 *
 * Links stay valid after the run ends. Where retention has removed the target,
 * the row degrades to a sentence rather than a link that 404s on click.
 */
const LINK_CLASS =
  "inline-flex items-center gap-1 text-form-label text-subtle hover:text-fg-strong";

function LoopNodeLinkRow({ link }: { link: LoopNodeLink }) {
  const label = (
    <>
      {link.label}
      <ArrowUpRight aria-hidden="true" className="size-3" />
    </>
  );
  const testId = `loop-node-panel-link-${link.kind}`;
  // Each destination is its own typed route: the router types the params per
  // path, so one spread across three shapes is not something it can check.
  if (link.kind === "session") {
    return (
      <Link className={LINK_CLASS} data-testid={testId} params={{ id: link.id }} to="/session/$id">
        {label}
      </Link>
    );
  }
  if (link.kind === "child-run") {
    return (
      <Link
        className={LINK_CLASS}
        data-testid={testId}
        params={{ runId: link.id }}
        to="/loop-runs/$runId"
      >
        {label}
      </Link>
    );
  }
  return (
    <Link className={LINK_CLASS} data-testid={testId} params={{ id: link.id }} to="/tasks/$id">
      {label}
    </Link>
  );
}

export function LoopNodePanel({ panel, actions }: LoopNodePanelProps) {
  return (
    <Panel data-node-id={panel.nodeId} data-testid="loop-node-panel" title={panel.nodeId}>
      <div className="flex flex-col gap-3 px-4 py-3.5">
        <div className="flex flex-wrap items-center gap-2">
          <LoopNodeStateChip chip={panel.chip} />
          <span className="font-mono text-mono-id text-faint">
            round {panel.generation} · {panel.kindLabel}
          </span>
          {panel.attemptLabel ? (
            <span
              className="font-mono text-mono-id text-subtle"
              data-testid="loop-node-panel-attempt"
            >
              {panel.attemptLabel}
            </span>
          ) : null}
        </div>

        {panel.neverMaterialized ? (
          <p className="text-small-body text-muted" data-testid="loop-node-panel-never">
            This step never ran, so it has no session, record or timing to open.
          </p>
        ) : null}

        {panel.cancellation ? (
          <div className="flex flex-col gap-1" data-testid="loop-node-panel-cancellation">
            <span className="text-small-body text-fg-strong">{panel.cancellation.label}</span>
            {panel.cancellation.actorLabel ? (
              <span className="text-form-hint text-subtle">by {panel.cancellation.actorLabel}</span>
            ) : null}
            {panel.cancellation.cause ? (
              <span className="text-form-hint text-subtle">{panel.cancellation.cause}</span>
            ) : null}
          </div>
        ) : null}

        {panel.nextRetryAt ? (
          <PropertyRow label="Next retry" mono>
            <Time iso={panel.nextRetryAt} />
          </PropertyRow>
        ) : null}
        {panel.startedAt ? (
          <PropertyRow label="Started" mono>
            <Time iso={panel.startedAt} />
          </PropertyRow>
        ) : null}
        {panel.endedAt ? (
          <PropertyRow label="Ended" mono>
            <Time iso={panel.endedAt} />
          </PropertyRow>
        ) : null}

        {/* Every recorded attempt, including the only one: "it succeeded first
            try, at 18:43" is an answer, and hiding it makes a single-attempt node
            look like one with no history at all. */}
        {panel.attempts.length > 0 ? (
          <div data-testid="loop-node-panel-attempts">
            <ul className="flex flex-col divide-y divide-line-soft">
              {panel.attempts.map(attempt => (
                <li className="flex items-center gap-2 py-1.5" key={attempt.key}>
                  <span className="font-mono text-mono-id text-subtle">
                    Attempt {attempt.attempt}
                  </span>
                  <LoopNodeStateChip chip={attempt.chip} />
                  {attempt.failureLabel ? (
                    <span className="text-form-hint text-muted">{attempt.failureLabel}</span>
                  ) : null}
                  {attempt.endedAt ? (
                    <span className="ml-auto text-form-hint text-faint">
                      <Time iso={attempt.endedAt} />
                    </span>
                  ) : null}
                </li>
              ))}
            </ul>
          </div>
        ) : null}

        {panel.links.length > 0 || panel.degradedLinks.length > 0 || actions ? (
          <div className="flex flex-wrap items-center gap-3 border-t border-line-soft pt-3">
            {panel.links.map(link => (
              <LoopNodeLinkRow key={link.kind} link={link} />
            ))}
            {panel.degradedLinks.map(degraded => (
              <Pill
                data-testid={`loop-node-panel-degraded-${degraded.kind}`}
                key={degraded.kind}
                tone="neutral"
              >
                <CircleSlash aria-hidden="true" className="size-3" />
                {degraded.note}
              </Pill>
            ))}
            {actions ? <span className="ml-auto">{actions}</span> : null}
          </div>
        ) : null}
      </div>
    </Panel>
  );
}
