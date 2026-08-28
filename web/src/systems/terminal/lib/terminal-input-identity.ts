import type { TerminalInputActorProjection, TerminalInputRequest } from "../types";

/** The words the pin puts in its title — who is waiting, and for what. */
export function terminalInputRequestTitle(request: TerminalInputRequest): string {
  return `${request.requester.id} needs ${request.redacted ? "a password" : "an answer"}`;
}

/** Bell title: what kind of question this is, never a paraphrase of the reason. */
export function terminalInputAttentionTitle(redacted: boolean): string {
  return redacted ? "Password requested" : "Answer requested";
}

/** True when stacked cards would otherwise look interchangeable. */
export function terminalInputStackNeedsOrigin(
  requests: readonly Pick<TerminalInputRequest, "terminal_id">[]
): boolean {
  const terminals = new Set(requests.map(request => request.terminal_id));
  return terminals.size > 1;
}

export function isHumanInputActor(actor: TerminalInputActorProjection | undefined): boolean {
  return actor?.kind === "human";
}
