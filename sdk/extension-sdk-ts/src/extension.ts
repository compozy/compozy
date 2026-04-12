import { HostAPI, type HostCaller } from "./host_api.js";
import {
  registerMutableHook,
  registerObserverHook,
  requestContext,
  type EventHandler,
  type HealthCheckHandler,
  type HookHandlerMatrix,
  type RawHookHandler,
  type ShutdownHandler,
} from "./handlers.js";
import {
  EOFError,
  RPCError,
  type Message,
  type MessageID,
  type Transport,
  StdIOTransport,
  newInternalError,
  newInvalidParamsError,
  newMethodNotFoundError,
} from "./transport.js";
import {
  CAPABILITIES,
  HOOKS,
  PROTOCOL_VERSION,
  SDK_NAME,
  SDK_VERSION,
  type Capability,
  type Event,
  type EventKind,
  type EventSubscribeRequest,
  type ExecuteHookRequest,
  type ExecuteHookResponse,
  type FetchRequest,
  type HealthCheckRequest,
  type HealthCheckResponse,
  type HookName,
  type InitializeRequest,
  type InitializeResponse,
  type InitializeResponseInfo,
  type JsonValue,
  type OnEventRequest,
  type ResolveIssuesRequest,
  type ReviewItem,
  type ReviewProviderContext,
  type ShutdownRequest,
  type ShutdownResponse,
  type Supports,
} from "./types.js";

type PendingCall = {
  reject: (reason?: unknown) => void;
  resolve: (value: Message) => void;
};

type EventRegistration = {
  kinds: Set<EventKind>;
  handler: EventHandler;
};

export interface ReviewProviderHandler {
  fetchReviews(
    context: ReviewProviderContext,
    request: FetchRequest
  ): Promise<ReviewItem[]> | ReviewItem[];
  resolveIssues(
    context: ReviewProviderContext,
    request: ResolveIssuesRequest
  ): Promise<void> | void;
}

/**
 * SDK runtime for Compozy executable extensions.
 *
 * Manages protocol negotiation, capability exchange, hook dispatch, event
 * delivery, health checks, and graceful shutdown over JSON-RPC 2.0 on
 * stdin/stdout.
 */
export class Extension implements HostCaller {
  /** Extension name reported during initialize. */
  readonly name: string;
  /** Extension version reported during initialize. */
  readonly version: string;
  /** Host API client bound to the current extension session. */
  readonly host: HostAPI;

  private sdkVersion: string;
  private transport?: Transport;
  private started = false;
  private initialized = false;
  private draining = false;
  private initializeRequestValue?: InitializeRequest;
  private initializeResponseValue?: InitializeResponse;
  private readonly acceptedCapabilities = new Set<Capability>();
  private readonly declaredCapabilities = new Set<Capability>();
  private readonly hooks = new Map<HookName, RawHookHandler>();
  private readonly reviewProviders = new Map<string, ReviewProviderHandler>();
  private readonly eventHandlers: EventRegistration[] = [];
  private healthHandler?: HealthCheckHandler;
  private shutdownHandler?: ShutdownHandler;
  private requestID = 0;
  private readonly pending = new Map<string, PendingCall>();
  private finishResolve?: (error?: unknown) => void;
  private finishPromise?: Promise<unknown>;
  private finishError?: unknown;

  /** Constructs a new extension runtime with the provided identity. */
  constructor(name: string, version: string) {
    this.name = name.trim();
    this.version = version.trim();
    this.sdkVersion = SDK_VERSION;
    this.host = new HostAPI(this);
  }

  /** Declares the capabilities this extension requires from the host. */
  withCapabilities(...capabilities: Capability[]): this {
    for (const capability of capabilities) {
      if (capability.trim() !== "") {
        this.declaredCapabilities.add(capability);
      }
    }
    return this;
  }

  /** Overrides the default stdio transport used by {@link start}. */
  withTransport(transport: Transport): this {
    this.transport = transport;
    return this;
  }

  /** Overrides the sdk_version reported during initialize. */
  withSDKVersion(version: string): this {
    this.sdkVersion = version.trim();
    return this;
  }

  /** Returns the last initialize request processed by the extension, or undefined before initialize. */
  initializeRequest(): InitializeRequest | undefined {
    return this.initializeRequestValue;
  }

  /** Returns the last initialize response, or undefined before initialize. */
  initializeResponse(): InitializeResponse | undefined {
    return this.initializeResponseValue;
  }

  /** Returns the sorted list of capabilities negotiated during initialize. */
  acceptedCapabilitiesList(): Capability[] {
    return [...this.acceptedCapabilities].sort();
  }

  /** Registers a raw hook handler for one hook event. */
  handle(hook: HookName, handler: RawHookHandler): this {
    this.hooks.set(hook, handler);
    return this;
  }

  /** Registers a forwarded event handler with an optional kind filter. When no kinds are specified, the handler receives all events. */
  onEvent(handler: EventHandler, ...kinds: EventKind[]): this {
    const normalizedKinds = new Set(kinds.map(kind => kind.trim()).filter(Boolean));
    this.eventHandlers.push({ kinds: normalizedKinds, handler });
    return this;
  }

  /** Overrides the default healthy response returned by health checks. */
  onHealthCheck(handler: HealthCheckHandler): this {
    this.healthHandler = handler;
    return this;
  }

  /** Registers a callback invoked during graceful shutdown. */
  onShutdown(handler: ShutdownHandler): this {
    this.shutdownHandler = handler;
    return this;
  }

  /** Registers one executable review provider handler by name. */
  registerReviewProvider(name: string, handler: ReviewProviderHandler): this {
    const normalized = name.trim();
    if (normalized !== "") {
      this.reviewProviders.set(normalized, handler);
      this.declaredCapabilities.add(CAPABILITIES.providersRegister);
    }
    return this;
  }

  /** Serves the extension over the configured transport until shutdown or transport termination. Blocks until the extension session ends. */
  async start(signal?: AbortSignal): Promise<void> {
    if (this.name === "") {
      throw new Error("start extension: name is required");
    }
    if (this.version === "") {
      throw new Error("start extension: version is required");
    }
    if (this.started) {
      throw this.finishError ?? new Error("start extension: already started");
    }

    this.started = true;
    this.finishPromise = new Promise(resolve => {
      this.finishResolve = resolve;
    });
    this.transport ??= new StdIOTransport();

    const abortHandler = async () => {
      await this.transport?.close();
    };

    if (signal !== undefined) {
      signal.addEventListener("abort", abortHandler, { once: true });
    }

    try {
      await this.readLoop();
      const maybeError = await this.finishPromise;
      if (maybeError !== undefined) {
        throw maybeError;
      }
    } catch (error) {
      this.finish(error);
      if (signal?.aborted) {
        throw signal.reason ?? error;
      }
      throw error;
    } finally {
      if (signal !== undefined) {
        signal.removeEventListener("abort", abortHandler);
      }
      await this.transport?.close();
    }
  }

  /** Issues one Host API JSON-RPC call to the host and awaits the typed response. */
  async call<T>(method: string, params?: unknown): Promise<T> {
    if (!this.initialized) {
      throw newNotInitializedError();
    }
    if (this.draining) {
      throw newShutdownInProgressError();
    }
    this.ensureCapabilityForHostMethod(method);

    const id = String(++this.requestID);
    const response = await new Promise<Message>((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      void this.transport
        ?.writeMessage({
          id,
          method,
          params,
        })
        .catch(error => {
          this.pending.delete(id);
          reject(error);
        });
    });

    if (response.error !== undefined) {
      throw RPCError.fromShape(response.error);
    }
    return response.result as T;
  }

  /** Registers the {@link HOOKS.planPreDiscover | plan.pre_discover} handler. */
  onPlanPreDiscover(handler: HookHandlerMatrix["plan.pre_discover"]): this {
    registerMutableHook(this, HOOKS.planPreDiscover, handler);
    return this;
  }

  /** Registers the {@link HOOKS.planPostDiscover | plan.post_discover} handler. */
  onPlanPostDiscover(handler: HookHandlerMatrix["plan.post_discover"]): this {
    registerMutableHook(this, HOOKS.planPostDiscover, handler);
    return this;
  }

  /** Registers the {@link HOOKS.planPreGroup | plan.pre_group} handler. */
  onPlanPreGroup(handler: HookHandlerMatrix["plan.pre_group"]): this {
    registerMutableHook(this, HOOKS.planPreGroup, handler);
    return this;
  }

  /** Registers the {@link HOOKS.planPostGroup | plan.post_group} handler. */
  onPlanPostGroup(handler: HookHandlerMatrix["plan.post_group"]): this {
    registerMutableHook(this, HOOKS.planPostGroup, handler);
    return this;
  }

  /** Registers the {@link HOOKS.planPrePrepareJobs | plan.pre_prepare_jobs} handler. */
  onPlanPrePrepareJobs(handler: HookHandlerMatrix["plan.pre_prepare_jobs"]): this {
    registerMutableHook(this, HOOKS.planPrePrepareJobs, handler);
    return this;
  }

  /** Registers the {@link HOOKS.planPostPrepareJobs | plan.post_prepare_jobs} handler. */
  onPlanPostPrepareJobs(handler: HookHandlerMatrix["plan.post_prepare_jobs"]): this {
    registerMutableHook(this, HOOKS.planPostPrepareJobs, handler);
    return this;
  }

  /** Registers the {@link HOOKS.promptPreBuild | prompt.pre_build} handler. */
  onPromptPreBuild(handler: HookHandlerMatrix["prompt.pre_build"]): this {
    registerMutableHook(this, HOOKS.promptPreBuild, handler);
    return this;
  }

  /** Registers the {@link HOOKS.promptPostBuild | prompt.post_build} handler. */
  onPromptPostBuild(handler: HookHandlerMatrix["prompt.post_build"]): this {
    registerMutableHook(this, HOOKS.promptPostBuild, handler);
    return this;
  }

  /** Registers the {@link HOOKS.promptPreSystem | prompt.pre_system} handler. */
  onPromptPreSystem(handler: HookHandlerMatrix["prompt.pre_system"]): this {
    registerMutableHook(this, HOOKS.promptPreSystem, handler);
    return this;
  }

  /** Registers the {@link HOOKS.agentPreSessionCreate | agent.pre_session_create} handler. */
  onAgentPreSessionCreate(handler: HookHandlerMatrix["agent.pre_session_create"]): this {
    registerMutableHook(this, HOOKS.agentPreSessionCreate, handler);
    return this;
  }

  /** Registers the {@link HOOKS.agentPostSessionCreate | agent.post_session_create} handler. */
  onAgentPostSessionCreate(handler: HookHandlerMatrix["agent.post_session_create"]): this {
    registerObserverHook(this, HOOKS.agentPostSessionCreate, handler);
    return this;
  }

  /** Registers the {@link HOOKS.agentPreSessionResume | agent.pre_session_resume} handler. */
  onAgentPreSessionResume(handler: HookHandlerMatrix["agent.pre_session_resume"]): this {
    registerMutableHook(this, HOOKS.agentPreSessionResume, handler);
    return this;
  }

  /** Registers the {@link HOOKS.agentOnSessionUpdate | agent.on_session_update} handler. */
  onAgentOnSessionUpdate(handler: HookHandlerMatrix["agent.on_session_update"]): this {
    registerObserverHook(this, HOOKS.agentOnSessionUpdate, handler);
    return this;
  }

  /** Registers the {@link HOOKS.agentPostSessionEnd | agent.post_session_end} handler. */
  onAgentPostSessionEnd(handler: HookHandlerMatrix["agent.post_session_end"]): this {
    registerObserverHook(this, HOOKS.agentPostSessionEnd, handler);
    return this;
  }

  /** Registers the {@link HOOKS.jobPreExecute | job.pre_execute} handler. */
  onJobPreExecute(handler: HookHandlerMatrix["job.pre_execute"]): this {
    registerMutableHook(this, HOOKS.jobPreExecute, handler);
    return this;
  }

  /** Registers the {@link HOOKS.jobPostExecute | job.post_execute} handler. */
  onJobPostExecute(handler: HookHandlerMatrix["job.post_execute"]): this {
    registerObserverHook(this, HOOKS.jobPostExecute, handler);
    return this;
  }

  /** Registers the {@link HOOKS.jobPreRetry | job.pre_retry} handler. */
  onJobPreRetry(handler: HookHandlerMatrix["job.pre_retry"]): this {
    registerMutableHook(this, HOOKS.jobPreRetry, handler);
    return this;
  }

  /** Registers the {@link HOOKS.runPreStart | run.pre_start} handler. */
  onRunPreStart(handler: HookHandlerMatrix["run.pre_start"]): this {
    registerMutableHook(this, HOOKS.runPreStart, handler);
    return this;
  }

  /** Registers the {@link HOOKS.runPostStart | run.post_start} handler. */
  onRunPostStart(handler: HookHandlerMatrix["run.post_start"]): this {
    registerObserverHook(this, HOOKS.runPostStart, handler);
    return this;
  }

  /** Registers the {@link HOOKS.runPreShutdown | run.pre_shutdown} handler. */
  onRunPreShutdown(handler: HookHandlerMatrix["run.pre_shutdown"]): this {
    registerObserverHook(this, HOOKS.runPreShutdown, handler);
    return this;
  }

  /** Registers the {@link HOOKS.runPostShutdown | run.post_shutdown} handler. */
  onRunPostShutdown(handler: HookHandlerMatrix["run.post_shutdown"]): this {
    registerObserverHook(this, HOOKS.runPostShutdown, handler);
    return this;
  }

  /** Registers the {@link HOOKS.reviewPreFetch | review.pre_fetch} handler. */
  onReviewPreFetch(handler: HookHandlerMatrix["review.pre_fetch"]): this {
    registerMutableHook(this, HOOKS.reviewPreFetch, handler);
    return this;
  }

  /** Registers the {@link HOOKS.reviewPostFetch | review.post_fetch} handler. */
  onReviewPostFetch(handler: HookHandlerMatrix["review.post_fetch"]): this {
    registerMutableHook(this, HOOKS.reviewPostFetch, handler);
    return this;
  }

  /** Registers the {@link HOOKS.reviewPreBatch | review.pre_batch} handler. */
  onReviewPreBatch(handler: HookHandlerMatrix["review.pre_batch"]): this {
    registerMutableHook(this, HOOKS.reviewPreBatch, handler);
    return this;
  }

  /** Registers the {@link HOOKS.reviewPostFix | review.post_fix} handler. */
  onReviewPostFix(handler: HookHandlerMatrix["review.post_fix"]): this {
    registerObserverHook(this, HOOKS.reviewPostFix, handler);
    return this;
  }

  /** Registers the {@link HOOKS.reviewPreResolve | review.pre_resolve} handler. */
  onReviewPreResolve(handler: HookHandlerMatrix["review.pre_resolve"]): this {
    registerMutableHook(this, HOOKS.reviewPreResolve, handler);
    return this;
  }

  /** Registers the {@link HOOKS.artifactPreWrite | artifact.pre_write} handler. */
  onArtifactPreWrite(handler: HookHandlerMatrix["artifact.pre_write"]): this {
    registerMutableHook(this, HOOKS.artifactPreWrite, handler);
    return this;
  }

  /** Registers the {@link HOOKS.artifactPostWrite | artifact.post_write} handler. */
  onArtifactPostWrite(handler: HookHandlerMatrix["artifact.post_write"]): this {
    registerObserverHook(this, HOOKS.artifactPostWrite, handler);
    return this;
  }

  private async readLoop(): Promise<void> {
    while (true) {
      let message: Message;
      try {
        message = await this.transport!.readMessage();
      } catch (error) {
        if (error instanceof EOFError && this.draining) {
          this.finish(undefined);
          return;
        }
        throw error;
      }

      if (message.id === undefined) {
        continue;
      }

      if ((message.method ?? "").trim() === "") {
        this.resolvePending(message);
        continue;
      }

      if (!this.initialized) {
        if (message.method !== "initialize") {
          await this.writeError(message.id, newNotInitializedError());
          continue;
        }
        await this.handleInitialize(message);
        continue;
      }

      if (message.method === "shutdown") {
        await this.handleShutdown(message);
        this.finish(undefined);
        return;
      }

      void this.handleRequest(message).catch(error => {
        this.finish(error);
      });
    }
  }

  private async handleInitialize(message: Message): Promise<void> {
    const request = (message.params ?? {}) as InitializeRequest;
    let response: InitializeResponse;
    try {
      response = this.buildInitializeResponse(request);
    } catch (error) {
      const requestError = toRPCError(error);
      await this.writeError(message.id!, requestError);
      throw requestError;
    }
    this.initializeRequestValue = request;
    this.initializeResponseValue = response;
    this.acceptedCapabilities.clear();
    for (const capability of response.accepted_capabilities ?? []) {
      this.acceptedCapabilities.add(capability);
    }
    this.initialized = true;

    await this.writeResult(message.id!, response);
    void this.subscribeFilteredEvents().catch(error => {
      this.finish(error);
    });
  }

  private buildInitializeResponse(request: InitializeRequest): InitializeResponse {
    const selectedVersion = negotiateProtocolVersion(request);
    if (selectedVersion === undefined) {
      throw newInvalidParamsError({
        reason: "unsupported_protocol_version",
        requested: request.protocol_version,
        supported_protocol_versions: [PROTOCOL_VERSION],
      });
    }

    const required = this.requiredCapabilities();
    const granted = new Set(request.granted_capabilities ?? []);
    const missing = required.filter(capability => !granted.has(capability));
    if (missing.length > 0) {
      throw newCapabilityDeniedError("initialize", missing, request.granted_capabilities ?? []);
    }

    const supports: Supports = {
      health_check: true,
      on_event: true,
    };

    return {
      protocol_version: selectedVersion,
      extension_info: {
        name: this.name,
        version: this.version,
        sdk_name: SDK_NAME,
        sdk_version: this.sdkVersion || SDK_VERSION,
      } satisfies InitializeResponseInfo,
      accepted_capabilities: required,
      supported_hook_events: [...this.hooks.keys()].sort(),
      registered_review_providers: [...this.reviewProviders.keys()].sort(),
      supports,
    };
  }

  private async handleShutdown(message: Message): Promise<void> {
    this.draining = true;
    const request = (message.params ?? {}) as ShutdownRequest;
    if (this.shutdownHandler !== undefined) {
      await this.shutdownHandler(request);
    }
    await this.writeResult(message.id!, { acknowledged: true } satisfies ShutdownResponse);
  }

  private async handleRequest(message: Message): Promise<void> {
    if (this.draining) {
      await this.writeError(message.id!, newShutdownInProgressError());
      return;
    }

    try {
      switch (message.method) {
        case "execute_hook":
          await this.handleExecuteHook(message);
          return;
        case "on_event":
          await this.handleOnEvent(message);
          return;
        case "health_check":
          await this.handleHealthCheck(message);
          return;
        case "fetch_reviews":
          await this.handleFetchReviews(message);
          return;
        case "resolve_issues":
          await this.handleResolveIssues(message);
          return;
        default:
          await this.writeError(message.id!, newMethodNotFoundError(message.method ?? ""));
      }
    } catch (error) {
      await this.writeError(message.id!, toRPCError(error));
    }
  }

  private async handleExecuteHook(message: Message): Promise<void> {
    const request = message.params as ExecuteHookRequest;
    const hook = request.hook.event;
    const handler = this.hooks.get(hook);
    if (handler === undefined) {
      await this.writeError(
        message.id!,
        newInvalidParamsError({ reason: "unsupported_hook_event", hook })
      );
      return;
    }

    this.ensureCapabilityForHook(hook);

    try {
      const patch = await handler(requestContext(request, this.host), request.payload);
      const response: ExecuteHookResponse =
        patch === undefined ? {} : { patch: patch as JsonValue | undefined };
      await this.writeResult(message.id!, response);
    } catch (error) {
      await this.writeError(message.id!, toRPCError(error));
    }
  }

  private async handleOnEvent(message: Message): Promise<void> {
    this.ensureCapability(CAPABILITIES.eventsRead, "on_event");
    const request = message.params as OnEventRequest;
    for (const registration of this.eventHandlers) {
      if (registration.kinds.size > 0 && !registration.kinds.has(request.event.kind)) {
        continue;
      }
      await registration.handler(request.event as Event);
    }
    await this.writeResult(message.id!, {});
  }

  private async handleHealthCheck(message: Message): Promise<void> {
    const request = (message.params ?? {}) as HealthCheckRequest;
    let response: HealthCheckResponse = {
      healthy: true,
      details: {
        active_requests: this.pending.size,
        queue_depth: 0,
      },
    };

    if (this.healthHandler !== undefined) {
      response = await this.healthHandler(request);
      response.details ??= {};
      response.details.active_requests ??= this.pending.size;
      response.details.queue_depth ??= 0;
    }

    await this.writeResult(message.id!, response);
  }

  private async handleFetchReviews(message: Message): Promise<void> {
    this.ensureCapability(CAPABILITIES.providersRegister, "fetch_reviews");

    const request = (message.params ?? {}) as { provider?: string } & FetchRequest;
    const providerName = (request.provider ?? "").trim();
    const handler = this.reviewProviders.get(providerName);
    if (handler === undefined) {
      await this.writeError(
        message.id!,
        newInvalidParamsError({ reason: "unsupported_review_provider", provider: providerName })
      );
      return;
    }

    const items = await handler.fetchReviews({ provider: providerName, host: this.host }, {
      pr: request.pr,
      include_nitpicks: request.include_nitpicks,
    } satisfies FetchRequest);
    await this.writeResult(message.id!, items);
  }

  private async handleResolveIssues(message: Message): Promise<void> {
    this.ensureCapability(CAPABILITIES.providersRegister, "resolve_issues");

    const request = (message.params ?? {}) as { provider?: string } & ResolveIssuesRequest;
    const providerName = (request.provider ?? "").trim();
    const handler = this.reviewProviders.get(providerName);
    if (handler === undefined) {
      await this.writeError(
        message.id!,
        newInvalidParamsError({ reason: "unsupported_review_provider", provider: providerName })
      );
      return;
    }

    await handler.resolveIssues({ provider: providerName, host: this.host }, {
      pr: request.pr,
      issues: request.issues,
    } satisfies ResolveIssuesRequest);
    await this.writeResult(message.id!, {});
  }

  private resolvePending(message: Message): void {
    const pending = this.pending.get(String(message.id));
    if (pending === undefined) {
      return;
    }
    this.pending.delete(String(message.id));
    pending.resolve(message);
  }

  private finish(error?: unknown): void {
    if (this.finishResolve === undefined) {
      return;
    }
    this.finishError ??= error;
    for (const [id, pending] of this.pending) {
      this.pending.delete(id);
      pending.reject(error ?? new EOFError());
    }
    const resolve = this.finishResolve;
    this.finishResolve = undefined;
    resolve(this.finishError);
  }

  private requiredCapabilities(): Capability[] {
    const required = new Set<Capability>(this.declaredCapabilities);
    for (const hook of this.hooks.keys()) {
      const capability = capabilityForHook(hook);
      if (capability !== undefined) {
        required.add(capability);
      }
    }
    if (this.eventHandlers.length > 0) {
      required.add(CAPABILITIES.eventsRead);
    }
    return [...required].sort();
  }

  private ensureCapabilityForHook(hook: HookName): void {
    const capability = capabilityForHook(hook);
    if (capability !== undefined) {
      this.ensureCapability(capability, hook);
    }
  }

  private ensureCapabilityForHostMethod(method: string): void {
    const capability = HOST_METHOD_CAPABILITIES[method];
    if (capability === undefined && method !== "host.prompts.render") {
      throw newMethodNotFoundError(method);
    }
    if (capability !== undefined) {
      this.ensureCapability(capability, method);
    }
  }

  private ensureCapability(capability: Capability, target: string): void {
    if (this.acceptedCapabilities.has(capability)) {
      return;
    }
    throw newCapabilityDeniedError(target, [capability], this.acceptedCapabilitiesList());
  }

  private async subscribeFilteredEvents(): Promise<void> {
    if (
      !this.acceptedCapabilities.has(CAPABILITIES.eventsRead) ||
      this.eventHandlers.length === 0
    ) {
      return;
    }

    const kinds = new Set<EventKind>();
    for (const registration of this.eventHandlers) {
      if (registration.kinds.size === 0) {
        return;
      }
      for (const kind of registration.kinds) {
        kinds.add(kind);
      }
    }

    await this.host.events.subscribe({
      kinds: [...kinds].sort(),
    } satisfies EventSubscribeRequest);
  }

  private async writeResult(id: MessageID, result: unknown): Promise<void> {
    await this.transport!.writeMessage({ id, result });
  }

  private async writeError(id: MessageID, error: RPCError): Promise<void> {
    await this.transport!.writeMessage({ id, error: error.toShape() });
  }
}

const HOST_METHOD_CAPABILITIES: Record<string, Capability | undefined> = {
  "host.events.subscribe": CAPABILITIES.eventsRead,
  "host.events.publish": CAPABILITIES.eventsPublish,
  "host.tasks.list": CAPABILITIES.tasksRead,
  "host.tasks.get": CAPABILITIES.tasksRead,
  "host.tasks.create": CAPABILITIES.tasksCreate,
  "host.runs.start": CAPABILITIES.runsStart,
  "host.artifacts.read": CAPABILITIES.artifactsRead,
  "host.artifacts.write": CAPABILITIES.artifactsWrite,
  "host.prompts.render": undefined,
  "host.memory.read": CAPABILITIES.memoryRead,
  "host.memory.write": CAPABILITIES.memoryWrite,
};

function negotiateProtocolVersion(request: InitializeRequest): string | undefined {
  if ((request.supported_protocol_versions ?? []).includes(PROTOCOL_VERSION)) {
    return PROTOCOL_VERSION;
  }
  if (request.protocol_version === PROTOCOL_VERSION) {
    return PROTOCOL_VERSION;
  }
  return undefined;
}

function capabilityForHook(hook: HookName): Capability | undefined {
  if (hook.startsWith("plan.")) {
    return CAPABILITIES.planMutate;
  }
  if (hook.startsWith("prompt.")) {
    return CAPABILITIES.promptMutate;
  }
  if (hook.startsWith("agent.")) {
    return CAPABILITIES.agentMutate;
  }
  if (hook.startsWith("job.")) {
    return CAPABILITIES.jobMutate;
  }
  if (hook.startsWith("run.")) {
    return CAPABILITIES.runMutate;
  }
  if (hook.startsWith("review.")) {
    return CAPABILITIES.reviewMutate;
  }
  if (hook.startsWith("artifact.")) {
    return CAPABILITIES.artifactsWrite;
  }
  return undefined;
}

function newCapabilityDeniedError(
  method: string,
  required: Capability[],
  granted: Capability[]
): RPCError {
  return new RPCError(-32001, "Capability denied", {
    method,
    required,
    granted,
  });
}

function newNotInitializedError(): RPCError {
  return new RPCError(-32003, "Not initialized", { reason: "not_initialized" });
}

function newShutdownInProgressError(): RPCError {
  return new RPCError(-32004, "Shutdown in progress", { reason: "shutdown_in_progress" });
}

function toRPCError(error: unknown): RPCError {
  if (error instanceof RPCError) {
    return error;
  }
  if (error instanceof Error) {
    return newInternalError({ error: error.message });
  }
  return newInternalError({ error: String(error) });
}
