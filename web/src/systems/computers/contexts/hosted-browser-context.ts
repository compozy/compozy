import { createContext } from "react";

import type {
  ComputerFixture,
  HostedPeerFixture,
  RemoteScope,
} from "../mocks/remote-computers-types";

export interface HostedBrowserContextValue {
  /** The hosted computer currently serving this browser tab. */
  computer: ComputerFixture;
  /** Current computer + other account computers with Hosted route enabled. */
  peers: readonly HostedPeerFixture[];
  /** Scopes granted to this browser session on the current computer. */
  scopes: readonly RemoteScope[];
  /** True when this browser session may open and operate terminals. */
  canOperateTerminal: boolean;
  /** True while the browser lost connectivity and is reconnecting. */
  reconnecting: boolean;
  /** Opens another hosted computer's own address (`https://<slug>.compozy.run`). */
  selectPeer: (peer: HostedPeerFixture) => void;
  /** Ends the current browser session without revoking the device. */
  signOut: () => void;
  /** Retries connection / peer refresh when offline. */
  retryConnection: () => void;
}

export const HostedBrowserContext = createContext<HostedBrowserContextValue | null>(null);
