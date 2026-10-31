import { sessionSidebarStore } from "./hooks/use-session-sidebar-state";

export { primarySessionFixture } from "./mocks/fixtures";

/** Expands every board column folded in any window, so a test starts from the default board. */
export function resetSessionBoardColumnFolds(): void {
  const { byScope } = sessionSidebarStore.getSnapshot().context;
  for (const [scopeId, state] of Object.entries(byScope)) {
    for (const columnId of Object.keys(state.collapsedBoardColumns)) {
      sessionSidebarStore.trigger.boardColumnCollapsedChanged({
        scopeId,
        columnId,
        collapsed: false,
      });
    }
  }
}
