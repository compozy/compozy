import { protoHref } from "@compozy/proto";

/** Reaches a profile state from any prototype that shows the agent page. */
export function goToProfile(state: string) {
  window.location.assign(protoHref("agent-profile", state));
}

export function goToAgents() {
  window.location.assign(protoHref("stays-on-agents-list", "cards"));
}
