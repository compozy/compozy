import { getResponse, type HttpHandler } from "msw";

type ScopedEntity = {
  id: string;
  scope?: string;
  workspace_id?: string | null;
};

export interface MswScopedListFilter {
  scope?: string | null;
  workspace?: string | null;
  workspace_id?: string | null;
}

function cloneItems<T>(items: ReadonlyArray<T>): T[] {
  return items.map(item => ({ ...item }));
}

function matchesScopedFilter<T extends ScopedEntity>(item: T, filter: MswScopedListFilter) {
  const scope = filter.scope?.trim();
  if (!scope || scope === "all") {
    return true;
  }
  if (item.scope !== scope) {
    return false;
  }
  if (scope !== "workspace") {
    return true;
  }
  const workspaceId = filter.workspace_id?.trim() || filter.workspace?.trim();
  if (!workspaceId) {
    return true;
  }
  return item.workspace_id === workspaceId;
}

export function createStatefulMswStore<T extends { id: string }>(initialItems: ReadonlyArray<T>) {
  let items = cloneItems(initialItems);

  return {
    all() {
      return cloneItems(items);
    },
    delete(id: string) {
      const before = items.length;
      items = items.filter(item => item.id !== id);
      return before !== items.length;
    },
    get(id: string) {
      const item = items.find(candidate => candidate.id === id);
      return item ? { ...item } : undefined;
    },
    list(predicate?: (item: T) => boolean) {
      return cloneItems(predicate ? items.filter(predicate) : items);
    },
    listScoped(this: void, filter: MswScopedListFilter) {
      return cloneItems(
        (items as Array<T & ScopedEntity>).filter(item => matchesScopedFilter(item, filter))
      );
    },
    patch(id: string, patch: Partial<T>) {
      let updated: T | undefined;
      items = items.map(item => {
        if (item.id !== id) {
          return item;
        }
        updated = { ...item, ...patch };
        return updated;
      });
      return updated ? { ...updated } : undefined;
    },
    prepend(item: T) {
      items = [{ ...item }, ...items.filter(candidate => candidate.id !== item.id)];
      return { ...item };
    },
    reset(nextItems: ReadonlyArray<T>) {
      items = cloneItems(nextItems);
    },
    set(nextItems: ReadonlyArray<T>) {
      items = cloneItems(nextItems);
    },
  };
}

export function createMswFetch(
  getHandlers: () => ReadonlyArray<HttpHandler>,
  baseUrl = typeof window === "undefined" ? "http://localhost" : window.location.origin
): typeof globalThis.fetch {
  return (async (input: RequestInfo | URL, init?: RequestInit) => {
    const request = input instanceof Request ? input.clone() : new Request(input, init);
    const response = await getResponse([...getHandlers()], request, { baseUrl });

    if (!response) {
      const url = new URL(request.url, baseUrl);
      throw new Error(`No MSW handler matched: ${request.method} ${url.pathname}${url.search}`);
    }

    return response;
  }) as typeof globalThis.fetch;
}
