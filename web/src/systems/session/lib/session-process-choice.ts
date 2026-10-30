import { processCountLabel } from "./session-processes";

/** The lifecycle verbs that ask what happens to a session's running processes. */
export type SessionProcessChoiceAction = "stop" | "archive" | "delete";

export interface SessionProcessChoiceCopy {
  title: string;
  description: string;
  confirmLabel: string;
  keepLabel: string;
  note: string;
}

const KEEP_LABEL = "Keep processes running";
const KEEP_NOTE =
  "Kept processes stay listed on the session's composer and under Processes left running.";

function titleFor(action: SessionProcessChoiceAction, sessions: number): string {
  const verb = action === "stop" ? "Stop" : action === "archive" ? "Archive" : "Delete";
  return sessions === 1 ? `${verb} this session?` : `${verb} ${sessions} sessions?`;
}

function confirmFor(action: SessionProcessChoiceAction, sessions: number, processes: string) {
  if (action === "stop") {
    return sessions === 1 ? `Stop session and ${processes}` : `Stop sessions and ${processes}`;
  }
  return `${action === "archive" ? "Archive" : "Delete"} and stop ${processes}`;
}

/**
 * COPY.md: the three-way choice a stop, archive, or delete offers while the
 * session still has processes running. Keeping them is the daemon default, so
 * it is never the destructive button.
 */
export function sessionProcessChoiceCopy(
  action: SessionProcessChoiceAction,
  sessions: number,
  processes: number
): SessionProcessChoiceCopy {
  const count = processCountLabel(processes);
  const running = processes === 1 ? "is still running" : "are still running";
  return {
    title: titleFor(action, sessions),
    description:
      sessions === 1
        ? `This session started ${count} that ${running}.`
        : `These sessions started ${count} that ${running}.`,
    confirmLabel: confirmFor(action, sessions, count),
    keepLabel: KEEP_LABEL,
    note:
      action === "delete" ? "Kept processes stay listed under Processes left running." : KEEP_NOTE,
  };
}

/** "Listening on localhost:5173 and localhost:3000"; empty when nothing listens. */
export function processPortsSentence(ports: readonly number[]): string {
  const unique = [...new Set(ports)];
  if (unique.length === 0) return "";
  const names = unique.map(port => `localhost:${port}`);
  const last = names.pop();
  return names.length === 0
    ? `Listening on ${last}.`
    : `Listening on ${names.join(", ")} and ${last}.`;
}
