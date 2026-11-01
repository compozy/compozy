import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { SCOUT, TRIAGE } from "../persistent-agents-shared/fixtures";
import { CheckInsEditor, type CheckInsVariant } from "./checkins-editor";
import { MemoryEditor, type MemoryVariant } from "./memory-editor";
import { WhatItDoesEditor, type WhatItDoesVariant } from "./what-it-does-editor";

export const meta = definePrototype({
  title: "Agent editors",
  spec: "always-on-agent",
  surface: "S6 S7 S9",
  route: "/agents/triage",
  states: [
    { id: "what", label: "What it does · Filled" },
    { id: "what-empty", label: "What it does · Empty" },
    { id: "what-unsaved", label: "What it does · Unsaved changes" },
    { id: "what-conflict", label: "What it does · Agent changed it" },
    { id: "what-advanced", label: "What it does · Advanced" },
    { id: "what-advanced-only", label: "What it does · File the editor can't show" },
    { id: "what-invalid", label: "What it does · File has a mistake" },
    { id: "checkins", label: "Check-ins · On" },
    { id: "checkins-empty", label: "Check-ins · Never turned on" },
    { id: "checkins-too-often", label: "Check-ins · Too often" },
    { id: "checkins-unsaved", label: "Check-ins · Unsaved changes" },
    { id: "checkins-conflict", label: "Check-ins · Agent changed it" },
    { id: "checkins-try-running", label: "Check-ins · Trying" },
    { id: "checkins-try-result", label: "Check-ins · Try result" },
    { id: "checkins-paused", label: "Check-ins · Paused by you" },
    { id: "checkins-failed", label: "Check-ins · Stopped after failures" },
    { id: "checkins-slowed", label: "Check-ins · Slowed down" },
    { id: "checkins-nowhere", label: "Check-ins · Nowhere to work" },
    { id: "checkins-advanced", label: "Check-ins · Advanced" },
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
