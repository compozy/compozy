import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { ScriptApprovalBadges } from "./agent-badges";
import { ScriptApprovalBell, type BellVariant } from "./bell-variations";
import { ScriptSourcesCard, type SourcesCase, type SourcesVariant } from "./sources-card";

export const meta = definePrototype({
  title: "Script approvals (#1258)",
  surface: "Heartbeat · Bell · Agents",
  route: "/agents/merge-train/check-ins",
  states: [
    { id: "row-inline-first", label: "Heartbeat row A · Before the first check" },
    { id: "row-inline-paused", label: "Heartbeat row A · Paused agent" },
    { id: "row-inline-changed", label: "Heartbeat row A · Script changed" },
    { id: "row-inline-approved", label: "Heartbeat row A · Approved" },
    { id: "row-callout-first", label: "Heartbeat row B · Before the first check" },
    { id: "row-callout-paused", label: "Heartbeat row B · Paused agent" },
    { id: "row-callout-changed", label: "Heartbeat row B · Script changed" },
    { id: "bell-inline", label: "Bell A · Approve on the row" },
    { id: "bell-peek", label: "Bell B · Peek, then approve" },
    { id: "bell-section", label: "Bell C · Scripts to approve section" },
    { id: "badges", label: "Agents list badge and profile alert" },
  ],
});

export default function ScriptApprovalsDraft({ state }: PrototypeProps) {
  const [surface, variant, which] = state.split("-");
  if (surface === "bell")
    return <ScriptApprovalBell key={state} variant={variant as BellVariant} />;
  if (surface === "badges") return <ScriptApprovalBadges />;
  return (
    <ScriptSourcesCard
      key={state}
      variant={(variant ?? "inline") as SourcesVariant}
      which={(which ?? "first") as SourcesCase}
    />
  );
}
