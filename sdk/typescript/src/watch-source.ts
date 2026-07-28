import { InvalidParamsError } from "./errors.js";
import { cloneJSON } from "./json.js";
import type { HostAPI } from "./host-api.js";
import type { ExtensionContext, ExtensionSession } from "./extension.js";
import type { ExtensionDefinition, JSONRPCRequestEnvelope, JSONValue } from "./types.js";

export const WATCH_SOURCE_CAPABILITY = "loop.watch_source";
export const WATCH_POLL_METHOD = "watch/poll";

export interface ExtensionWatchSourceOptions {}

export interface WatchPollRequest {
  spec: JSONValue;
  expected_state_digest?: string;
}

export interface WatchPollResponse {
  ready: boolean;
  state_digest?: string;
  payload?: JSONValue;
  settled_at?: string;
}

export interface ExtensionWatchSourceContext<TSpec extends JSONValue = JSONValue> {
  readonly kind: string;
  readonly spec: TSpec;
  readonly expectedStateDigest?: string;
  readonly context: ExtensionContext;
  readonly host: HostAPI;
  readonly session: ExtensionSession;
}

export type ExtensionWatchSourceHandler<TSpec extends JSONValue = JSONValue> = (
  request: ExtensionWatchSourceContext<TSpec>
) => Promise<WatchPollResponse> | WatchPollResponse;

export interface RegisteredWatchSource {
  kind: string;
  handler: ExtensionWatchSourceHandler;
}

interface ExtensionWatchSourcesOptions {
  bindMethod: (method: string) => void;
  hasUserHandler: (method: string) => boolean;
  makeContext: (request: JSONRPCRequestEnvelope) => ExtensionContext;
}

export class ExtensionWatchSources {
  private readonly handlers = new Map<string, RegisteredWatchSource>();

  public constructor(private readonly options: ExtensionWatchSourcesOptions) {}

  public register<TSpec extends JSONValue = JSONValue>(
    definition: ExtensionDefinition,
    kind: string,
    _registerOptions: ExtensionWatchSourceOptions,
    handler: ExtensionWatchSourceHandler<TSpec>
  ): void {
    const cleanKind = kind.trim();
    if (!cleanKind) {
      throw new Error("watch source kind is required");
    }
    if (this.handlers.has(cleanKind)) {
      throw new Error(`watch source already registered: ${cleanKind}`);
    }
    if (this.options.hasUserHandler(WATCH_POLL_METHOD)) {
      throw new Error("watch/poll is reserved by extension.watchSource()");
    }
    this.handlers.set(cleanKind, {
      kind: cleanKind,
      handler: handler as ExtensionWatchSourceHandler,
    });
    this.ensureCapability(definition);
    this.options.bindMethod(WATCH_POLL_METHOD);
  }

  public hasHandlers(): boolean {
    return this.handlers.size > 0;
  }

  public implementedMethods(): string[] {
    return this.hasHandlers() ? [WATCH_POLL_METHOD] : [];
  }

  public bindMethods(): void {
    if (this.hasHandlers()) {
      this.options.bindMethod(WATCH_POLL_METHOD);
    }
  }

  public kinds(): string[] {
    return Array.from(this.handlers.keys()).sort();
  }

  public initializeResponseFields(): { watch_source_kinds?: string[] } {
    return this.hasHandlers() ? { watch_source_kinds: this.kinds() } : {};
  }

  public async handlePoll(
    request: JSONRPCRequestEnvelope,
    params: unknown
  ): Promise<WatchPollResponse> {
    const poll = parseWatchPollRequest(params);
    const registered = this.handlers.get(poll.kind);
    if (!registered) {
      throw new InvalidParamsError("watch source kind is not registered", { kind: poll.kind });
    }

    const context = this.options.makeContext(request);
    const response = await registered.handler({
      kind: registered.kind,
      spec: poll.spec,
      context,
      host: context.host,
      session: context.session,
      ...(poll.expected_state_digest ? { expectedStateDigest: poll.expected_state_digest } : {}),
    });
    return normalizeWatchPollResponse(response);
  }

  private ensureCapability(definition: ExtensionDefinition): void {
    const capabilities = definition.capabilities ?? {};
    capabilities.provides = normalizeStringList([
      ...(capabilities.provides ?? []),
      WATCH_SOURCE_CAPABILITY,
    ]);
    definition.capabilities = capabilities;
  }
}

export function isWatchSourceMethod(method: string): boolean {
  return method === WATCH_POLL_METHOD;
}

export function parseWatchPollRequest(params: unknown): WatchPollRequest & { kind: string } {
  if (typeof params !== "object" || params === null) {
    throw new InvalidParamsError("watch/poll params must be an object");
  }
  const request = params as WatchPollRequest;
  if (typeof request.spec !== "object" || request.spec === null || Array.isArray(request.spec)) {
    throw new InvalidParamsError("spec must be an object");
  }
  const kind = (request.spec as Record<string, JSONValue>).kind;
  if (typeof kind !== "string" || kind.trim() === "") {
    throw new InvalidParamsError("spec.kind is required");
  }
  if (
    request.expected_state_digest !== undefined &&
    typeof request.expected_state_digest !== "string"
  ) {
    throw new InvalidParamsError("expected_state_digest must be a string");
  }
  return {
    spec: cloneJSON(request.spec),
    kind: kind.trim(),
    ...(request.expected_state_digest
      ? { expected_state_digest: request.expected_state_digest.trim() }
      : {}),
  };
}

export function normalizeWatchPollResponse(response: WatchPollResponse): WatchPollResponse {
  if (typeof response !== "object" || response === null) {
    throw new InvalidParamsError("watch source response must be an object");
  }
  if (typeof response.ready !== "boolean") {
    throw new InvalidParamsError("watch source response.ready must be a boolean");
  }
  if (response.state_digest !== undefined && typeof response.state_digest !== "string") {
    throw new InvalidParamsError("watch source response.state_digest must be a string");
  }
  if (response.settled_at !== undefined && typeof response.settled_at !== "string") {
    throw new InvalidParamsError("watch source response.settled_at must be a string");
  }
  return {
    ready: response.ready,
    ...(response.state_digest ? { state_digest: response.state_digest } : {}),
    ...(response.payload !== undefined ? { payload: cloneJSON(response.payload) } : {}),
    ...(response.settled_at ? { settled_at: response.settled_at } : {}),
  };
}

function normalizeStringList(values: readonly string[] | undefined): string[] {
  return Array.from(new Set((values ?? []).map(value => value.trim()).filter(Boolean))).sort();
}
