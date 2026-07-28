import { createContext } from "react";

import type { NetworkListFiltersStore } from "../hooks/use-network-list-filters";

const NetworkListFiltersContext = createContext<NetworkListFiltersStore | null>(null);

export { NetworkListFiltersContext };
