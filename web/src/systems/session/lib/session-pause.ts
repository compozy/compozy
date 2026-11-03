/** Client-written copy for the pause and context reports: counts and codes from the daemon. */

export function thingsCount(count: number) {
  return count === 1 ? "1 thing" : `${count} things`;
}

/** "2 things are waiting"; empty when nothing is held. */
export function heldPhrase(held: number) {
  if (held <= 0) return "";
  return `${thingsCount(held)} ${held === 1 ? "is" : "are"} waiting`;
}

const BLOCKER: Record<string, string> = {
  active_turn: "I'm still finishing a step",
  session_state_or_interaction: "a question is waiting for your answer",
  pending_interaction: "a question is waiting for your answer",
  queued_input: "messages are still waiting for me",
  prompt_admission: "a message is still on its way to me",
  active_goal: "a goal is still running here",
  reply_watch: "I'm waiting on a reply",
  child_wake: "work I handed off is still running",
  child_delivery: "work I handed off is still running",
  live_descendant: "work I handed off is still running",
};

/** What keeps a full conversation from moving on, in plain words, without repeats. */
export function blockerPhrases(blockers: readonly string[]) {
  const phrases = blockers.map(code =>
    code.startsWith("active_monitor:")
      ? "I'm still watching for something to happen"
      : (BLOCKER[code] ?? "other work is still in progress")
  );
  return [...new Set(phrases)];
}
