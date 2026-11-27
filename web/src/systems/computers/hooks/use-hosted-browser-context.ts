import { useContext } from "react";

import {
  HostedBrowserContext,
  type HostedBrowserContextValue,
} from "../contexts/hosted-browser-context";

/** Returns the active hosted-browser session context, or `null` outside a hosted origin. */
export function useHostedBrowserContext(): HostedBrowserContextValue | null {
  return useContext(HostedBrowserContext);
}
