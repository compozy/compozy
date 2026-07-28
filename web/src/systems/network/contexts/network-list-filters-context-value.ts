import { createContext } from "react";

import type { UseNetworkListFiltersResult } from "../hooks/use-network-list-filters";

const NetworkListFiltersContext = createContext<UseNetworkListFiltersResult | null>(null);

export { NetworkListFiltersContext };
