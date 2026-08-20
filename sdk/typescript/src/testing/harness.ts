import path from "node:path";
import { pathToFileURL } from "node:url";

import { MethodNotFoundError } from "../errors.js";
import { Extension } from "../extension.js";
import type { ExtensionDescribeProcess } from "../extension-contract.js";
import type {
  ExtensionDefinition,
  HostAPIMethod,
  InitializeRequest,
  InitializeResponse,
  InitializeRuntime,
  ProtocolVersion,
} from "../types.js";
import { MockTransport, createMockTransportPair } from "./mock-transport.js";

export interface HarnessLoadOptions {
  sourceTier?: "bundled" | "user" | "workspace" | "marketplace";
  provides?: string[];
  grantedPermissions?: string[];
  grantedResourceKinds?: string[];
  grantedResourceScopes?: ("global" | "workspace")[];
  sessionNonce?: string;
  daemonRequests?: string[];
  extensionServices?: string[];
  compozyVersion?: string;
  runtime?: Partial<InitializeRuntime>;
}

export interface ExtensionFactoryOptions {
  transport?: MockTransport;
  stderr?: NodeJS.WritableStream;
}

type ExtensionFactory =
  | Extension
  | ((options: ExtensionFactoryOptions) => Extension | Promise<Extension>);

interface ExtensionModuleShape {
  createExtension?: (options: ExtensionFactoryOptions) => Extension | Promise<Extension>;
  extension?: Extension;
  default?: Extension | ((options: ExtensionFactoryOptions) => Extension | Promise<Extension>);
}

const DEFAULT_RUNTIME: InitializeRuntime = {
  health_check_interval_ms: 30_000,
  health_check_timeout_ms: 5_000,
  shutdown_timeout_ms: 10_000,
  default_hook_timeout_ms: 5_000,
  default_view_timeout_ms: 3_000,
};

const DAEMON_METHODS = new Set(["execute_hook", "health_check", "shutdown"]);

export class TestHarness {
  private readonly mockedHostHandlers = new Map<
    string,
    (params: unknown) => unknown | Promise<unknown>
  >();
  private readonly stderr: NodeJS.WritableStream;
  private hostTransport: MockTransport | undefined;
  private extensionTransport: MockTransport | undefined;
  private extension: Extension | undefined;
  private initializeRequest: InitializeRequest | undefined;
  private initializeResponse: InitializeResponse | undefined;
  private describeStdout = "";
  private describeExitCode: number | undefined;

  public constructor(options: { stderr?: NodeJS.WritableStream } = {}) {
    this.stderr = options.stderr ?? process.stderr;
  }

  public mockHostAPI<TResult = unknown>(
    method: string,
    handler: (params: unknown) => TResult | Promise<TResult>
  ): this {
    this.mockedHostHandlers.set(method.trim(), handler);
    this.bindMockedHostHandler(method.trim(), handler);
    return this;
  }

  public async loadExtension(
    target: string | ExtensionFactory,
    options: HarnessLoadOptions = {}
  ): Promise<Extension> {
    const pair = createMockTransportPair();
    this.hostTransport = pair.host;
    this.extensionTransport = pair.extension;

    for (const [method, handler] of this.mockedHostHandlers) {
      this.bindMockedHostHandler(method, handler);
    }

    const extension = await this.resolveExtension(target);
    extension.bindTransport(this.extensionTransport);
    this.extension = extension;

    const request = this.buildInitializeRequest(
      extension.definition,
      extension.getImplementedMethods(),
      options
    );
    this.initializeRequest = request;

    const startPromise = extension.start();
    this.initializeResponse = await this.hostTransport.call<InitializeResponse>(
      "initialize",
      request
    );
    await startPromise;

    return extension;
  }

  public async call<TResult = unknown>(method: string, params?: unknown): Promise<TResult> {
    if (!this.hostTransport) {
      throw new Error("extension is not loaded");
    }
    return await this.hostTransport.call<TResult>(method, params);
  }

  public getLastInitializeRequest(): InitializeRequest | undefined {
    return this.initializeRequest;
  }

  public getLastInitializeResponse(): InitializeResponse | undefined {
    return this.initializeResponse;
  }

  public getHostTransport(): MockTransport {
    if (!this.hostTransport) {
      throw new Error("host transport is not initialized");
    }
    return this.hostTransport;
  }

  public captureDescribeProcess(
    argv: readonly string[] = ["extension", "__describe"]
  ): ExtensionDescribeProcess {
    this.describeStdout = "";
    this.describeExitCode = undefined;
    return {
      argv,
      stdout: {
        write: ((chunk: string | Uint8Array) => {
          this.describeStdout += typeof chunk === "string" ? chunk : Buffer.from(chunk).toString();
          return true;
        }) as NodeJS.WritableStream["write"],
      },
      setExitCode: code => {
        this.describeExitCode = code;
      },
    };
  }

  public getDescribeResult(): { stdout: string; exitCode: number | undefined } {
    return { stdout: this.describeStdout, exitCode: this.describeExitCode };
  }

  private bindMockedHostHandler(
    method: string,
    handler: (params: unknown) => unknown | Promise<unknown>
  ): void {
    if (!this.hostTransport) {
      return;
    }
    this.hostTransport.handle(method, async params => await handler(params));
  }

  private async resolveExtension(target: string | ExtensionFactory): Promise<Extension> {
    if (target instanceof Extension) {
      return target;
    }

    if (typeof target === "function") {
      return await target({ transport: this.extensionTransport!, stderr: this.stderr });
    }

    const moduleURL = pathToFileURL(path.resolve(target)).href;
    const imported = (await import(moduleURL)) as ExtensionModuleShape;

    if (typeof imported.createExtension === "function") {
      return await imported.createExtension({
        transport: this.extensionTransport!,
        stderr: this.stderr,
      });
    }
    if (imported.extension instanceof Extension) {
      return imported.extension;
    }
    if (imported.default instanceof Extension) {
      return imported.default;
    }
    if (typeof imported.default === "function") {
      return await imported.default({ transport: this.extensionTransport!, stderr: this.stderr });
    }

    throw new MethodNotFoundError("createExtension");
  }

  private buildInitializeRequest(
    definition: ExtensionDefinition,
    implementedMethods: string[],
    options: HarnessLoadOptions
  ): InitializeRequest {
    const daemonRequests =
      options.daemonRequests ?? implementedMethods.filter(method => DAEMON_METHODS.has(method));
    const extensionServices =
      options.extensionServices ?? implementedMethods.filter(method => !DAEMON_METHODS.has(method));

    const requestedProvides = definition.capabilities?.provides ?? [];
    const requestedPermissions = definition.permissions?.requires ?? [];

    return {
      protocol_version: "1",
      supported_protocol_versions: ["1" satisfies ProtocolVersion],
      compozy_version: options.compozyVersion ?? "0.5.0",
      session_nonce: options.sessionNonce ?? "session-nonce-test",
      extension: {
        name: definition.name,
        version: definition.version,
        source_tier: options.sourceTier ?? "user",
      },
      capabilities: {
        provides: options.provides ?? [...requestedProvides],
        granted_permissions: (options.grantedPermissions ?? [
          ...requestedPermissions,
        ]) as HostAPIMethod[],
        granted_resource_kinds: options.grantedResourceKinds ?? [],
        granted_resource_scopes: options.grantedResourceScopes ?? [],
      },
      methods: {
        daemon_requests: daemonRequests,
        extension_services: extensionServices,
      },
      runtime: {
        ...DEFAULT_RUNTIME,
        ...options.runtime,
      },
    };
  }
}
