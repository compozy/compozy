import { useState } from "react";

import {
  NetworkWindowController,
  type NetworkWindowLocation,
  type NetworkWindowNavigation,
} from "@/systems/network";

import { useDesktop } from "../../hooks/use-desktop";
import { useOsShell } from "../../hooks/use-os-shell";

const DEFAULT_NETWORK_ROUTE = { pathname: "/network", search: {} } as const;

/** OS adapter: WM location in, routing-coordinator transition out. */
export function NetworkWindow({ windowId }: { windowId: string }) {
  const { coordinator } = useOsShell();
  const active = useDesktop(state => state.focusedId === windowId);
  const location = useDesktop(state => state.windows[windowId]?.route ?? DEFAULT_NETWORK_ROUTE);
  const [navigation] = useState<NetworkWindowNavigation>(() => ({
    push: (next: NetworkWindowLocation) => {
      void coordinator.userOpen({ app: "network", route: next });
    },
  }));

  return <NetworkWindowController active={active} location={location} navigation={navigation} />;
}
