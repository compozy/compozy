import { useSyncExternalStore } from "react";

import { subscribeSearch } from "./contract";

/** The current query string as React state; every gallery and prototype view reads the URL. */
export function useSearch() {
  return useSyncExternalStore(subscribeSearch, () => window.location.search);
}
