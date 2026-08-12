import { useState } from "react";

import { NetworkLiveDataContext } from "@/systems/network";

import { useDesktop } from "../../hooks/use-desktop";
import { useOsShell } from "../../hooks/use-os-shell";
import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import {
  NetworkWindowController,
  type NetworkWindowLocation,
  type NetworkWindowNavigation,
} from "@/systems/network";

const DEFAULT_NETWORK_ROUTE = { pathname: "/network", search: {} } as const;

/** OS adapter: WM location in, routing-coordinator transition out. */
export function NetworkWindow({ windowId }: { windowId: string }) {
  const { coordinator } = useOsShell();
  const active = useCurrentWindowLiveDataEnabled();
  const location = useDesktop(state => state.windows[windowId]?.route ?? DEFAULT_NETWORK_ROUTE);
  const [navigation] = useState<NetworkWindowNavigation>(() => ({
    push: (next: NetworkWindowLocation) => {
      void coordinator.userOpen({ app: "network", route: next });
    },
  }));

  return (
    <NetworkLiveDataContext value={active}>
      <NetworkWindowController active={active} location={location} navigation={navigation} />
    </NetworkLiveDataContext>
  );
}
