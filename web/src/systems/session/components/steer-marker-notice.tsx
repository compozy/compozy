import { Marker, MarkerMeta } from "@compozy/ui";

import { SessionSteerReceipt } from "@/components/assistant-ui/session-steer-receipt";

import type { SteerMarkerView } from "../lib/steer-marker";
import { useSteerProvenance } from "../hooks/use-steer-provenance";
import type { AgentEventPayload } from "../types";

/**
 * The legacy steer line: one quiet marker row when the daemon's marker names
 * no message identity (older sessions) or the delivered message is not loaded.
 * A superseded steer drops one ink step so nothing disappears silently.
 */
export function SteerMarkerNotice({ view }: { view: SteerMarkerView }) {
  const Icon = view.icon;
  return (
    <Marker
      role="status"
      data-testid="steer-marker-notice"
      data-steer={view.kind}
      tone="neutral"
      className={view.kind === "superseded" ? "text-faint" : undefined}
      icon={<Icon strokeWidth={1.8} />}
    >
      <span data-testid="steer-marker-text">{view.text}</span>
      {view.meta ? (
        <>
          {" "}
          <MarkerMeta data-testid="steer-marker-meta">{view.meta}</MarkerMeta>
        </>
      ) : null}
    </Marker>
  );
}

/**
 * A steer marker row resolved through the thread's provenance (VC-07): when the
 * identity's real message is loaded the bubble's meta line owns the fact and
 * the row renders nothing; guidance never dispatched as a message renders its
 * receipt bubble here, once, with the latest truth (a clustered row renders
 * every receipt it stands for); a marker without identity stays the legacy
 * line — never a bubble the daemon did not write.
 */
export function SteerMarkerRow({
  event,
  view,
  count,
}: {
  event: AgentEventPayload;
  view: SteerMarkerView;
  count: number;
}) {
  const provenance = useSteerProvenance();
  const render = provenance.renderFor(event);
  if (render === null || render.kind === "legacy") {
    return <SteerMarkerNotice view={view} />;
  }
  if (render.kind === "bound" || render.kind === "superseded-marker") {
    const receipts = provenance.receiptsForCluster(event, count);
    return receipts.length === 0
      ? null
      : receipts.map(receipt => (
          <SessionSteerReceipt key={receipt.messageId} provenance={receipt} />
        ));
  }
  return provenance
    .receiptsForCluster(event, count)
    .map(receipt => <SessionSteerReceipt key={receipt.messageId} provenance={receipt} />);
}
