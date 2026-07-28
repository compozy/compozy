export interface SlashCommandEntry {
  /** The literal command keyword without the leading `/`. */
  command: string;
  description: string;
  disabled?: boolean;
  disabledReason?: string;
}

const SLASH_COMMANDS: ReadonlyArray<SlashCommandEntry> = [
  {
    command: "run",
    description: "Run a capability for this conversation.",
  },
  {
    command: "mention",
    description: "Mention a peer to draw their attention.",
  },
  {
    command: "attach",
    description: "Attach context (file, URL, capability ref).",
    disabled: true,
    disabledReason: "Post-MVP",
  },
];

export function filterSlashCommandEntries(filterValue: string): ReadonlyArray<SlashCommandEntry> {
  const trimmed = filterValue.trim().toLowerCase();
  if (trimmed === "") {
    return SLASH_COMMANDS;
  }
  return SLASH_COMMANDS.filter(entry => entry.command.toLowerCase().startsWith(trimmed));
}
