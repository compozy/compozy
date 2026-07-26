import { format } from "node:util";

import { InternalError } from "./errors.js";
import type { HostAPI } from "./host-api.js";
import type { ExtensionContext, ExtensionSession } from "./extension-contract.js";
import type { JSONRPCRequestEnvelope } from "./types.js";

export function makeExtensionContext(
  request: JSONRPCRequestEnvelope,
  host: HostAPI,
  session: ExtensionSession | undefined,
  stderr: NodeJS.WritableStream
): ExtensionContext {
  if (!session) {
    throw new InternalError("session is not initialized");
  }
  if (request.id === undefined) {
    throw new InternalError("request id is required");
  }
  return {
    request,
    requestId: request.id,
    host,
    session,
    log: (...values: unknown[]) => {
      stderr.write(`${format(...values)}\n`);
    },
  };
}

export function writeExtensionError(
  stderr: NodeJS.WritableStream,
  message: string,
  error: unknown
): void {
  const detail = error instanceof Error ? (error.stack ?? error.message) : String(error);
  stderr.write(`${message}: ${detail}\n`);
}
