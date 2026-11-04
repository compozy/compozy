import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { ScriptApprovalBadges } from "./agent-badges";
import { ScriptApprovalBell, type BellVariant } from "./bell-variations";
import { AgentsInShell, ProfileInShell } from "./shell-agents";
import { HeartbeatInShell } from "./shell-heartbeat";
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
    { id: "shell-heartbeat-inline-first", label: "In the app · Heartbeat row A · First check" },
    { id: "shell-heartbeat-inline-paused", label: "In the app · Heartbeat row A · Paused" },
    { id: "shell-heartbeat-callout-first", label: "In the app · Heartbeat row B · First check" },
    { id: "shell-heartbeat-callout-paused", label: "In the app · Heartbeat row B · Paused" },
    { id: "shell-bell-inline", label: "In the app · Bell A over Agents" },
    { id: "shell-bell-peek", label: "In the app · Bell B over Agents" },
    { id: "shell-bell-section", label: "In the app · Bell C over Agents" },
    { id: "shell-agents-badge", label: "In the app · Agents with the waiting badge" },
    { id: "shell-profile-alert", label: "In the app · Merge Train profile alert" },
  ],
});

function InShell({ state }: { state: string }) {
  const [, surface, variant, which] = state.split("-");
  if (surface === "heartbeat") {
    return (
      <HeartbeatInShell
        key={state}
        variant={variant as SourcesVariant}
        which={which as SourcesCase}
      />
    );
  }
  if (surface === "bell") return <AgentsInShell badges bell={variant as BellVariant} key={state} />;
  if (surface === "agents") return <AgentsInShell badges key={state} />;
  return <ProfileInShell />;
}

export default function ScriptApprovalsDraft({ state }: PrototypeProps) {
  if (state.startsWith("shell-")) return <InShell state={state} />;
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
