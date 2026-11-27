import { useMemo, useState } from "react";

import {
  toBoardSessionPayload,
  type RemoteSessionRowEntry,
} from "@/systems/computers/mocks/remote-computers-fixtures-sessions";
import type { SessionsBoardColumnView } from "@/systems/session/hooks/use-sessions-board";
import type { SessionBoardColumnId } from "@/systems/session/lib/session-board-contract";

const COLUMN_DEFS: ReadonlyArray<{ id: SessionBoardColumnId; label: string }> = [
  { id: "needs_you", label: "Needs you" },
  { id: "working", label: "Working" },
  { id: "in_review", label: "In review" },
  { id: "done", label: "Done" },
];

export function buildRemoteBoardColumns(
  entries: readonly RemoteSessionRowEntry[]
): SessionsBoardColumnView[] {
  const payloads = entries.map(toBoardSessionPayload);
  return COLUMN_DEFS.map(def => {
    const sessions = payloads.filter(s => s.board_column === def.id);
    return {
      id: def.id,
      label: def.label,
      count: sessions.length,
      sessions,
      loading: false,
      failure: null,
      retrying: false,
      fetching: false,
      hasMore: false,
      loadingMore: false,
      loadMore: () => undefined,
      retry: () => undefined,
      total: sessions.length,
    };
  });
}

export interface BoardSwimlane {
  key: string;
  project: string;
  computerName: string;
  isLocal: boolean;
  offline: boolean;
  iconKind: RemoteSessionRowEntry["iconKind"];
  platform: RemoteSessionRowEntry["platform"];
  entries: RemoteSessionRowEntry[];
}

export function useRemoteKanbanView(
  sessions: readonly RemoteSessionRowEntry[],
  initialGrouped: boolean
) {
  const [grouped, setGrouped] = useState(initialGrouped);
  const [collapsedColumns, setCollapsedColumns] = useState<Record<string, boolean>>({});
  const [selectedId, setSelectedId] = useState<string | null>("sess-devbox-1");

  const entryMap = useMemo(() => new Map(sessions.map(entry => [entry.id, entry])), [sessions]);
  const flatColumns = useMemo(() => buildRemoteBoardColumns(sessions), [sessions]);

  const swimlanes = useMemo<BoardSwimlane[]>(() => {
    const map = new Map<string, BoardSwimlane>();
    for (const item of sessions) {
      const key = `${item.project}::${item.computerId}`;
      const existing = map.get(key);
      if (existing) {
        existing.entries.push(item);
      } else {
        map.set(key, {
          key,
          project: item.project,
          computerName: item.isLocal ? "This computer" : item.computerName,
          isLocal: item.isLocal,
          offline: Boolean(item.computerOffline),
          iconKind: item.iconKind,
          platform: item.platform,
          entries: [item],
        });
      }
    }
    return [...map.values()].sort((a, b) => {
      const p = a.project.localeCompare(b.project);
      if (p !== 0) return p;
      if (a.isLocal !== b.isLocal) return a.isLocal ? -1 : 1;
      return a.computerName.localeCompare(b.computerName);
    });
  }, [sessions]);

  return {
    grouped,
    setGrouped,
    collapsedColumns,
    setCollapsedColumns,
    selectedId,
    setSelectedId,
    entryMap,
    flatColumns,
    swimlanes,
  };
}
