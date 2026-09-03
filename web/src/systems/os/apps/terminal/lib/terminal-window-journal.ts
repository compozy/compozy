/** Journal pages stay unfetched until the journal surface is first opened. */
export function terminalJournalQueryEnabled(workspaceId: string, unlocked: boolean): boolean {
  return workspaceId !== "" && unlocked;
}
