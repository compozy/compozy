import type { SessionListViewModel } from "../hooks/use-session-list-view";

const noop = () => undefined;

/**
 * A settled, flat, single-profile session list view model for stories and
 * component tests: one project focused, created order, no groups.
 */
export function sessionListViewFixture(
  overrides: Partial<SessionListViewModel> = {}
): SessionListViewModel {
  return {
    sort: "created",
    storedGroupBy: "none",
    groupBy: "none",
    groupByOptions: ["none"],
    sortNote: null,
    newFolderUnavailable: null,
    shelf: "active",
    setSort: noop,
    setGroupBy: noop,
    setShelf: noop,
    groups: [],
    groupsLoading: false,
    collapsedGroupKeys: new Set(),
    toggleGroup: noop,
    groupFilters: {},
    aggregate: false,
    scopeLabel: "default",
    ownerOf: row => ({ id: row.profile_id, name: row.profile_name, archived: false }),
    selectionKey: "fixture",
    ...overrides,
  };
}

/** A settled flat catalog: one page, nothing failed or paging. */
export function sessionListCatalogFixture(
  overrides: Partial<NonNullable<SessionListViewModel["catalog"]>> = {}
): NonNullable<SessionListViewModel["catalog"]> {
  return {
    sessions: [],
    facets: undefined,
    total: undefined,
    loading: false,
    placeholder: false,
    failed: false,
    next: false,
    previous: false,
    paging: false,
    nextPage: noop,
    previousPage: noop,
    retry: noop,
    ...overrides,
  };
}
