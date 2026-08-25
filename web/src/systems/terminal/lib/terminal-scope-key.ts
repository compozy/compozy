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

function segment(value: string): string {
  return `${value.length}:${value}`;
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

/** Identity of one `(workspace, profile)` view. */
export function terminalScopeKey(workspaceId: string, profileKey: string): string {
  return `${segment(workspaceId)}${segment(profileKey)}`;
}

/**
 * Identity of one terminal's buffer.
 *
 * Scoped by profile as well as workspace, so switching profile can never hand a
 * pane the buffer another profile's terminal was using.
 */
export function terminalInstanceKey(
  workspaceId: string,
  profileKey: string,
  terminalId: string
): string {
  return `${PANE_NAMESPACE}${terminalScopeKey(workspaceId, profileKey)}${segment(terminalId)}`;
}

/**
 * Identity of one transcript preview's buffer.
 *
 * Deliberately outside the pane namespace: a preview is a second view of a
 * terminal, never a second claim on the app's buffer, and the app's scope sweep
 * must leave it alone. Scoped by the block that renders it as well as the
 * terminal, because the same terminal id can appear in two blocks — and, across
 * profiles, mean two different terminals.
 */
export function sessionPreviewInstanceKey(input: {
  blockId: string;
  terminalId: string;
  workspaceId?: string;
  profile?: string;
}): string {
  return [
    segment("session-preview"),
    segment(input.blockId),
    segment(input.workspaceId ?? ""),
    segment(input.profile ?? ""),
    segment(input.terminalId),
  ].join("");
}

/** True when a buffer belongs to the Terminal app rather than another surface. */
export function isTerminalPaneKey(key: string): boolean {
  return key.startsWith(PANE_NAMESPACE);
}

/** True when a buffer key belongs to the given scope. */
export function terminalInstanceKeyInScope(key: string, scopeKey: string): boolean {
  return key.startsWith(`${PANE_NAMESPACE}${scopeKey}`);
}
