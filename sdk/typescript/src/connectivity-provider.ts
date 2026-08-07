import type { Extension } from "./extension.js";
import type { ExtensionContext, ExtensionHandler } from "./extension-contract.js";
import { registerProvideSurface } from "./extension-provide-surface.js";
import type {
  ConnectivityEstablishRequest,
  ConnectivityReachability,
  ConnectivityStatusRequest,
  ConnectivityTeardownRequest,
  ConnectivityTeardownResponse,
} from "./types.js";

export const CONNECTIVITY_PROVIDER_CAPABILITY = "connectivity.provider";
export const CONNECTIVITY_ESTABLISH_METHOD = "connectivity/establish";
export const CONNECTIVITY_STATUS_METHOD = "connectivity/status";
export const CONNECTIVITY_TEARDOWN_METHOD = "connectivity/teardown";

export interface ConnectivityProviderHandlers {
  establish: (
    context: ExtensionContext,
    request: ConnectivityEstablishRequest
  ) => Promise<ConnectivityReachability> | ConnectivityReachability;
  status: (
    context: ExtensionContext,
    request: ConnectivityStatusRequest
  ) => Promise<ConnectivityReachability> | ConnectivityReachability;
  teardown: (
    context: ExtensionContext,
    request: ConnectivityTeardownRequest
  ) => Promise<ConnectivityTeardownResponse> | ConnectivityTeardownResponse;
}

export function registerConnectivityProvider(
  extension: Extension,
  handlers: ConnectivityProviderHandlers
): Extension {
  if (!extension) {
    throw new Error("extension is required");
  }
  if (
    typeof handlers?.establish !== "function" ||
    typeof handlers.status !== "function" ||
    typeof handlers.teardown !== "function"
  ) {
    throw new Error("connectivity provider handlers are required");
  }
  return extension[registerProvideSurface](CONNECTIVITY_PROVIDER_CAPABILITY, [
    {
      method: CONNECTIVITY_ESTABLISH_METHOD,
      handler: (async (context, request) =>
        await handlers.establish(
          context,
          request as ConnectivityEstablishRequest
        )) as ExtensionHandler,
    },
    {
      method: CONNECTIVITY_STATUS_METHOD,
      handler: (async (context, request) =>
        await handlers.status(context, request as ConnectivityStatusRequest)) as ExtensionHandler,
    },
    {
      method: CONNECTIVITY_TEARDOWN_METHOD,
      handler: (async (context, request) =>
        await handlers.teardown(
          context,
          request as ConnectivityTeardownRequest
        )) as ExtensionHandler,
    },
  ]);
}
