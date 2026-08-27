/**
 * The ask a child should show the operator.
 *
 * Admission prompts can start with a depth duty line. That line is for the
 * model, not the transcript plate.
 */

const DUTY_PREFIX = /^Call context:/;

export function operatorAskGist(text: string): string {
  return text
    .split("\n")
    .filter(line => !DUTY_PREFIX.test(line.trim()))
    .join("\n")
    .trim();
}

/** Speaker for a child bookend. Missing names become "the caller", never the child. */
export function callerDisplayName(turn: { callerAgentName: string | null }): string {
  return turn.callerAgentName ?? "the caller";
}
