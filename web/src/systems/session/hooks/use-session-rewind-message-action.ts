import { useAuiState } from "@assistant-ui/react";
import { use } from "react";

import { SessionMessageEditContext } from "@/components/assistant-ui/session-message-edit-context";
import { useSessionMessageActionGate } from "./use-session-message-action-gate";

export function useSessionRewindMessageAction() {
  const gate = useSessionMessageActionGate();
  const edit = use(SessionMessageEditContext);
  const text = useAuiState(state =>
    state.message.content.flatMap(part => (part.type === "text" ? [part.text] : [])).join("\n")
  );

  // Selecting text is local. Only submission asks the daemon to validate and rewind.
  return {
    available: gate.durable && edit !== null,
    busy: edit?.pending ?? false,
    trigger: () => edit?.start(gate.messageId, text),
  };
}
