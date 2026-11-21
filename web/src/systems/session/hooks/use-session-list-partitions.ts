import { InfiniteQueryObserver, useQueryClient } from "@tanstack/react-query";
import { useMemo, useSyncExternalStore } from "react";

import { normalizeSessionListFilters } from "../lib/session-list-query";
import {
  partitionSessionRows,
  SESSION_LIST_ROOT_FOLDER,
  sessionListPartitionOptions,
} from "../lib/session-list-partitions";
import type { SessionFolderPayload, SessionListFilters } from "../types";
import type { SessionFoldSet } from "../lib/session-sidebar-scope";
import { sessionCatalogPageView } from "./use-session-catalog";

export type SessionListPartitionPage = ReturnType<typeof sessionCatalogPageView>;
export type SessionListPartitions = ReadonlyMap<string, SessionListPartitionPage>;

export function useSessionListPartitions(
  filters: SessionListFilters,
  folders: readonly SessionFolderPayload[] | undefined,
  folds: SessionFoldSet,
  enabled: boolean,
  filtering: boolean
) {
  const client = useQueryClient();
  const filterKey = JSON.stringify(normalizeSessionListFilters(filters));
  const containerKey = JSON.stringify(
    folders?.length
      ? [
          [SESSION_LIST_ROOT_FOLDER, false],
          ...folders.map(folder => [folder.id, !filtering && folds.has(folder.id)]),
        ]
      : []
  );
  // An observer per container keeps cursor state in the same infinite-query
  // envelope as the flat catalog, even when the number of folders changes.
  const source = useMemo(() => {
    const scoped: SessionListFilters = JSON.parse(filterKey);
    const containers: [string, boolean][] = JSON.parse(containerKey);
    const entries = containers.map(([folder, countOnly]) => ({
      folder,
      countOnly,
      observer: new InfiniteQueryObserver(client, {
        ...sessionListPartitionOptions(scoped, folder, countOnly),
        enabled,
      }),
    }));
    const read = () => entries.map(({ observer }) => observer.getCurrentResult());
    let snapshot = read();
    return {
      entries,
      subscribe: (notify: () => void) => {
        const subscriptions = entries.map(({ observer }) => observer.subscribe(notify));
        return () => subscriptions.forEach(unsubscribe => unsubscribe());
      },
      getSnapshot: () => {
        const next = read();
        if (next.some((entry, index) => entry !== snapshot[index])) snapshot = next;
        return snapshot;
      },
    };
  }, [client, containerKey, enabled, filterKey]);
  const results = useSyncExternalStore(source.subscribe, source.getSnapshot, source.getSnapshot);
  if (!folders?.length) return undefined;

  const pages = new Map(
    source.entries.map(({ folder, countOnly }, index) => {
      const result = results[index]!;
      const page = sessionCatalogPageView(result);
      return [
        folder,
        {
          ...page,
          loading: enabled && result.isPending && !result.isError,
          sessions: countOnly ? [] : page.sessions,
        },
      ] as const;
    })
  );
  const root = pages.get(SESSION_LIST_ROOT_FOLDER)!;
  const { sessions, folderOf } = partitionSessionRows(pages);
  return {
    pages,
    folderOf,
    catalog: {
      ...root,
      sessions,
      loading: [...pages.values()].some(page => page.loading),
      facets: undefined,
    },
  };
}
