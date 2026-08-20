import type { Extension } from "./extension.js";
import type { ExtensionContext } from "./extension-contract.js";
import { registerProvideSurface } from "./extension-provide-surface.js";
import { requestRecord, requiredString } from "./protocol-params.js";
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

function parseEstablishRequest(request: unknown): ConnectivityEstablishRequest {
  const record = requestRecord(CONNECTIVITY_ESTABLISH_METHOD, request);
  return {
    tier: requiredString(CONNECTIVITY_ESTABLISH_METHOD, record, "tier"),
    forward_target: requiredString(CONNECTIVITY_ESTABLISH_METHOD, record, "forward_target"),
    challenge_path: requiredString(CONNECTIVITY_ESTABLISH_METHOD, record, "challenge_path"),
    deadline: requiredString(CONNECTIVITY_ESTABLISH_METHOD, record, "deadline"),
  };
}

function parseStatusRequest(request: unknown): ConnectivityStatusRequest {
  const record = requestRecord(CONNECTIVITY_STATUS_METHOD, request);
  return {
    tier: requiredString(CONNECTIVITY_STATUS_METHOD, record, "tier"),
  };
}

function parseTeardownRequest(request: unknown): ConnectivityTeardownRequest {
  const record = requestRecord(CONNECTIVITY_TEARDOWN_METHOD, request);
  return {
    tier: requiredString(CONNECTIVITY_TEARDOWN_METHOD, record, "tier"),
    deadline: requiredString(CONNECTIVITY_TEARDOWN_METHOD, record, "deadline"),
  };
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
      handler: async (context, request) =>
        await handlers.establish(context, parseEstablishRequest(request)),
    },
    {
      method: CONNECTIVITY_STATUS_METHOD,
      handler: async (context, request) =>
        await handlers.status(context, parseStatusRequest(request)),
    },
    {
      method: CONNECTIVITY_TEARDOWN_METHOD,
      handler: async (context, request) =>
        await handlers.teardown(context, parseTeardownRequest(request)),
    },
  ]);
}
