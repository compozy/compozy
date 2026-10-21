/** Spoken and tooltip form of a workspace's working-session count. */
export function workspaceWorkingLabel(count: number): string {
  return `${count} session${count === 1 ? "" : "s"} working`;
}
