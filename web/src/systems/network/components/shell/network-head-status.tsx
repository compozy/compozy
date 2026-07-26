import { Pill } from "@agh/ui";

import type { OpenWorkEntry } from "../../hooks/use-work";
import { getNetworkStatusTone } from "../../lib/network-formatters";
import type { NetworkStatus } from "../../types";

export interface NetworkHeadStatusProps {
  status: NetworkStatus;
  workEntries: ReadonlyArray<OpenWorkEntry>;
  openWorkCount: number;
  drilledIntoConversation: boolean;
}

/**
 * Head status for the network window (prototype w2-status contract): the
 * listener state + live-peer count at root/channel level; the open-work state
 * takes over while drilled into a thread with work in flight.
 */
export function NetworkHeadStatus({
  status,
  workEntries,
  openWorkCount,
  drilledIntoConversation,
}: NetworkHeadStatusProps) {
  if (drilledIntoConversation && openWorkCount > 0) {
    const needsInput = workEntries.some(entry => entry.state === "needs_input");
    const working = workEntries.some(entry => entry.state === "working");
    const tone = needsInput ? "warning" : working ? "info" : "neutral";
    const stateLabel = needsInput ? "needs input · " : working ? "working · " : "";
    return (
      <Pill data-testid="network-head-status" tone={tone}>
        <Pill.Dot pulse={needsInput} tone={tone} />
        {stateLabel}
        {openWorkCount} work open
      </Pill>
    );
  }

  if (!status.enabled) return null;
  const live = status.local_peers ?? 0;
  const runtimeStatus = status.status.trim();
  const active = runtimeStatus === "active";
  const tone = getNetworkStatusTone(runtimeStatus);

  return (
    <Pill data-testid="network-head-status" tone={tone}>
      <Pill.Dot tone={tone} />
      {active ? "Active" : runtimeStatus === "ready" ? "Ready" : "Network"}
      {active && live > 0 ? ` · ${live} live` : ""}
    </Pill>
  );
}
