import type { JSONValue } from "./base-types.js";
import type {
  HookEvent,
  HookExecutorKind,
  HookMatcher,
  HookMode,
  HookPatchByEvent,
  HookPayloadByEvent,
  HostAPIMethod,
  RiskClass,
  SourceKind,
  ToolID,
  ToolsetID,
  Visibility,
} from "./generated/contracts.js";

export * from "./base-types.js";
export * from "./generated/contracts.js";

export type ToolSource = SourceKind;

export type ExtensionSourceTier = "bundled" | "user" | "workspace" | "marketplace";

export interface HookExecutorConfig {
  kind?: HookExecutorKind;
  command?: string;
  args?: string[];
  env?: Record<string, string>;
}

export interface HookConfig {
  name: string;
  event: HookEvent;
  mode?: HookMode;
  required?: boolean;
  priority?: number;
  timeout?: string;
  matcher?: HookMatcher;
  command?: string;
  args?: string[];
  env?: Record<string, string>;
  executor?: HookExecutorConfig;
}

export interface MCPServerConfig {
  command: string;
  args?: string[];
  env?: Record<string, string>;
}

export interface ToolConfig {
  id?: ToolID;
  display_title?: string;
  description?: string;
  input_schema?: JSONValue;
  output_schema?: JSONValue;
  handler?: string;
  backend?: {
    kind?: "extension_host" | "mcp";
    handler?: string;
    server?: string;
    tool?: string;
  };
  visibility?: Visibility;
  risk?: RiskClass;
  read_only?: boolean;
  destructive?: boolean;
  open_world?: boolean;
  requires_interaction?: boolean;
  concurrency_safe?: boolean;
  max_result_bytes?: number;
  toolsets?: ToolsetID[];
  tags?: string[];
  search_hints?: string[];
  required_capabilities?: string[];
}

export interface ResourcesConfig {
  skills?: string[];
  agents?: string[];
  hooks?: HookConfig[];
  tools?: Record<string, ToolConfig>;
  mcp_servers?: Record<string, MCPServerConfig>;
}

export interface CapabilitiesConfig {
  provides?: string[];
}

export interface ActionsConfig {
  requires?: HostAPIMethod[];
}

export interface SubprocessConfig {
  command?: string;
  args?: string[];
  env?: Record<string, string>;
  health_check_interval?: string;
  shutdown_timeout?: string;
}

export interface SecurityConfig {
  capabilities?: string[];
}

export interface ExtensionManifest {
  name: string;
  version: string;
  description?: string;
  min_agh_version?: string;
  resources?: ResourcesConfig;
  capabilities?: CapabilitiesConfig;
  actions?: ActionsConfig;
  subprocess?: SubprocessConfig;
  security?: SecurityConfig;
}

export interface ExtensionDefinition extends Pick<
  ExtensionManifest,
  "name" | "version" | "description" | "min_agh_version" | "capabilities" | "actions" | "security"
> {
  supported_hook_events?: HookEvent[];
}

export interface HealthCheckResult {
  healthy: boolean;
  message?: string;
  details?: Record<string, JSONValue>;
}

export interface HookInvocation<TEvent extends HookEvent = HookEvent> {
  name: string;
  event: TEvent;
  mode: HookMode;
  required: boolean;
  timeout_ms: number;
  source: string;
  metadata?: Record<string, string>;
}

export interface ExecuteHookParams<TEvent extends HookEvent = HookEvent> {
  invocation_id: string;
  hook: HookInvocation<TEvent>;
  payload: HookPayloadByEvent[TEvent];
}

export type ExecuteHookResult<TEvent extends HookEvent = HookEvent> = HookPatchByEvent[TEvent];
