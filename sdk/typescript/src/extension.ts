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
  ExtensionProvideToolsResponse,
  HealthCheckResult,
  HookEvent,
  ExtensionToolCallResponse,
  ExtensionToolRuntimeDescriptor,
  InitializeResponse,
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
  validateProvidedMethodCoverage,
} from "./capabilities.js";
import { schemaDigest } from "./schema-digest.js";
import {
  InvalidParamsError,
  MethodNotFoundError,
  NotInitializedError,
  ShutdownInProgressError,
  isRPCError,
} from "./errors.js";

import {
  canonicalExtensionToolID,
  ensureSubset,
  normalizeHostMethodList,
  normalizeSchema,
  normalizeStringList,
  normalizeToolResult,
  parseShutdownRequest,
  parseToolCallRequest,
  toolExecutionError,
} from "./extension-runtime.js";
import { SDK_NAME, SDK_VERSION } from "./extension-contract.js";
import { makeExtensionContext, writeExtensionError } from "./extension-context.js";
import { parseInitializeRequest, validateProtocolVersion } from "./extension-initialize.js";
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
  private readonly handlers = new Map<string, ExtensionHandler>();
  private readonly toolHandlers = new Map<string, RegisteredTool>();
  private readonly watchSources = new ExtensionWatchSources({
    bindMethod: method => this.bindMethod(method),
    hasUserHandler: method => this.handlers.has(method),
    makeContext: request => this.makeContext(request),
  });
  private readonly readyCallbacks = new Set<ReadyCallback>();
  private readonly transportBindings = new Set<string>();
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
    const cleanMethod = method.trim();
    if (cleanMethod === "initialize") {
      throw new Error("initialize is reserved by the SDK");
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

  public tool<TInput = unknown>(
    handler: string,
    options: ExtensionToolOptions,
    toolHandler: ExtensionToolHandler<TInput>
  ): this {
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
    const inputSchema = normalizeSchema(options.inputSchema, "inputSchema");
    const outputSchema =
      options.outputSchema === undefined
        ? undefined
        : normalizeSchema(options.outputSchema, "outputSchema");
    const descriptor: ExtensionToolRuntimeDescriptor = {
      id: options.id ?? canonicalExtensionToolID(this.definition.name, cleanHandler),
      handler: cleanHandler,
      input_schema_digest: schemaDigest(inputSchema),
      ...(outputSchema ? { output_schema_digest: schemaDigest(outputSchema) } : {}),
      read_only: Boolean(options.readOnly),
      risk: options.risk ?? (options.readOnly ? "read" : "mutating"),
      capabilities: normalizeStringList(options.capabilities),
    };
    this.toolHandlers.set(cleanHandler, {
      descriptor,
      handler: toolHandler as ExtensionToolHandler,
      sensitiveInputFields: normalizeStringList(options.sensitiveInputFields),
    });
    this.ensureToolProviderCapability();
    this.ensureToolProviderHandlers();
    return this;
  }

  public watchSource<TSpec extends JSONValue = JSONValue>(
    kind: string,
    options: ExtensionWatchSourceOptions,
    handler: ExtensionWatchSourceHandler<TSpec>
  ): this {
    this.watchSources.register(this.definition, kind, options, handler);
    return this;
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
    const methods = new Set<string>(["health_check", "shutdown"]);
    for (const method of this.handlers.keys()) {
      methods.add(method);
    }
    if (this.toolHandlers.size > 0) {
      methods.add(PROVIDE_TOOLS_METHOD);
      methods.add(TOOLS_CALL_METHOD);
    }
    for (const method of this.watchSources.implementedMethods()) {
      methods.add(method);
    }
    return Array.from(methods).sort();
  }

  public getSupportedHookEvents(): HookEvent[] {
    return [...(this.definition.supported_hook_events ?? [])];
  }

  public getToolDescriptors(): ExtensionToolRuntimeDescriptor[] {
    return [...this.toolHandlers.values()].map(tool => ({
      ...tool.descriptor,
      capabilities: [...(tool.descriptor.capabilities ?? [])],
    }));
  }

  private bindTransportHandlers(): void {
    this.bindMethod("initialize");
    this.bindMethod("health_check");
    this.bindMethod("shutdown");
    for (const method of this.handlers.keys()) {
      this.bindMethod(method);
    }
    if (this.toolHandlers.size > 0) {
      this.bindMethod(PROVIDE_TOOLS_METHOD);
      this.bindMethod(TOOLS_CALL_METHOD);
    }
    this.watchSources.bindMethods();
  }

  private bindMethod(method: string): void {
    if (this.transportBindings.has(method)) {
      this.transport.handle(
        method,
        async (params, request) => await this.dispatch(method, params, request)
      );
      return;
    }
    this.transportBindings.add(method);
    this.transport.handle(
      method,
      async (params, request) => await this.dispatch(method, params, request)
    );
  }

  private async dispatch(
    method: string,
    params: unknown,
    request: JSONRPCRequestEnvelope
  ): Promise<unknown> {
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
        return this.handleProvideTools();
      case TOOLS_CALL_METHOD:
        return await this.handleToolCall(request, params);
      case WATCH_POLL_METHOD:
        return await this.watchSources.handlePoll(request, params);
      default:
        return await this.handleUserMethod(method, request, params);
    }
  }

  private async handleInitialize(params: unknown): Promise<InitializeResponse> {
    if (this.initialized) {
      throw new InvalidParamsError("initialize may only be called once");
    }

    const request = parseInitializeRequest(params);
    validateProtocolVersion(request);

    const requestedProvides = normalizeStringList(this.definition.capabilities?.provides);
    const requestedActions = normalizeHostMethodList(this.definition.actions?.requires);
    const requestedSecurity = normalizeStringList(this.definition.security?.capabilities);

    ensureSubset("provides", requestedProvides, request.capabilities.provides);
    ensureSubset("actions", requestedActions, request.capabilities.granted_actions);
    ensureSubset("security", requestedSecurity, request.capabilities.granted_security);

    const implementedMethods = this.getImplementedMethods();
    validateProvidedMethodCoverage(requestedProvides, implementedMethods);

    const response: InitializeResponse = {
      protocol_version: "1",
      extension_info: {
        name: this.definition.name,
        version: this.definition.version,
        sdk_name: SDK_NAME,
        sdk_version: this.sdkVersion,
      },
      accepted_capabilities: {
        provides: requestedProvides,
        actions: requestedActions,
        security: requestedSecurity,
      },
      implemented_methods: implementedMethods,
      supported_hook_events: this.getSupportedHookEvents(),
      ...this.watchSources.initializeResponseFields(),
      supports: {
        health_check: true,
      },
    };

    this.initialized = true;
    this.session = {
      initializeRequest: request,
      initializeResponse: response,
      runtime: request.runtime,
      acceptedCapabilities: response.accepted_capabilities,
    };

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

  private handleProvideTools(): ExtensionProvideToolsResponse {
    return {
      tools: this.getToolDescriptors(),
    };
  }

  private async handleToolCall(
    request: JSONRPCRequestEnvelope,
    params: unknown
  ): Promise<ExtensionToolCallResponse> {
    const call = parseToolCallRequest(params);
    const registered = this.toolHandlers.get(call.handler);
    if (!registered) {
      throw new MethodNotFoundError(call.handler);
    }
    if (registered.descriptor.id !== call.tool_id) {
      throw new InvalidParamsError("tool_id does not match handler", {
        expected_tool_id: registered.descriptor.id,
        actual_tool_id: call.tool_id,
        handler: call.handler,
      });
    }

    const context = this.makeContext(request);
    try {
      const result = await registered.handler({
        input: call.input,
        context,
        host: context.host,
        session: context.session,
        toolID: call.tool_id,
        handler: call.handler,
      });
      return { result: normalizeToolResult(result) };
    } catch (error) {
      if (isRPCError(error)) {
        throw error;
      }
      throw toolExecutionError(error, call, registered.sensitiveInputFields);
    }
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
    return makeExtensionContext(request, this.host, this.session, this.stderr);
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
  ExtensionHandler,
  ExtensionOptions,
  ExtensionSession,
  ExtensionToolContext,
  ExtensionToolHandler,
  ExtensionToolOptions,
} from "./extension-contract.js";
