import type { HostAPI } from "./host_api.js";

// ---------------------------------------------------------------------------
// Protocol constants
// ---------------------------------------------------------------------------

/** Extension subprocess protocol version the SDK speaks. */
export const PROTOCOL_VERSION = "1";

/** Name of this SDK package. */
export const SDK_NAME = "@compozy/extension-sdk";

/** Current SDK release version. */
export const SDK_VERSION = "0.1.10";

/** Maximum allowed JSON-RPC message size in bytes (10 MiB). */
export const MAX_MESSAGE_SIZE = 10 * 1024 * 1024;

// ---------------------------------------------------------------------------
// Capability constants and type
// ---------------------------------------------------------------------------

/** Supported capability grant values. */
export const CAPABILITIES = {
  eventsRead: "events.read",
  eventsPublish: "events.publish",
  promptMutate: "prompt.mutate",
  planMutate: "plan.mutate",
  agentMutate: "agent.mutate",
  jobMutate: "job.mutate",
  runMutate: "run.mutate",
  reviewMutate: "review.mutate",
  artifactsRead: "artifacts.read",
  artifactsWrite: "artifacts.write",
  tasksRead: "tasks.read",
  tasksCreate: "tasks.create",
  runsStart: "runs.start",
  memoryRead: "memory.read",
  memoryWrite: "memory.write",
  providersRegister: "providers.register",
  skillsShip: "skills.ship",
  subprocessSpawn: "subprocess.spawn",
  networkEgress: "network.egress",
} as const;

/** One extension capability grant. */
export type Capability = (typeof CAPABILITIES)[keyof typeof CAPABILITIES];

// ---------------------------------------------------------------------------
// Hook constants and type
// ---------------------------------------------------------------------------

/** Supported hook name values. */
export const HOOKS = {
  planPreDiscover: "plan.pre_discover",
  planPostDiscover: "plan.post_discover",
  planPreGroup: "plan.pre_group",
  planPostGroup: "plan.post_group",
  planPrePrepareJobs: "plan.pre_prepare_jobs",
  planPostPrepareJobs: "plan.post_prepare_jobs",
  promptPreBuild: "prompt.pre_build",
  promptPostBuild: "prompt.post_build",
  promptPreSystem: "prompt.pre_system",
  agentPreSessionCreate: "agent.pre_session_create",
  agentPostSessionCreate: "agent.post_session_create",
  agentPreSessionResume: "agent.pre_session_resume",
  agentOnSessionUpdate: "agent.on_session_update",
  agentPostSessionEnd: "agent.post_session_end",
  jobPreExecute: "job.pre_execute",
  jobPostExecute: "job.post_execute",
  jobPreRetry: "job.pre_retry",
  runPreStart: "run.pre_start",
  runPostStart: "run.post_start",
  runPreShutdown: "run.pre_shutdown",
  runPostShutdown: "run.post_shutdown",
  reviewPreFetch: "review.pre_fetch",
  reviewPostFetch: "review.post_fetch",
  reviewPreBatch: "review.pre_batch",
  reviewPostFix: "review.post_fix",
  reviewPreResolve: "review.pre_resolve",
  artifactPreWrite: "artifact.pre_write",
  artifactPostWrite: "artifact.post_write",
} as const;

/** One canonical extension hook event name. */
export type HookName = (typeof HOOKS)[keyof typeof HOOKS];

// ---------------------------------------------------------------------------
// Execution mode constants and type
// ---------------------------------------------------------------------------

/** Supported execution mode values. */
export const EXECUTION_MODES = {
  prReview: "pr-review",
  prdTasks: "prd-tasks",
  exec: "exec",
} as const;

/** Target Compozy execution mode. */
export type ExecutionMode = (typeof EXECUTION_MODES)[keyof typeof EXECUTION_MODES];

// ---------------------------------------------------------------------------
// Output format constants and type
// ---------------------------------------------------------------------------

/** Supported output format values. */
export const OUTPUT_FORMATS = {
  text: "text",
  json: "json",
  rawJson: "raw-json",
} as const;

/** Requested output rendering mode. */
export type OutputFormat = (typeof OUTPUT_FORMATS)[keyof typeof OUTPUT_FORMATS];

// ---------------------------------------------------------------------------
// Memory write mode constants and type
// ---------------------------------------------------------------------------

/** Supported memory write mode values. */
export const MEMORY_WRITE_MODES = {
  replace: "replace",
  append: "append",
} as const;

/** How a memory document write is applied. */
export type MemoryWriteMode = (typeof MEMORY_WRITE_MODES)[keyof typeof MEMORY_WRITE_MODES];

// ---------------------------------------------------------------------------
// Session status constants and type
// ---------------------------------------------------------------------------

/** Supported session status values. */
export const SESSION_STATUSES = {
  running: "running",
  completed: "completed",
  failed: "failed",
} as const;

/** Lifecycle state of a streamed session update. */
export type SessionStatus = (typeof SESSION_STATUSES)[keyof typeof SESSION_STATUSES];

// ---------------------------------------------------------------------------
// JSON utility types
// ---------------------------------------------------------------------------

/** A JSON scalar value. */
export type JsonPrimitive = boolean | number | string | null;

/** A JSON object with string keys. */
export interface JsonObject {
  [key: string]: JsonValue | undefined;
}

/** Any JSON-representable value. */
export type JsonValue = JsonPrimitive | JsonValue[] | JsonObject;

// ---------------------------------------------------------------------------
// Event and session domain types
// ---------------------------------------------------------------------------

/** Identifies one forwarded bus event kind. */
export type EventKind = string;

/** One versioned event envelope forwarded to extensions. */
export interface Event {
  schema_version: string;
  run_id: string;
  seq: number;
  ts: string;
  kind: EventKind;
  payload?: JsonValue;
}

/** Token consumption summary embedded in session updates. */
export interface Usage {
  input_tokens?: number;
  output_tokens?: number;
  total_tokens?: number;
  cache_reads?: number;
  cache_writes?: number;
}

/** One typed content payload in its canonical JSON form. */
export interface ContentBlock {
  type: string;
  [key: string]: JsonValue | undefined;
}

/** One plan entry inside a session update. */
export interface SessionPlanEntry {
  content: string;
  priority: string;
  status: string;
}

/** One slash-command style action available in a session. */
export interface SessionAvailableCommand {
  name: string;
  description?: string;
  argument_hint?: string;
}

/** Public view of one streamed ACP session update. */
export interface SessionUpdate {
  kind?: string;
  tool_call_id?: string;
  tool_call_state?: string;
  blocks?: ContentBlock[];
  thought_blocks?: ContentBlock[];
  plan_entries?: SessionPlanEntry[];
  available_commands?: SessionAvailableCommand[];
  current_mode_id?: string;
  usage?: Usage;
  status: SessionStatus;
}

// ---------------------------------------------------------------------------
// Hook context types
// ---------------------------------------------------------------------------

/** Describes the current hook invocation metadata. */
export interface HookInfo {
  name: string;
  event: HookName;
  mutable: boolean;
  required: boolean;
  priority: number;
  timeout_ms: number;
}

/**
 * Carries request metadata and Host API access for one handler invocation.
 */
export interface HookContext {
  invocation_id: string;
  hook: HookInfo;
  host: HostAPI;
}

// ---------------------------------------------------------------------------
// Initialize request/response types
// ---------------------------------------------------------------------------

/** Identifies the extension instance the host loaded. */
export interface InitializeRequestIdentity {
  name: string;
  version: string;
  source: "bundled" | "user" | "workspace";
}

/** Describes the run-scoped runtime contract. */
export interface InitializeRuntime {
  run_id: string;
  parent_run_id?: string;
  workspace_root: string;
  invoking_command: string;
  shutdown_timeout_ms: number;
  default_hook_timeout_ms: number;
  health_check_interval_ms?: number;
}

/** Host-originated initialize request. */
export interface InitializeRequest {
  protocol_version: string;
  supported_protocol_versions: string[];
  compozy_version: string;
  extension: InitializeRequestIdentity;
  granted_capabilities?: Capability[];
  runtime: InitializeRuntime;
}

/** Describes the running SDK identity. */
export interface InitializeResponseInfo {
  name?: string;
  version?: string;
  sdk_name?: string;
  sdk_version?: string;
}

/** Reports which optional base methods the extension serves. */
export interface Supports {
  health_check: boolean;
  on_event: boolean;
}

/** Extension's initialize acknowledgement. */
export interface InitializeResponse {
  protocol_version: string;
  extension_info: InitializeResponseInfo;
  accepted_capabilities?: Capability[];
  supported_hook_events?: HookName[];
  registered_review_providers?: string[];
  supports: Supports;
}

// ---------------------------------------------------------------------------
// Protocol RPC types
// ---------------------------------------------------------------------------

/** One host-originated hook invocation envelope. */
export interface ExecuteHookRequest {
  invocation_id: string;
  hook: HookInfo;
  payload: JsonValue;
}

/** Hook response envelope returned to the host. */
export interface ExecuteHookResponse {
  patch?: JsonValue;
}

/** One host-originated event delivery request. */
export interface OnEventRequest {
  event: Event;
}

/** Host-originated liveness probe payload. */
export interface HealthCheckRequest {}

/** Extension health status. */
export interface HealthCheckResponse {
  healthy: boolean;
  message?: string;
  details?: Record<string, JsonValue>;
}

/** Host-originated graceful shutdown request. */
export interface ShutdownRequest {
  reason: string;
  deadline_ms: number;
}

/** Acknowledges a graceful shutdown request. */
export interface ShutdownResponse {
  acknowledged: boolean;
}

/** Carries provider-local metadata and Host API access for review provider handlers. */
export interface ReviewProviderContext {
  provider: string;
  host: HostAPI;
}

/** Mirrors the host-side review fetch request. */
export interface FetchRequest {
  pr: string;
  include_nitpicks?: boolean;
}

/** Mirrors the host-side normalized review item shape. */
export interface ReviewItem {
  title: string;
  file: string;
  line?: number;
  severity?: string;
  author?: string;
  body: string;
  provider_ref?: string;
  review_hash?: string;
  source_review_id?: string;
  source_review_submitted_at?: string;
}

/** Mirrors the host-side resolved issue payload. */
export interface ResolvedIssue {
  file_path: string;
  provider_ref?: string;
}

/** Mirrors the host-side resolve request. */
export interface ResolveIssuesRequest {
  pr: string;
  issues?: ResolvedIssue[];
}

// ---------------------------------------------------------------------------
// Domain model types
// ---------------------------------------------------------------------------

/** Mirrors the issue/task entry shape used in planning hooks. */
export interface IssueEntry {
  name?: string;
  abs_path?: string;
  content?: string;
  code_file?: string;
}

/** Describes the current workflow memory documents. */
export interface WorkflowMemoryContext {
  directory?: string;
  workflow_path?: string;
  task_path?: string;
  workflow_needs_compaction?: boolean;
  task_needs_compaction?: boolean;
}

/** Mirrors the prompt build input snapshot exposed to prompt hooks. */
export interface BatchParams {
  name?: string;
  round?: number;
  provider?: string;
  pr?: string;
  reviews_dir?: string;
  batch_groups?: Record<string, IssueEntry[]>;
  auto_commit?: boolean;
  mode?: ExecutionMode;
  memory?: WorkflowMemoryContext;
}

/** Mirrors the mutable create-session payload delivered to agent hooks. */
export interface SessionRequest {
  prompt?: string;
  working_dir?: string;
  model?: string;
  extra_env?: Record<string, string>;
}

/** Mirrors the mutable resume-session payload delivered to agent hooks. */
export interface ResumeSessionRequest {
  session_id?: string;
  prompt?: string;
  working_dir?: string;
  model?: string;
  extra_env?: Record<string, string>;
}

/** Captures the stable agent session identifiers. */
export interface SessionIdentity {
  acp_session_id: string;
  agent_session_id?: string;
  resumed?: boolean;
}

/** Mirrors the terminal session outcome payload. */
export interface SessionOutcome {
  status: SessionStatus;
  error?: string;
}

/** Mirrors the planned job shape exposed to run/job hooks. */
export interface Job {
  code_files?: string[];
  groups?: Record<string, IssueEntry[]>;
  task_title?: string;
  task_type?: string;
  safe_name?: string;
  prompt?: string;
  system_prompt?: string;
  out_prompt_path?: string;
  out_log?: string;
  err_log?: string;
}

/** Mirrors the review provider fetch configuration. */
export interface FetchConfig {
  reviews_dir?: string;
  include_resolved?: boolean;
}

/** Mirrors the review fix result payload. */
export interface FixOutcome {
  status: string;
  error?: string;
}

/** Mirrors the job execution result payload. */
export interface JobResult {
  status: string;
  exit_code?: number;
  attempts?: number;
  duration_ms?: number;
  error?: string;
}

/** Mirrors the run configuration payload exposed to run hooks. */
export interface RuntimeConfig {
  workspace_root?: string;
  name?: string;
  round?: number;
  provider?: string;
  pr?: string;
  nitpicks?: boolean;
  reviews_dir?: string;
  tasks_dir?: string;
  dry_run?: boolean;
  auto_commit?: boolean;
  concurrent?: number;
  batch_size?: number;
  ide?: string;
  model?: string;
  add_dirs?: string[];
  tail_lines?: number;
  reasoning_effort?: string;
  access_mode?: string;
  mode?: ExecutionMode;
  output_format?: OutputFormat;
  verbose?: boolean;
  tui?: boolean;
  persist?: boolean;
  enable_executable_extensions?: boolean;
  run_id?: string;
  parent_run_id?: string;
  prompt_text?: string;
  prompt_file?: string;
  read_prompt_stdin?: boolean;
  resolved_prompt_text?: string;
  include_completed?: boolean;
  include_resolved?: boolean;
  timeout_ms?: number;
  max_retries?: number;
  retry_backoff_multiplier?: number;
}

/** Mirrors the run artifact directory layout exposed to run hooks. */
export interface RunArtifacts {
  run_id?: string;
  run_dir?: string;
  run_meta_path?: string;
  events_path?: string;
  turns_dir?: string;
  jobs_dir?: string;
  result_path?: string;
}

/** Mirrors the terminal run summary payload. */
export interface RunSummary {
  status: string;
  jobs_total: number;
  jobs_succeeded?: number;
  jobs_failed?: number;
  jobs_canceled?: number;
  error?: string;
  teardown_error?: string;
}

// ---------------------------------------------------------------------------
// Hook payload interfaces
// ---------------------------------------------------------------------------

/** Payload delivered for the {@link HOOKS.planPreDiscover | plan.pre_discover} hook. */
export interface PlanPreDiscoverPayload {
  run_id: string;
  workflow: string;
  mode: ExecutionMode;
  extra_sources?: string[];
}

/** Payload delivered for the {@link HOOKS.planPostDiscover | plan.post_discover} hook. */
export interface PlanPostDiscoverPayload {
  run_id: string;
  workflow: string;
  entries?: IssueEntry[];
}

/** Payload delivered for the {@link HOOKS.planPreGroup | plan.pre_group} hook. */
export interface PlanPreGroupPayload {
  run_id: string;
  entries?: IssueEntry[];
}

/** Payload delivered for the {@link HOOKS.planPostGroup | plan.post_group} hook. */
export interface PlanPostGroupPayload {
  run_id: string;
  groups?: Record<string, IssueEntry[]>;
}

/** Payload delivered for the {@link HOOKS.planPrePrepareJobs | plan.pre_prepare_jobs} hook. */
export interface PlanPrePrepareJobsPayload {
  run_id: string;
  groups?: Record<string, IssueEntry[]>;
}

/** Payload delivered for the {@link HOOKS.planPostPrepareJobs | plan.post_prepare_jobs} hook. */
export interface PlanPostPrepareJobsPayload {
  run_id: string;
  jobs?: Job[];
}

/** Payload delivered for the {@link HOOKS.promptPreBuild | prompt.pre_build} hook. */
export interface PromptPreBuildPayload {
  run_id: string;
  job_id: string;
  batch_params: BatchParams;
}

/** Payload delivered for the {@link HOOKS.promptPostBuild | prompt.post_build} hook. */
export interface PromptPostBuildPayload {
  run_id: string;
  job_id: string;
  prompt_text: string;
  batch_params: BatchParams;
}

/** Payload delivered for the {@link HOOKS.promptPreSystem | prompt.pre_system} hook. */
export interface PromptPreSystemPayload {
  run_id: string;
  job_id: string;
  system_addendum: string;
  batch_params: BatchParams;
}

/** Payload delivered for the {@link HOOKS.agentPreSessionCreate | agent.pre_session_create} hook. */
export interface AgentPreSessionCreatePayload {
  run_id: string;
  job_id: string;
  session_request: SessionRequest;
}

/** Payload delivered for the {@link HOOKS.agentPostSessionCreate | agent.post_session_create} hook. */
export interface AgentPostSessionCreatePayload {
  run_id: string;
  job_id: string;
  session_id: string;
  identity: SessionIdentity;
}

/** Payload delivered for the {@link HOOKS.agentPreSessionResume | agent.pre_session_resume} hook. */
export interface AgentPreSessionResumePayload {
  run_id: string;
  job_id: string;
  resume_request: ResumeSessionRequest;
}

/** Payload delivered for the {@link HOOKS.agentOnSessionUpdate | agent.on_session_update} hook. */
export interface AgentOnSessionUpdatePayload {
  run_id: string;
  job_id: string;
  session_id: string;
  update: SessionUpdate;
}

/** Payload delivered for the {@link HOOKS.agentPostSessionEnd | agent.post_session_end} hook. */
export interface AgentPostSessionEndPayload {
  run_id: string;
  job_id: string;
  session_id: string;
  outcome: SessionOutcome;
}

/** Payload delivered for the {@link HOOKS.jobPreExecute | job.pre_execute} hook. */
export interface JobPreExecutePayload {
  run_id: string;
  job: Job;
}

/** Payload delivered for the {@link HOOKS.jobPostExecute | job.post_execute} hook. */
export interface JobPostExecutePayload {
  run_id: string;
  job: Job;
  result: JobResult;
}

/** Payload delivered for the {@link HOOKS.jobPreRetry | job.pre_retry} hook. */
export interface JobPreRetryPayload {
  run_id: string;
  job: Job;
  attempt: number;
  last_error: string;
}

/** Payload delivered for the {@link HOOKS.runPreStart | run.pre_start} hook. */
export interface RunPreStartPayload {
  run_id: string;
  config: RuntimeConfig;
  artifacts: RunArtifacts;
}

/** Payload delivered for the {@link HOOKS.runPostStart | run.post_start} hook. */
export interface RunPostStartPayload {
  run_id: string;
  config: RuntimeConfig;
}

/** Payload delivered for the {@link HOOKS.runPreShutdown | run.pre_shutdown} hook. */
export interface RunPreShutdownPayload {
  run_id: string;
  reason: string;
}

/** Payload delivered for the {@link HOOKS.runPostShutdown | run.post_shutdown} hook. */
export interface RunPostShutdownPayload {
  run_id: string;
  reason: string;
  summary: RunSummary;
}

/** Payload delivered for the {@link HOOKS.reviewPreFetch | review.pre_fetch} hook. */
export interface ReviewPreFetchPayload {
  run_id: string;
  pr: string;
  provider: string;
  fetch_config: FetchConfig;
}

/** Payload delivered for the {@link HOOKS.reviewPostFetch | review.post_fetch} hook. */
export interface ReviewPostFetchPayload {
  run_id: string;
  pr: string;
  issues?: IssueEntry[];
}

/** Payload delivered for the {@link HOOKS.reviewPreBatch | review.pre_batch} hook. */
export interface ReviewPreBatchPayload {
  run_id: string;
  pr: string;
  groups?: Record<string, IssueEntry[]>;
}

/** Payload delivered for the {@link HOOKS.reviewPostFix | review.post_fix} hook. */
export interface ReviewPostFixPayload {
  run_id: string;
  pr: string;
  issue: IssueEntry;
  outcome: FixOutcome;
}

/** Payload delivered for the {@link HOOKS.reviewPreResolve | review.pre_resolve} hook. */
export interface ReviewPreResolvePayload {
  run_id: string;
  pr: string;
  issue: IssueEntry;
  outcome: FixOutcome;
}

/** Payload delivered for the {@link HOOKS.artifactPreWrite | artifact.pre_write} hook. */
export interface ArtifactPreWritePayload {
  run_id: string;
  path: string;
  content_preview: string;
}

/** Payload delivered for the {@link HOOKS.artifactPostWrite | artifact.post_write} hook. */
export interface ArtifactPostWritePayload {
  run_id: string;
  path: string;
  bytes_written: number;
}

// ---------------------------------------------------------------------------
// Patch interfaces
// ---------------------------------------------------------------------------

/** Patch returned by the {@link HOOKS.planPreDiscover | plan.pre_discover} handler to describe what to mutate. */
export interface ExtraSourcesPatch {
  extra_sources?: string[];
}

/** Patch that replaces one issue entry slice. */
export interface EntriesPatch {
  entries?: IssueEntry[];
}

/** Patch that replaces one review issue slice. */
export interface IssuesPatch {
  issues?: IssueEntry[];
}

/** Patch that replaces one grouped issue map. */
export interface GroupsPatch {
  groups?: Record<string, IssueEntry[]>;
}

/** Patch that replaces one prepared job slice. */
export interface JobsPatch {
  jobs?: Job[];
}

/** Patch that replaces prompt build parameters. */
export interface BatchParamsPatch {
  batch_params?: BatchParams;
}

/** Patch that replaces the rendered prompt text. */
export interface PromptTextPatch {
  prompt_text?: string;
}

/** Patch that replaces the system prompt addendum. */
export interface SystemAddendumPatch {
  system_addendum?: string;
}

/** Patch that replaces the ACP create-session request payload. */
export interface SessionRequestPatch {
  session_request?: SessionRequest;
}

/** Patch that replaces the ACP resume-session request payload. */
export interface ResumeSessionRequestPatch {
  resume_request?: ResumeSessionRequest;
}

/** Patch that replaces one job payload. */
export interface JobPatch {
  job?: Job;
}

/** Patch that controls retry continuation and delay. */
export interface RetryDecisionPatch {
  proceed?: boolean;
  delay_ms?: number;
}

/** Patch that replaces the run configuration payload. */
export interface RuntimeConfigPatch {
  config?: RuntimeConfig;
}

/** Patch that replaces the review fetch configuration. */
export interface FetchConfigPatch {
  fetch_config?: FetchConfig;
}

/** Patch that controls remote issue resolution. */
export interface ResolveDecisionPatch {
  resolve?: boolean;
  message?: string;
}

/** Patch that mutates an artifact write request. */
export interface ArtifactWritePatch {
  path?: string;
  content?: string;
  cancel?: boolean;
}

// ---------------------------------------------------------------------------
// Host API request/result interfaces
// ---------------------------------------------------------------------------

/** Request payload for {@link EventsClient.subscribe | host.events.subscribe}. */
export interface EventSubscribeRequest {
  kinds: EventKind[];
}

/** Result payload for {@link EventsClient.subscribe | host.events.subscribe}. */
export interface EventSubscribeResult {
  subscription_id: string;
}

/** Request payload for {@link EventsClient.publish | host.events.publish}. */
export interface EventPublishRequest {
  kind: string;
  payload?: JsonValue;
}

/** Result payload for {@link EventsClient.publish | host.events.publish}. */
export interface EventPublishResult {
  seq?: number;
}

/** Task metadata written by {@link TasksClient.create | host.tasks.create}. */
export interface TaskFrontmatter {
  status: string;
  type: string;
  complexity?: string;
  dependencies?: string[];
}

/** One task document returned by the Host API. */
export interface Task {
  workflow: string;
  number: number;
  path: string;
  status: string;
  title?: string;
  type?: string;
  complexity?: string;
  dependencies?: string[];
  body?: string;
}

/** Request payload for {@link TasksClient.list | host.tasks.list}. */
export interface TaskListRequest {
  workflow: string;
}

/** Request payload for {@link TasksClient.get | host.tasks.get}. */
export interface TaskGetRequest {
  workflow: string;
  number: number;
}

/** Request payload for {@link TasksClient.create | host.tasks.create}. */
export interface TaskCreateRequest {
  workflow: string;
  title: string;
  body?: string;
  frontmatter?: TaskFrontmatter;
}

/** Request payload for {@link RunsClient.start | host.runs.start}. */
export interface RunStartRequest {
  runtime: RunConfig;
}

/** Host API run-start payload. */
export interface RunConfig {
  workspace_root?: string;
  name?: string;
  round?: number;
  provider?: string;
  pr?: string;
  reviews_dir?: string;
  tasks_dir?: string;
  auto_commit?: boolean;
  concurrent?: number;
  batch_size?: number;
  ide?: string;
  model?: string;
  add_dirs?: string[];
  tail_lines?: number;
  reasoning_effort?: string;
  access_mode?: string;
  mode?: ExecutionMode;
  output_format?: OutputFormat;
  verbose?: boolean;
  tui?: boolean;
  persist?: boolean;
  run_id?: string;
  prompt_text?: string;
  prompt_file?: string;
  read_prompt_stdin?: boolean;
  include_completed?: boolean;
  include_resolved?: boolean;
  timeout_ms?: number;
  max_retries?: number;
  retry_backoff_multiplier?: number;
}

/** Identifies a host-started run. */
export interface RunHandle {
  run_id: string;
  parent_run_id?: string;
}

/** Request payload for {@link ArtifactsClient.read | host.artifacts.read}. */
export interface ArtifactReadRequest {
  path: string;
}

/** Result payload for {@link ArtifactsClient.read | host.artifacts.read}. */
export interface ArtifactReadResult {
  path: string;
  content: string;
}

/** Request payload for {@link ArtifactsClient.write | host.artifacts.write}. */
export interface ArtifactWriteRequest {
  path: string;
  content: string;
}

/** Result payload for {@link ArtifactsClient.write | host.artifacts.write}. */
export interface ArtifactWriteResult {
  path: string;
  bytes_written: number;
}

/** Mirrors the prompt render issue input shape. */
export interface PromptIssueRef {
  name: string;
  abs_path?: string;
  content?: string;
  code_file?: string;
}

/** Describes the prompt render input snapshot. */
export interface PromptRenderParams {
  name?: string;
  round?: number;
  provider?: string;
  pr?: string;
  reviews_dir?: string;
  batch_groups?: Record<string, PromptIssueRef[]>;
  auto_commit?: boolean;
  mode?: ExecutionMode;
  memory?: WorkflowMemoryContext;
}

/** Request payload for {@link PromptsClient.render | host.prompts.render}. */
export interface PromptRenderRequest {
  template: string;
  params?: PromptRenderParams;
}

/** Result payload for {@link PromptsClient.render | host.prompts.render}. */
export interface PromptRenderResult {
  rendered: string;
}

/** Request payload for {@link MemoryClient.read | host.memory.read}. */
export interface MemoryReadRequest {
  workflow: string;
  task_file?: string;
}

/** Result payload for {@link MemoryClient.read | host.memory.read}. */
export interface MemoryReadResult {
  path: string;
  content: string;
  exists: boolean;
  needs_compaction: boolean;
}

/** Request payload for {@link MemoryClient.write | host.memory.write}. */
export interface MemoryWriteRequest {
  workflow: string;
  task_file?: string;
  content: string;
  mode?: MemoryWriteMode;
}

/** Result payload for {@link MemoryClient.write | host.memory.write}. */
export interface MemoryWriteResult {
  path: string;
  bytes_written: number;
}
