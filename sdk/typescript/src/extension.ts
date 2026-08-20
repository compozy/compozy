import type { TransportLike } from "./transport.js";
import {
  ExtensionWatchSources,
  WATCH_POLL_METHOD,
  isWatchSourceMethod,
  type ExtensionWatchSourceHandler,
  type ExtensionWatchSourceOptions,
} from "./watch-source.js";
import { HostAPI } from "./host-api.js";
import type {
  ExtensionDefinition,
  HealthCheckResult,
  HookEvent,
  ExtensionCommandGroupSpec,
  ExtensionToolRuntimeDescriptor,
  JSONRPCRequestEnvelope,
  JSONValue,
  ShutdownResponse,
} from "./types.js";
import { StdioTransport } from "./transport.js";
import {
  PROVIDE_TOOLS_METHOD,
  TOOL_PROVIDER_CAPABILITY,
  TOOLS_CALL_METHOD,
  isToolProviderMethod,
} from "./capabilities.js";
import {
  InvalidParamsError,
  MethodNotFoundError,
  NotInitializedError,
  ShutdownInProgressError,
} from "./errors.js";

import {
  implementedExtensionMethods,
  normalizeStringList,
  parseShutdownRequest,
  transportMethods,
} from "./extension-runtime.js";
import { SDK_VERSION } from "./extension-contract.js";
import { makeExtensionContext, writeExtensionError } from "./extension-context.js";
import { buildExtensionSession } from "./extension-session.js";
import {
  buildExtensionDescribePayload,
  cloneExtensionToolDescriptors,
  defaultExtensionDescribeProcess,
  runExtensionDescribeMode,
} from "./extension-describe.js";
import { buildRegisteredTool } from "./extension-tool-registration.js";
import { callRegisteredTool, provideRegisteredTools } from "./extension-tool-dispatch.js";
import {
  ExtensionProvideSurfaces,
  registerProvideSurface,
  type ProvideSurfaceRegistration,
} from "./extension-provide-surface.js";
import type {
  ExtensionContext,
  ExtensionHandler,
  ExtensionOptions,
  ExtensionSession,
  ExtensionToolHandler,
  ExtensionToolOptions,
  ReadyCallback,
  RegisteredTool,
} from "./extension-contract.js";

export class Extension {
  private transport: TransportLike;
  private readonly stderr: NodeJS.WritableStream;
  private readonly sdkVersion: string;
  private readonly describeProcess: NonNullable<ExtensionOptions["describeProcess"]>;
  private readonly handlers = new Map<string, ExtensionHandler>();
  private readonly toolHandlers = new Map<string, RegisteredTool>();
  private readonly commandGroups: ExtensionCommandGroupSpec[] = [];
  private readonly watchSources = new ExtensionWatchSources({
    bindMethod: method => this.bindMethod(method),
    hasUserHandler: method => this.handlers.has(method),
    makeContext: request => this.makeContext(request),
  });
  private readonly readyCallbacks = new Set<ReadyCallback>();
  private readonly transportBindings = new Set<string>();
  private readonly requestSignals = new Map<string, AbortSignal>();
  private readonly provideSurfaces = new ExtensionProvideSurfaces({
    handlers: this.handlers,
    validateMethod: method => {
      if (this.toolHandlers.size > 0 && isToolProviderMethod(method)) {
        throw new Error(`${method} is reserved by extension.tool()`);
      }
      if (this.watchSources.hasHandlers() && isWatchSourceMethod(method)) {
        throw new Error(`${method} is reserved by extension.watchSource()`);
      }
    },
    bindMethod: method => this.bindMethod(method),
    unbindMethod: method => {
      this.transport.unhandle(method);
      this.transportBindings.delete(method);
    },
  });
  private readonly host: HostAPI;
  private initialized = false;
  private shutdownStarted = false;
  private shutdownDeadlineMS: number | undefined;
  private session: ExtensionSession | undefined;
  private startPromise: Promise<HostAPI> | undefined;
  private resolveStart: ((host: HostAPI) => void) | undefined;
  private rejectStart: ((reason: unknown) => void) | undefined;

  public constructor(
    public readonly definition: ExtensionDefinition,
    options: ExtensionOptions = {}
  ) {
    this.transport = options.transport ?? new StdioTransport();
    this.stderr = options.stderr ?? process.stderr;
    this.sdkVersion = options.sdkVersion ?? SDK_VERSION;
    this.describeProcess = options.describeProcess ?? defaultExtensionDescribeProcess();
    this.host = new HostAPI(
      {
        call: async <TResult>(method: string, params?: unknown): Promise<TResult> =>
          await this.transport.call<TResult>(method, params),
      },
      { isReady: () => this.initialized && !this.shutdownStarted }
    );

    this.bindTransportHandlers();
  }

  public bindTransport(transport: TransportLike): this {
    if (this.startPromise) {
      throw new Error("transport may only be swapped before start()");
    }
    this.transport = transport;
    this.transportBindings.clear();
    this.bindTransportHandlers();
    return this;
  }

  public handle<TParams = unknown, TResult = unknown>(
    method: string,
    handler: ExtensionHandler<TParams, TResult>
  ): this {
    this.ensureRegistrationOpen();
    const cleanMethod = method.trim();
    if (!cleanMethod) {
      throw new Error("method is required");
    }
    if (cleanMethod === "initialize") {
      throw new Error("initialize is reserved by the SDK");
    }
    const provideOwner = this.provideSurfaces.owner(cleanMethod);
    if (provideOwner) {
      throw new Error(`${cleanMethod} is reserved by ${provideOwner}`);
    }
    if (this.toolHandlers.size > 0 && isToolProviderMethod(cleanMethod)) {
      throw new Error(`${cleanMethod} is reserved by extension.tool()`);
    }
    if (this.watchSources.hasHandlers() && isWatchSourceMethod(cleanMethod)) {
      throw new Error(`${cleanMethod} is reserved by extension.watchSource()`);
    }
    this.handlers.set(cleanMethod, handler as ExtensionHandler);
    this.bindMethod(cleanMethod);
    return this;
  }

  public [registerProvideSurface](
    capability: string,
    registrations: readonly ProvideSurfaceRegistration[]
  ): this {
    this.ensureRegistrationOpen();
    this.provideSurfaces.register(this.definition, capability, registrations);
    return this;
  }

  public tool<TInput = unknown>(
    handler: string,
    options: ExtensionToolOptions,
    toolHandler: ExtensionToolHandler<TInput>
  ): this {
    this.ensureRegistrationOpen();
    const cleanHandler = handler.trim();
    if (!cleanHandler) {
      throw new Error("tool handler is required");
    }
    if (this.toolHandlers.has(cleanHandler)) {
      throw new Error(`tool handler already registered: ${cleanHandler}`);
    }
    if (this.handlers.has(PROVIDE_TOOLS_METHOD) || this.handlers.has(TOOLS_CALL_METHOD)) {
      throw new Error("provide_tools and tools/call are reserved by extension.tool()");
    }
    this.toolHandlers.set(
      cleanHandler,
      buildRegisteredTool(this.definition.name, cleanHandler, options, toolHandler)
    );
    this.ensureToolProviderCapability();
    this.ensureToolProviderHandlers();
    return this;
  }

  public watchSource<TSpec extends JSONValue = JSONValue>(
    kind: string,
    options: ExtensionWatchSourceOptions,
    handler: ExtensionWatchSourceHandler<TSpec>
  ): this {
    this.ensureRegistrationOpen();
    this.watchSources.register(this.definition, kind, options, handler);
    return this;
  }

  public commandGroup(path: string, summary: string): this {
    this.ensureRegistrationOpen();
    this.commandGroups.push({ path: path.trim(), summary: summary.trim() });
    return this;
  }

  private ensureRegistrationOpen(): void {
    if (this.initialized || this.startPromise || this.shutdownStarted) {
      throw new Error("extension registration is closed after start");
    }
  }

  public onReady(callback: ReadyCallback): this {
    this.readyCallbacks.add(callback);
    if (this.initialized && this.session) {
      queueMicrotask(() => {
        void this.runReadyCallback(callback, this.session!);
      });
    }
    return this;
  }

  public async start(): Promise<HostAPI> {
    if (runExtensionDescribeMode(this.describeProcess, () => this.describe())) {
      return this.host;
    }
    if (this.startPromise) {
      return await this.startPromise;
    }

    this.startPromise = new Promise<HostAPI>((resolve, reject) => {
      this.resolveStart = resolve;
      this.rejectStart = reject;
    });

    this.transport.onTransportError(error => {
      if (!this.initialized && this.rejectStart) {
        this.rejectStart(error);
      }
      this.logError("transport error", error);
    });
    this.transport.start();

    return await this.startPromise;
  }

  public getImplementedMethods(): string[] {
    return implementedExtensionMethods(
      this.handlers.keys(),
      this.toolHandlers.size > 0,
      this.watchSources.implementedMethods()
    );
  }

  public getSupportedHookEvents(): HookEvent[] {
    return [...(this.definition.supported_hook_events ?? [])];
  }

  public getToolDescriptors(): ExtensionToolRuntimeDescriptor[] {
    return cloneExtensionToolDescriptors(this.toolHandlers.values());
  }

  public describe() {
    return buildExtensionDescribePayload({
      definition: this.definition,
      tools: this.getToolDescriptors(),
      commandGroups: this.commandGroups.map(group => ({ ...group })),
      watchSourceKinds: this.watchSources.kinds(),
      sdkVersion: this.sdkVersion,
    });
  }

  private bindTransportHandlers(): void {
    for (const method of transportMethods(this.handlers.keys(), this.toolHandlers.size > 0)) {
      this.bindMethod(method);
    }
    this.watchSources.bindMethods();
  }

  private bindMethod(method: string): void {
    if (this.transportBindings.has(method)) {
      this.transport.handle(
        method,
        async (params, request, signal) => await this.dispatch(method, params, request, signal)
      );
      return;
    }
    this.transport.handle(
      method,
      async (params, request, signal) => await this.dispatch(method, params, request, signal)
    );
    this.transportBindings.add(method);
  }

  private async dispatch(
    method: string,
    params: unknown,
    request: JSONRPCRequestEnvelope,
    signal: AbortSignal
  ): Promise<unknown> {
    const requestKey = `${typeof request.id}:${String(request.id)}`;
    this.requestSignals.set(requestKey, signal);
    try {
      if (method === "initialize") {
        return await this.handleInitialize(params);
      }
      if (!this.initialized || !this.session) {
        throw new NotInitializedError();
      }
      if (this.shutdownStarted && method !== "shutdown") {
        throw new ShutdownInProgressError(
          this.shutdownDeadlineMS === undefined ? {} : { deadline_ms: this.shutdownDeadlineMS }
        );
      }

      switch (method) {
        case "health_check":
          return await this.handleHealthCheck(request, params);
        case "shutdown":
          return await this.handleShutdown(request, params);
        case PROVIDE_TOOLS_METHOD:
          return provideRegisteredTools(this.getToolDescriptors());
        case TOOLS_CALL_METHOD:
          return await this.handleToolCall(request, params);
        case WATCH_POLL_METHOD:
          return await this.watchSources.handlePoll(request, params);
        default:
          return await this.handleUserMethod(method, request, params);
      }
    } finally {
      this.requestSignals.delete(requestKey);
    }
  }

  private async handleInitialize(params: unknown) {
    if (this.initialized) {
      throw new InvalidParamsError("initialize may only be called once");
    }

    const { response, session } = buildExtensionSession({
      definition: this.definition,
      params,
      implementedMethods: this.getImplementedMethods(),
      supportedHookEvents: this.getSupportedHookEvents(),
      watchSourceKinds: this.watchSources.kinds(),
      sdkVersion: this.sdkVersion,
    });
    this.initialized = true;
    this.session = session;

    setImmediate(() => {
      void this.finishInitialization();
    });

    return response;
  }

  private async finishInitialization(): Promise<void> {
    if (!this.session) {
      return;
    }
    for (const callback of this.readyCallbacks) {
      await this.runReadyCallback(callback, this.session);
    }
    this.resolveStart?.(this.host);
    this.resolveStart = undefined;
    this.rejectStart = undefined;
  }

  private async runReadyCallback(
    callback: ReadyCallback,
    session: ExtensionSession
  ): Promise<void> {
    try {
      await callback(this.host, session);
    } catch (error) {
      this.logError("onReady callback failed", error);
    }
  }

  private async handleHealthCheck(
    request: JSONRPCRequestEnvelope,
    params: unknown
  ): Promise<HealthCheckResult> {
    const customHandler = this.handlers.get("health_check");
    if (!customHandler) {
      return {
        healthy: true,
        message: "",
        details: {},
      };
    }
    return (await customHandler(this.makeContext(request), params as never)) as HealthCheckResult;
  }

  private async handleShutdown(
    request: JSONRPCRequestEnvelope,
    params: unknown
  ): Promise<ShutdownResponse> {
    const shutdownRequest = parseShutdownRequest(params);
    this.shutdownStarted = true;
    this.shutdownDeadlineMS = shutdownRequest.deadline_ms;

    const customHandler = this.handlers.get("shutdown");
    if (customHandler) {
      const result = (await customHandler(
        this.makeContext(request),
        shutdownRequest as never
      )) as ShutdownResponse;
      return result ?? { acknowledged: true };
    }
    return { acknowledged: true };
  }

  private async handleToolCall(request: JSONRPCRequestEnvelope, params: unknown) {
    return await callRegisteredTool(this.toolHandlers, params, this.makeContext(request));
  }

  private async handleUserMethod(
    method: string,
    request: JSONRPCRequestEnvelope,
    params: unknown
  ): Promise<unknown> {
    const handler = this.handlers.get(method);
    if (!handler) {
      throw new MethodNotFoundError(method);
    }
    return await handler(this.makeContext(request), params as never);
  }

  private makeContext(request: JSONRPCRequestEnvelope): ExtensionContext {
    const requestKey = `${typeof request.id}:${String(request.id)}`;
    const signal = this.requestSignals.get(requestKey) ?? new AbortController().signal;
    return makeExtensionContext(request, signal, this.host, this.session, this.stderr);
  }

  private logError(message: string, error: unknown): void {
    writeExtensionError(this.stderr, message, error);
  }

  private ensureToolProviderCapability(): void {
    const capabilities = this.definition.capabilities ?? {};
    capabilities.provides = normalizeStringList([
      ...(capabilities.provides ?? []),
      TOOL_PROVIDER_CAPABILITY,
    ]);
    this.definition.capabilities = capabilities;
  }

  private ensureToolProviderHandlers(): void {
    this.bindMethod(PROVIDE_TOOLS_METHOD);
    this.bindMethod(TOOLS_CALL_METHOD);
  }
}

export type {
  ExtensionContext,
  ExtensionDescribeProcess,
  ExtensionHandler,
  ExtensionOptions,
  ExtensionSession,
  ExtensionToolContext,
  ExtensionToolHandler,
  ExtensionToolOptions,
} from "./extension-contract.js";
