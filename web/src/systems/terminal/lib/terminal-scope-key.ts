/**
 * Binding keys for terminal scope and terminal buffers.
 *
 * Segments are length-prefixed rather than joined by a delimiter. A workspace
 * id, a profile name and a terminal id are all free-form enough that any
 * printable delimiter could appear inside one of them, and two different scopes
 * would then produce one key — which is exactly how a pane ends up showing
 * another profile's terminal. Length prefixes make that impossible, and unlike a
 * control-character delimiter they stay readable in a devtools cache view.
 */

import { isRemoteComputerId, normalizeComputerId, type ComputerId } from "@/lib/computer-scope";

function segment(value: string): string {
  return `${value.length}:${value}`;
}

function computerPrefix(computerId?: ComputerId): string {
  return isRemoteComputerId(computerId) ? segment(normalizeComputerId(computerId)) : "";
}

/**
 * Marks a buffer as the Terminal app's own.
 *
 * The registry is shared with every other emulator on the page — the session
 * transcript's preview blocks, most of all. Sweeping instances by scope has to
 * be able to say "not mine, leave it alone", or closing a profile would blank a
 * preview sitting in someone's conversation.
 */
const PANE_NAMESPACE = segment("terminal-pane");

/** Identity of one `(computer, workspace, profile)` view. */
export function terminalScopeKey(
  workspaceId: string,
  profileKey: string,
  computerId?: ComputerId
): string {
  return `${computerPrefix(computerId)}${segment(workspaceId)}${segment(profileKey)}`;
}

/**
 * Identity of one terminal's buffer.
 *
 * Scoped by computer and profile as well as workspace, so switching computer or
 * profile can never hand a pane the buffer another terminal was using.
 */
export function terminalInstanceKey(
  workspaceId: string,
  profileKey: string,
  terminalId: string,
  computerId?: ComputerId
): string {
  return `${PANE_NAMESPACE}${terminalScopeKey(workspaceId, profileKey, computerId)}${segment(terminalId)}`;
}

/**
 * Identity of one transcript preview's buffer.
 *
 * Deliberately outside the pane namespace: a preview is a second view of a
 * terminal, never a second owner of the app's buffer, and the app's scope sweep
 * must leave it alone. Scoped by the block that renders it as well as the
 * terminal, because the same terminal id can appear in two blocks — and, across
 * profiles or computers, mean two different terminals.
 */
export function sessionPreviewInstanceKey(input: {
  blockId: string;
  terminalId: string;
  workspaceId?: string;
  profile?: string;
  computerId?: ComputerId;
}): string {
  return [
    segment("session-preview"),
    computerPrefix(input.computerId),
    segment(input.blockId),
    segment(input.workspaceId ?? ""),
    segment(input.profile ?? ""),
    segment(input.terminalId),
  ].join("");
}

/** True when a buffer key belongs to the given scope. */
export function terminalInstanceKeyInScope(key: string, scopeKey: string): boolean {
  return key.startsWith(`${PANE_NAMESPACE}${scopeKey}`);
}
