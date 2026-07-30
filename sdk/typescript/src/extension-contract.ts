import type { TransportLike } from "./transport.js";
import type {
  AcceptedCapabilities,
  ClarifyAnswer,
  ClarifyAskParams,
  ExtensionCommandSpec,
  ExtensionToolRuntimeDescriptor,
  ExtensionToolWorkspaceScope,
  InitializeRequest,
  InitializeResponse,
  InitializeRuntime,
  JSONRPCRequestEnvelope,
  JSONValue,
  RiskClass,
  ToolID,
  ToolResult,
} from "./types.js";
import { HostAPI } from "./host-api.js";

export const SDK_NAME = "@compozy/extension-sdk";

export const SDK_VERSION = "0.1.0";

export const SDK_MIN_COMPOZY_VERSION = "0.3.0-beta.1";

export const SUPPORTED_PROTOCOL_VERSIONS = ["1"];

export interface ExtensionDescribeProcess {
  argv: readonly string[];
  stdout: Pick<NodeJS.WritableStream, "write">;
  setExitCode: (code: number) => void;
}

export interface ExtensionOptions {
  transport?: TransportLike;
  stderr?: NodeJS.WritableStream;
  sdkVersion?: string;
  describeProcess?: ExtensionDescribeProcess;
}

export interface ExtensionSession {
  initializeRequest: InitializeRequest;
  initializeResponse: InitializeResponse;
  runtime: InitializeRuntime;
  acceptedCapabilities: AcceptedCapabilities;
}

export interface ExtensionContext {
  readonly request: JSONRPCRequestEnvelope;
  readonly requestId: string | number;
  readonly host: HostAPI;
  readonly session: ExtensionSession;
  log: (...values: unknown[]) => void;
}

export type ExtensionHandler<TParams = unknown, TResult = unknown> = (
  context: ExtensionContext,
  params: TParams
) => Promise<TResult> | TResult;

export interface ExtensionToolOptions {
  id?: ToolID;
  description?: string;
  friendlyVerb?: string;
  preview?: string;
  inputSchema: JSONValue;
  outputSchema?: JSONValue;
  readOnly?: boolean;
  risk?: RiskClass;
  requiresInteraction?: boolean;
  capabilities?: string[];
  sensitiveInputFields?: string[];
  command?: ExtensionCommandSpec;
}

export interface ExtensionToolContext<TInput = unknown> {
  readonly input: TInput;
  readonly context: ExtensionContext;
  readonly host: HostAPI;
  readonly session: ExtensionSession;
  readonly toolID: ToolID;
  readonly handler: string;
  readonly trustedWorkspace?: ExtensionToolWorkspaceScope;
  readonly invocationId?: string;
  askClarification: (question: Omit<ClarifyAskParams, "invocation_id">) => Promise<ClarifyAnswer>;
}

export type ExtensionToolHandler<TInput = unknown> = (
  request: ExtensionToolContext<TInput>
) => Promise<ToolResult | JSONValue | string | void> | ToolResult | JSONValue | string | void;

export type ReadyCallback = (host: HostAPI, session: ExtensionSession) => Promise<void> | void;

export interface RegisteredTool {
  descriptor: ExtensionToolRuntimeDescriptor;
  handler: ExtensionToolHandler;
  sensitiveInputFields: string[];
}
