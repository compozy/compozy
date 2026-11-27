import { useMemo, useState } from "react";

import type {
  ComputerIconKind,
  ComputerPlatform,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import type { RemoteSessionRowEntry } from "@/systems/computers/mocks/remote-computers-fixtures-sessions";

export interface ProjectComputerGroup {
  key: string;
  project: string;
  computerName: string;
  isLocal: boolean;
  offline: boolean;
  iconKind: ComputerIconKind;
  platform: ComputerPlatform;
  rows: RemoteSessionRowEntry[];
}

export function useRemoteSessionsSidebar(
  sessions: readonly RemoteSessionRowEntry[],
  initialGroupBy: "none" | "project"
) {
  const [groupBy, setGroupBy] = useState<"none" | "project">(initialGroupBy);
  const [query, setQuery] = useState("");
  const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>({});
  const [retryingIds, setRetryingIds] = useState<string[]>([]);
  const [loadedMoreIds, setLoadedMoreIds] = useState<string[]>([]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return sessions;
    return sessions.filter(item =>
      `${item.title} ${item.project} ${item.computerName} ${item.model}`.toLowerCase().includes(q)
    );
  }, [sessions, query]);

  const groups = useMemo<ProjectComputerGroup[]>(() => {
    const map = new Map<string, ProjectComputerGroup>();
    for (const item of filtered) {
      const key = `${item.project}::${item.computerId}`;
      const existing = map.get(key);
      if (existing) {
        existing.rows.push(item);
      } else {
        map.set(key, {
          key,
          project: item.project,
          computerName: item.isLocal ? "This computer" : item.computerName,
          isLocal: item.isLocal,
          offline: Boolean(item.computerOffline),
          iconKind: item.iconKind,
          platform: item.platform,
          rows: [item],
        });
      }
    }
    return [...map.values()].sort((a, b) => {
      const proj = a.project.localeCompare(b.project);
      if (proj !== 0) return proj;
      if (a.isLocal !== b.isLocal) return a.isLocal ? -1 : 1;
      return a.computerName.localeCompare(b.computerName);
    });
  }, [filtered]);

  const totals = useMemo(() => {
    let working = 0;
    let needsYou = 0;
    for (const row of filtered) {
      if (row.computerOffline) continue;
      if (row.state === "running") working += 1;
      if (row.state === "attention") needsYou += 1;
    }
    return {
      working,
      needsYou,
      idle: 0,
    };
  }, [filtered]);

  const loadedMoreSet = useMemo(() => new Set(loadedMoreIds), [loadedMoreIds]);
  const retryingSet = useMemo(() => new Set(retryingIds), [retryingIds]);

  return {
    groupBy,
    setGroupBy,
    query,
    setQuery,
    collapsedGroups,
    setCollapsedGroups,
    setRetryingIds,
    setLoadedMoreIds,
    loadedMoreSet,
    retryingSet,
    filtered,
    groups,
    totals,
  };
}
