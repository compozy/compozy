import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { AgentAppPage, SaveActions } from "../persistent-agents-shared/agent-page";
import { SCOUT, TRIAGE } from "../persistent-agents-shared/fixtures";
import { HeartbeatEditor, type HeartbeatState } from "./heartbeat-editor";
import { MemoryEditor, type MemoryState } from "./memory-editor";
import { PersonaEditor, type PersonaState } from "./persona-editor";

export const meta = definePrototype({
  title: "Agent editors",
  spec: "always-on-agent",
  surface: "S6 S7 S9",
  route: "/agents/triage",
  states: [
    { id: "persona", label: "S6 · Form" },
    { id: "persona-advanced", label: "S6 · Advanced" },
    { id: "persona-conflict", label: "S6 · Use theirs or Keep mine" },
    { id: "persona-inexpressible", label: "S6 · Content the form can't express" },
    { id: "persona-invalid", label: "S6 · Invalid raw content" },
    { id: "heartbeat", label: "S7 · Heartbeat" },
    { id: "heartbeat-paused", label: "S7 · Paused" },
    { id: "heartbeat-fatigue", label: "S7 · Fatigue" },
    { id: "heartbeat-backoff", label: "S7 · Backoff active" },
    { id: "heartbeat-no-instances", label: "S7 · No instances" },
    { id: "heartbeat-floor", label: "S7 · Floor reached" },
    { id: "heartbeat-test-dry", label: "S7 · Test, dry run" },
    { id: "heartbeat-test-running", label: "S7 · Test, running" },
    { id: "heartbeat-advanced", label: "S7 · Advanced" },
    { id: "memory", label: "S9 · Notes" },
    { id: "memory-empty", label: "S9 · Empty" },
    { id: "memory-near-budget", label: "S9 · Near budget" },
    { id: "memory-full", label: "S9 · Full of undated notes" },
    { id: "memory-archive", label: "S9 · Archive present" },
    { id: "memory-conflict", label: "S9 · Conflict" },
    { id: "memory-expired", label: "S9 · Expired notes" },
    { id: "memory-advanced", label: "S9 · Advanced" },
  ],
});

export default function AgentEditorsDraft({ state }: PrototypeProps) {
  if (state.startsWith("persona")) {
    const variant = (state.split("-")[1] ?? "form") as PersonaState;
    return (
      <AgentAppPage
        actions={<SaveActions unsaved={variant === "form" ? 0 : 2} />}
        agent={TRIAGE}
        leaf="Persona"
      >
        <PersonaEditor state={variant} />
      </AgentAppPage>
    );
  }
  if (state.startsWith("memory")) {
    const variant = (state.split("-").slice(1).join("-") || "notes") as MemoryState;
    return (
      <AgentAppPage
        actions={<SaveActions unsaved={variant === "conflict" ? 1 : 0} />}
        agent={TRIAGE}
        leaf="Memory"
        width="max-w-5xl"
      >
        <MemoryEditor state={variant} />
      </AgentAppPage>
    );
  }
  const variant = (state.split("-").slice(1).join("-") || "default") as HeartbeatState;
  return (
    <AgentAppPage
      actions={<SaveActions unsaved={variant === "floor" ? 2 : 0} />}
      agent={variant === "no-instances" ? SCOUT : TRIAGE}
      leaf="Heartbeat"
      width="max-w-5xl"
    >
      <HeartbeatEditor state={variant} />
    </AgentAppPage>
  );
}
