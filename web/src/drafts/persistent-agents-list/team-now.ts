/** What a teammate is up to, and the way to their profile. */
import { protoHref } from "@compozy/proto";

import { PAUSE_SHORT, profileStateOf } from "../persistent-agents-shared/agents-fixtures";
import type { ListedAgent } from "../persistent-agents-shared/agents-page";
import { SCOPE_LABEL } from "../persistent-agents-shared/fixtures";

export function openProfile(agent: ListedAgent) {
  window.location.assign(protoHref("agent-profile", profileStateOf(agent)));
}

/** One plain sentence: what this agent is up to right now. */
export function nowLine(agent: ListedAgent) {
  const places = agent.places ?? [];
  const live = places.find(place => place.state === "needs-you" || place.state === "working");
  if (live) {
    return { text: `${live.now ?? "Working"} in ${SCOPE_LABEL(live.scope)}`, tone: live.state };
  }
  const paused = places.find(place => place.state === "paused");
  if (paused) {
    return {
      text: paused.pauseReason ? PAUSE_SHORT[paused.pauseReason] : "Paused",
      tone: "paused" as const,
    };
  }
  if (places.length === 0) {
    return { text: "Waiting to be added to a project", tone: "idle" as const };
  }
  const last = agent.lastActive?.toLowerCase() ?? "a while ago";
  return { text: `Resting. Last check-in ${last}`, tone: "idle" as const };
}
