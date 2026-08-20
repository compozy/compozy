import type { Extension } from "./extension.js";
import type { ExtensionContext } from "./extension-contract.js";
import { InvalidParamsError } from "./errors.js";
import { registerProvideSurface } from "./extension-provide-surface.js";
import { isRequestRecord, requestRecord, requiredString } from "./protocol-params.js";
import type { ViewCloseRequest, ViewEvent, ViewFrame, ViewOpenRequest } from "./types.js";
import { ViewSessionRegistry } from "./view-session-registry.js";

export const VIEW_PROVIDER_CAPABILITY = "view.provider";
export const VIEW_OPEN_METHOD = "view/open";
export const VIEW_EVENT_METHOD = "view/event";
export const VIEW_CLOSE_METHOD = "view/close";

export interface ViewProviderHandlers {
  open: (context: ExtensionContext, request: ViewOpenRequest) => Promise<ViewFrame> | ViewFrame;
  event: (
    context: ExtensionContext,
    request: ViewEvent
  ) => Promise<ViewFrame | void> | ViewFrame | void;
  close: (context: ExtensionContext, request: ViewCloseRequest) => Promise<void> | void;
}

export interface ViewProviderOptions {
  registry?: ViewSessionRegistry;
}

export function registerViewProvider(
  extension: Extension,
  handlers: ViewProviderHandlers,
  options: ViewProviderOptions = {}
): Extension {
  if (!extension) {
    throw new Error("extension is required");
  }
  if (
    typeof handlers?.open !== "function" ||
    typeof handlers.event !== "function" ||
    typeof handlers.close !== "function"
  ) {
    throw new Error("view provider handlers are required");
  }
  const registry = options.registry ?? new ViewSessionRegistry();

  return extension[registerProvideSurface](VIEW_PROVIDER_CAPABILITY, [
    {
      method: VIEW_OPEN_METHOD,
      handler: async (context, params) => {
        const request = parseOpenRequest(params);
        registry.open(request);
        try {
          const frame = await handlers.open(context, request);
          validateFrame(frame, request.view_session);
          return frame;
        } catch (error) {
          registry.close(request.view_session);
          throw error;
        }
      },
    },
    {
      method: VIEW_EVENT_METHOD,
      handler: async (context, params) => {
        const request = parseEvent(params);
        registry.require(request.view_session);
        registry.admitGeneration(request.view_session, request.generation);
        const frame = await handlers.event(context, request);
        if (frame !== undefined) {
          validateFrame(frame, request.view_session);
        }
        return frame ?? {};
      },
    },
    {
      method: VIEW_CLOSE_METHOD,
      handler: async (context, params) => {
        const request = parseCloseRequest(params);
        if (!registry.close(request.view_session)) {
          return {};
        }
        await handlers.close(context, request);
        return {};
      },
    },
  ]);
}

function parseOpenRequest(params: unknown): ViewOpenRequest {
  const record = requestRecord(VIEW_OPEN_METHOD, params);
  const args = record.args;
  if (args !== undefined && !isRequestRecord(args)) {
    throw new InvalidParamsError(`${VIEW_OPEN_METHOD} args must be an object`);
  }
  return {
    view_session: requiredString(VIEW_OPEN_METHOD, record, "view_session"),
    view: requiredString(VIEW_OPEN_METHOD, record, "view"),
    workspace: requiredString(VIEW_OPEN_METHOD, record, "workspace"),
    client: requiredString(VIEW_OPEN_METHOD, record, "client"),
    ...(args === undefined ? {} : { args }),
  } as ViewOpenRequest;
}

function parseEvent(params: unknown): ViewEvent {
  const record = requestRecord(VIEW_EVENT_METHOD, params);
  const seq = requiredPositiveInteger(VIEW_EVENT_METHOD, record, "seq");
  const generation = requiredPositiveInteger(VIEW_EVENT_METHOD, record, "generation");
  return {
    view_session: requiredString(VIEW_EVENT_METHOD, record, "view_session"),
    handler: requiredString(VIEW_EVENT_METHOD, record, "handler"),
    revision: requiredString(VIEW_EVENT_METHOD, record, "revision"),
    seq,
    generation,
    ...(Array.isArray(record.args) ? { args: record.args as NonNullable<ViewEvent["args"]> } : {}),
    ...(Array.isArray(record.ack_effects) ? { ack_effects: record.ack_effects as string[] } : {}),
    ...(isRequestRecord(record.effect_result)
      ? {
          effect_result: record.effect_result as unknown as NonNullable<ViewEvent["effect_result"]>,
        }
      : {}),
  };
}

function parseCloseRequest(params: unknown): ViewCloseRequest {
  const record = requestRecord(VIEW_CLOSE_METHOD, params);
  const reason = record.reason;
  if (reason !== undefined && typeof reason !== "string") {
    throw new InvalidParamsError(`${VIEW_CLOSE_METHOD} reason must be a string`);
  }
  return {
    view_session: requiredString(VIEW_CLOSE_METHOD, record, "view_session"),
    ...(reason === undefined ? {} : { reason }),
  };
}

function validateFrame(frame: ViewFrame, viewSession: string): void {
  if (!isRequestRecord(frame)) {
    throw new InvalidParamsError("view provider must return a frame object");
  }
  if (frame.view_session !== viewSession) {
    throw new InvalidParamsError("view frame session does not match the request");
  }
  if (typeof frame.revision !== "string" || frame.revision.length === 0) {
    throw new InvalidParamsError("view frame revision is required");
  }
  if (!Number.isSafeInteger(frame.generation) || frame.generation < 0) {
    throw new InvalidParamsError("view frame generation must be a non-negative integer");
  }
  if ((frame.payload === undefined) === (frame.patch === undefined)) {
    throw new InvalidParamsError("view frame must include exactly one of payload or patch");
  }
  if (!Array.isArray(frame.handlers)) {
    throw new InvalidParamsError("view frame handlers are required");
  }
}

function requiredPositiveInteger(
  method: string,
  record: Record<string, unknown>,
  field: string
): number {
  const value = record[field];
  if (!Number.isSafeInteger(value) || (value as number) <= 0) {
    throw new InvalidParamsError(`${method} requires a positive integer ${field}`);
  }
  return value as number;
}
