import { Extension } from "../extension.js";
import { RPCError, type Message } from "../transport.js";
import type {
  Capability,
  Event,
  ExecuteHookResponse,
  HookInfo,
  InitializeRequestIdentity,
  InitializeRuntime,
  InitializeRequest,
  InitializeResponse,
  OnEventRequest,
  ShutdownRequest,
  ShutdownResponse,
} from "../types.js";
import { PROTOCOL_VERSION } from "../types.js";
import { MockTransport, createMockTransportPair } from "./mock_transport.js";

export interface HarnessOptions {
  protocol_version?: string;
  supported_protocol_versions?: string[];
  compozy_version?: string;
  source?: "bundled" | "user" | "workspace";
  granted_capabilities?: Capability[];
  runtime?: InitializeRuntime;
}

export interface HostCall {
  method: string;
  params: unknown;
}

export type HostHandler = (params: unknown) => Promise<unknown> | unknown;

export class TestHarness {
  readonly extensionTransport: MockTransport;
  readonly hostTransport: MockTransport;

  private readonly options: Required<HarnessOptions>;
  private readonly handlers = new Map<string, HostHandler>();
  private readonly calls: HostCall[] = [];
  private readonly pending = new Map<
    string,
    { resolve: (message: Message) => void; reject: (error: unknown) => void }
  >();
  private requestID = 0;

  constructor(options: HarnessOptions = {}) {
    const [extensionTransport, hostTransport] = createMockTransportPair();
    this.extensionTransport = extensionTransport;
    this.hostTransport = hostTransport;
    this.options = normalizeOptions(options);
  }

  run(extension: Extension): Promise<void> {
    void this.hostLoop();
    return extension.withTransport(this.extensionTransport).start();
  }

  handleHostMethod(method: string, handler: HostHandler): void {
    this.handlers.set(method, handler);
  }

  hostCalls(): HostCall[] {
    return [...this.calls];
  }

  async initialize(identity: InitializeRequestIdentity): Promise<InitializeResponse> {
    return this.call("initialize", {
      protocol_version: this.options.protocol_version,
      supported_protocol_versions: this.options.supported_protocol_versions,
      compozy_version: this.options.compozy_version,
      extension: {
        ...identity,
        source: identity.source || this.options.source,
      },
      granted_capabilities: this.options.granted_capabilities,
      runtime: this.options.runtime,
    } satisfies InitializeRequest);
  }

  async dispatchHook(
    invocationID: string,
    hook: HookInfo,
    payload: unknown
  ): Promise<ExecuteHookResponse> {
    return this.call("execute_hook", {
      invocation_id: invocationID,
      hook,
      payload,
    });
  }

  async sendEvent(event: Event): Promise<void> {
    await this.call("on_event", { event } satisfies OnEventRequest);
  }

  async healthCheck(): Promise<unknown> {
    return this.call("health_check", {});
  }

  async shutdown(request: ShutdownRequest): Promise<ShutdownResponse> {
    return this.call("shutdown", request);
  }

  private async hostLoop(): Promise<void> {
    while (true) {
      let message: Message;
      try {
        message = await this.hostTransport.readMessage();
      } catch {
        return;
      }

      if (message.id === undefined) {
        continue;
      }

      if ((message.method ?? "").trim() === "") {
        const pending = this.pending.get(String(message.id));
        if (pending !== undefined) {
          this.pending.delete(String(message.id));
          pending.resolve(message);
        }
        continue;
      }

      const handler = this.handlers.get(message.method ?? "");
      this.calls.push({ method: message.method ?? "", params: message.params });

      if (handler === undefined) {
        await this.hostTransport.writeMessage({
          id: message.id,
          error: { code: -32601, message: "Method not found", data: { method: message.method } },
        });
        continue;
      }

      try {
        const result = await handler(message.params);
        await this.hostTransport.writeMessage({ id: message.id, result });
      } catch (error) {
        const requestError =
          error instanceof RPCError
            ? error
            : new RPCError(-32603, "Internal error", {
                error: error instanceof Error ? error.message : String(error),
              });
        await this.hostTransport.writeMessage({ id: message.id, error: requestError.toShape() });
      }
    }
  }

  private async call<T>(method: string, params: unknown): Promise<T> {
    const id = String(++this.requestID);
    const response = await new Promise<Message>((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      void this.hostTransport.writeMessage({ id, method, params }).catch(error => {
        this.pending.delete(id);
        reject(error);
      });
    });

    if (response.error !== undefined) {
      throw RPCError.fromShape(response.error);
    }
    return response.result as T;
  }
}

function normalizeOptions(options: HarnessOptions): Required<HarnessOptions> {
  return {
    protocol_version: options.protocol_version ?? PROTOCOL_VERSION,
    supported_protocol_versions: options.supported_protocol_versions ?? [PROTOCOL_VERSION],
    compozy_version: options.compozy_version ?? "dev",
    source: options.source ?? "workspace",
    granted_capabilities: options.granted_capabilities ?? [],
    runtime: {
      run_id: options.runtime?.run_id ?? "run-test",
      parent_run_id: options.runtime?.parent_run_id ?? "",
      workspace_root: options.runtime?.workspace_root ?? ".",
      invoking_command: options.runtime?.invoking_command ?? "start",
      shutdown_timeout_ms: options.runtime?.shutdown_timeout_ms ?? 1000,
      default_hook_timeout_ms: options.runtime?.default_hook_timeout_ms ?? 5000,
      health_check_interval_ms: options.runtime?.health_check_interval_ms ?? 0,
    },
  };
}
