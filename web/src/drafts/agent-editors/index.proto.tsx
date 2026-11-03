import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { SCOUT, TRIAGE } from "../always-on-agents-shared/fixtures";
import { CheckInsEditor, type CheckInsVariant } from "./checkins-editor";
import { MemoryEditor, type MemoryVariant } from "./memory-editor";
import { WhatItDoesEditor, type WhatItDoesVariant } from "./what-it-does-editor";

export const meta = definePrototype({
  title: "Agent editors",
  spec: "always-on-agent",
  surface: "S6 S7 S9",
  route: "/agents/triage",
  states: [
    { id: "what", label: "Instructions · Filled" },
    { id: "what-empty", label: "Instructions · Empty" },
    { id: "what-unsaved", label: "Instructions · Unsaved changes" },
    { id: "what-conflict", label: "Instructions · Agent changed it" },
    { id: "what-advanced", label: "Instructions · Advanced" },
    { id: "what-advanced-only", label: "Instructions · File the editor can't show" },
    { id: "what-invalid", label: "Instructions · File has a mistake" },
    { id: "checkins", label: "Heartbeat · On" },
    { id: "checkins-empty", label: "Heartbeat · Never turned on" },
    { id: "checkins-too-often", label: "Heartbeat · Too often" },
    { id: "checkins-unsaved", label: "Heartbeat · Unsaved changes" },
    { id: "checkins-conflict", label: "Heartbeat · Agent changed it" },
    { id: "checkins-try-running", label: "Heartbeat · Trying" },
    { id: "checkins-try-result", label: "Heartbeat · Try result" },
    { id: "checkins-paused", label: "Heartbeat · Paused by you" },
    { id: "checkins-failed", label: "Heartbeat · Stopped after failures" },
    { id: "checkins-slowed", label: "Heartbeat · Slowed down" },
    { id: "checkins-nowhere", label: "Heartbeat · Nowhere to work" },
    { id: "checkins-advanced", label: "Heartbeat · Advanced" },
    { id: "memory", label: "Memory · Notes" },
    { id: "memory-empty", label: "Memory · Empty" },
    { id: "memory-adding", label: "Memory · Adding a note" },
    { id: "memory-unsaved", label: "Memory · Unsaved changes" },
    { id: "memory-conflict", label: "Memory · Agent changed it" },
    { id: "memory-almost-full", label: "Memory · Almost full" },
    { id: "memory-full", label: "Memory · Full" },
    { id: "memory-expired", label: "Memory · Expired note" },
    { id: "memory-advanced", label: "Memory · Advanced" },
  ],
});

export default function AgentEditorsDraft({ state }: PrototypeProps) {
  const [surface = "what", ...rest] = state.split("-");
  const variant = rest.join("-") || "filled";
  if (surface === "memory") {
    return <MemoryEditor agent={TRIAGE} key={state} variant={variant as MemoryVariant} />;
  }
  if (surface === "checkins") {
    return (
      <CheckInsEditor
        agent={variant === "nowhere" ? SCOUT : TRIAGE}
        key={state}
        variant={variant as CheckInsVariant}
      />
    );
  }
  return <WhatItDoesEditor agent={TRIAGE} key={state} variant={variant as WhatItDoesVariant} />;
}
