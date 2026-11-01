import { protoHref } from "@compozy/proto";

import type { DraftInstance } from "../stays-on-agents-shared/fixtures";

const REPORT_STATE: Partial<Record<DraftInstance["state"], string>> = {
  "needs-you": "proposal",
  failed: "failure",
};

/** The avatar's one click: this agent's own conversation, at whatever it left for you. */
export function openMainSession(instance: DraftInstance) {
  const state = REPORT_STATE[instance.state] ?? "conversation";
  window.location.assign(protoHref("stays-on-reports", state));
}
