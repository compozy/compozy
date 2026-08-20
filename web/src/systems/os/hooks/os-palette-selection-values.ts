/** The row identities the action panel already resolves — selection follows these. */
export interface PaletteSelectionRoot {
  readonly rowSources: {
    readonly commands: readonly { readonly id: string }[];
    readonly sessions: readonly { readonly sessionId: string }[];
    readonly tabs: readonly { readonly windowId: string }[];
    readonly worktrees: readonly { readonly key: string }[];
    readonly domainRows: readonly { readonly key: string }[];
  };
  readonly fallback: { readonly value: string } | null;
}

/** Selection keys follow the same row sources the action panel resolves. */
export function paletteSelectionValues(root: PaletteSelectionRoot): readonly string[] {
  const sources = root.rowSources;
  return [
    ...sources.commands.map(command => command.id),
    ...(root.fallback === null ? [] : [root.fallback.value]),
    ...sources.sessions.map(session => `session:${session.sessionId}`),
    ...sources.tabs.map(tab => `tab:${tab.windowId}`),
    ...sources.worktrees.map(entry => `worktree:${entry.key}`),
    ...sources.domainRows.map(row => row.key),
  ];
}
