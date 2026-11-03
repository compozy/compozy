import { sessionsBoardViewFixture } from "@/systems/session/mocks/sessions-board-view-fixtures";

export function emptyColumns() {
  return sessionsBoardViewFixture().columns.map(column => ({
    ...column,
    sessions: [],
    count: 0,
    hasMore: false,
  }));
}

export function liveColumns() {
  return sessionsBoardViewFixture().columns.map(column => ({
    ...column,
    hasMore: false,
    count: column.sessions.length,
  }));
}
