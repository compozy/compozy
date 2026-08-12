import { useContext } from "react";

import { NetworkLiveDataContext } from "../contexts/network-live-data-context";

/** Defaults to live for network surfaces rendered outside an OS window. */
export function useNetworkLiveDataEnabled(): boolean {
  return useContext(NetworkLiveDataContext);
}
