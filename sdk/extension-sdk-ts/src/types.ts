import type { HostAPI } from "./host_api.js";

export const PROTOCOL_VERSION = "1";
export const SDK_NAME = "@compozy/extension-sdk";
export const SDK_VERSION = "0.1.10";
export const MAX_MESSAGE_SIZE = 10 * 1024 * 1024;

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

export type Capability = (typeof CAPABILITIES)[keyof typeof CAPABILITIES];

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

export type HookName = (typeof HOOKS)[keyof typeof HOOKS];

export const EXECUTION_MODES = {
  prReview: "pr-review",
  prdTasks: "prd-tasks",
  exec: "exec",
} as const;

export type ExecutionMode = (typeof EXECUTION_MODES)[keyof typeof EXECUTION_MODES];

export const OUTPUT_FORMATS = {
  text: "text",
  json: "json",
  rawJson: "raw-json",
} as const;

export type OutputFormat = (typeof OUTPUT_FORMATS)[keyof typeof OUTPUT_FORMATS];

export const MEMORY_WRITE_MODES = {
  replace: "replace",
  append: "append",
} as const;

export type MemoryWriteMode = (typeof MEMORY_WRITE_MODES)[keyof typeof MEMORY_WRITE_MODES];

export const SESSION_STATUSES = {
  running: "running",
  completed: "completed",
  failed: "failed",
} as const;

export type SessionStatus = (typeof SESSION_STATUSES)[keyof typeof SESSION_STATUSES];

export type JsonPrimitive = boolean | number | string | null;
export interface JsonObject {
  [key: string]: JsonValue | undefined;
}
export type JsonValue = JsonPrimitive | JsonValue[] | JsonObject;

export type EventKind = string;

export interface Event {
  schema_version: string;
  run_id: string;
  seq: number;
  ts: string;
  kind: EventKind;
  payload?: JsonValue;
}

export interface Usage {
  input_tokens?: number;
  output_tokens?: number;
  total_tokens?: number;
  cache_reads?: number;
  cache_writes?: number;
}

export interface ContentBlock {
  type: string;
  [key: string]: JsonValue | undefined;
}

export interface SessionPlanEntry {
  content: string;
  priority: string;
  status: string;
}

export interface SessionAvailableCommand {
  name: string;
  description?: string;
  argument_hint?: string;
}

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

export interface HookInfo {
  name: string;
  event: HookName;
  mutable: boolean;
  required: boolean;
  priority: number;
  timeout_ms: number;
}

export interface HookContext {
  invocation_id: string;
  hook: HookInfo;
  host: HostAPI;
}

export interface InitializeRequestIdentity {
  name: string;
  version: string;
  source: "bundled" | "user" | "workspace";
}

export interface InitializeRuntime {
  run_id: string;
  parent_run_id?: string;
  workspace_root: string;
  invoking_command: string;
  shutdown_timeout_ms: number;
  default_hook_timeout_ms: number;
  health_check_interval_ms?: number;
}

export interface InitializeRequest {
  protocol_version: string;
  supported_protocol_versions: string[];
  compozy_version: string;
  extension: InitializeRequestIdentity;
  granted_capabilities?: Capability[];
  runtime: InitializeRuntime;
}

export interface InitializeResponseInfo {
  name?: string;
  version?: string;
  sdk_name?: string;
  sdk_version?: string;
}

export interface Supports {
  health_check: boolean;
  on_event: boolean;
}

export interface InitializeResponse {
  protocol_version: string;
  extension_info: InitializeResponseInfo;
  accepted_capabilities?: Capability[];
  supported_hook_events?: HookName[];
  supports: Supports;
}

export interface ExecuteHookRequest {
  invocation_id: string;
  hook: HookInfo;
  payload: JsonValue;
}

export interface ExecuteHookResponse {
  patch?: JsonValue;
}

export interface OnEventRequest {
  event: Event;
}

export interface HealthCheckRequest {}

export interface HealthCheckResponse {
  healthy: boolean;
  message?: string;
  details?: Record<string, JsonValue>;
}

export interface ShutdownRequest {
  reason: string;
  deadline_ms: number;
}

export interface ShutdownResponse {
  acknowledged: boolean;
}

export interface IssueEntry {
  name?: string;
  abs_path?: string;
  content?: string;
  code_file?: string;
}

export interface WorkflowMemoryContext {
  directory?: string;
  workflow_path?: string;
  task_path?: string;
  workflow_needs_compaction?: boolean;
  task_needs_compaction?: boolean;
}

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

export interface SessionRequest {
  prompt?: string;
  working_dir?: string;
  model?: string;
  extra_env?: Record<string, string>;
}

export interface ResumeSessionRequest {
  session_id?: string;
  prompt?: string;
  working_dir?: string;
  model?: string;
  extra_env?: Record<string, string>;
}

export interface SessionIdentity {
  acp_session_id: string;
  agent_session_id?: string;
  resumed?: boolean;
}

export interface SessionOutcome {
  status: SessionStatus;
  error?: string;
}

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

export interface FetchConfig {
  reviews_dir?: string;
  include_resolved?: boolean;
}

export interface FixOutcome {
  status: string;
  error?: string;
}

export interface JobResult {
  status: string;
  exit_code?: number;
  attempts?: number;
  duration_ms?: number;
  error?: string;
}

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

export interface RunArtifacts {
  run_id?: string;
  run_dir?: string;
  run_meta_path?: string;
  events_path?: string;
  turns_dir?: string;
  jobs_dir?: string;
  result_path?: string;
}

export interface RunSummary {
  status: string;
  jobs_total: number;
  jobs_succeeded?: number;
  jobs_failed?: number;
  jobs_canceled?: number;
  error?: string;
  teardown_error?: string;
}

export interface PlanPreDiscoverPayload {
  run_id: string;
  workflow: string;
  mode: ExecutionMode;
  extra_sources?: string[];
}

export interface PlanPostDiscoverPayload {
  run_id: string;
  workflow: string;
  entries?: IssueEntry[];
}

export interface PlanPreGroupPayload {
  run_id: string;
  entries?: IssueEntry[];
}

export interface PlanPostGroupPayload {
  run_id: string;
  groups?: Record<string, IssueEntry[]>;
}

export interface PlanPrePrepareJobsPayload {
  run_id: string;
  groups?: Record<string, IssueEntry[]>;
}

export interface PlanPostPrepareJobsPayload {
  run_id: string;
  jobs?: Job[];
}

export interface PromptPreBuildPayload {
  run_id: string;
  job_id: string;
  batch_params: BatchParams;
}

export interface PromptPostBuildPayload {
  run_id: string;
  job_id: string;
  prompt_text: string;
  batch_params: BatchParams;
}

export interface PromptPreSystemPayload {
  run_id: string;
  job_id: string;
  system_addendum: string;
  batch_params: BatchParams;
}

export interface AgentPreSessionCreatePayload {
  run_id: string;
  job_id: string;
  session_request: SessionRequest;
}

export interface AgentPostSessionCreatePayload {
  run_id: string;
  job_id: string;
  session_id: string;
  identity: SessionIdentity;
}

export interface AgentPreSessionResumePayload {
  run_id: string;
  job_id: string;
  resume_request: ResumeSessionRequest;
}

export interface AgentOnSessionUpdatePayload {
  run_id: string;
  job_id: string;
  session_id: string;
  update: SessionUpdate;
}

export interface AgentPostSessionEndPayload {
  run_id: string;
  job_id: string;
  session_id: string;
  outcome: SessionOutcome;
}

export interface JobPreExecutePayload {
  run_id: string;
  job: Job;
}

export interface JobPostExecutePayload {
  run_id: string;
  job: Job;
  result: JobResult;
}

export interface JobPreRetryPayload {
  run_id: string;
  job: Job;
  attempt: number;
  last_error: string;
}

export interface RunPreStartPayload {
  run_id: string;
  config: RuntimeConfig;
  artifacts: RunArtifacts;
}

export interface RunPostStartPayload {
  run_id: string;
  config: RuntimeConfig;
}

export interface RunPreShutdownPayload {
  run_id: string;
  reason: string;
}

export interface RunPostShutdownPayload {
  run_id: string;
  reason: string;
  summary: RunSummary;
}

export interface ReviewPreFetchPayload {
  run_id: string;
  pr: string;
  provider: string;
  fetch_config: FetchConfig;
}

export interface ReviewPostFetchPayload {
  run_id: string;
  pr: string;
  issues?: IssueEntry[];
}

export interface ReviewPreBatchPayload {
  run_id: string;
  pr: string;
  groups?: Record<string, IssueEntry[]>;
}

export interface ReviewPostFixPayload {
  run_id: string;
  pr: string;
  issue: IssueEntry;
  outcome: FixOutcome;
}

export interface ReviewPreResolvePayload {
  run_id: string;
  pr: string;
  issue: IssueEntry;
  outcome: FixOutcome;
}

export interface ArtifactPreWritePayload {
  run_id: string;
  path: string;
  content_preview: string;
}

export interface ArtifactPostWritePayload {
  run_id: string;
  path: string;
  bytes_written: number;
}

export interface ExtraSourcesPatch {
  extra_sources?: string[];
}

export interface EntriesPatch {
  entries?: IssueEntry[];
}

export interface IssuesPatch {
  issues?: IssueEntry[];
}

export interface GroupsPatch {
  groups?: Record<string, IssueEntry[]>;
}

export interface JobsPatch {
  jobs?: Job[];
}

export interface BatchParamsPatch {
  batch_params?: BatchParams;
}

export interface PromptTextPatch {
  prompt_text?: string;
}

export interface SystemAddendumPatch {
  system_addendum?: string;
}

export interface SessionRequestPatch {
  session_request?: SessionRequest;
}

export interface ResumeSessionRequestPatch {
  resume_request?: ResumeSessionRequest;
}

export interface JobPatch {
  job?: Job;
}

export interface RetryDecisionPatch {
  proceed?: boolean;
  delay_ms?: number;
}

export interface RuntimeConfigPatch {
  config?: RuntimeConfig;
}

export interface FetchConfigPatch {
  fetch_config?: FetchConfig;
}

export interface ResolveDecisionPatch {
  resolve?: boolean;
  message?: string;
}

export interface ArtifactWritePatch {
  path?: string;
  content?: string;
  cancel?: boolean;
}

export interface EventSubscribeRequest {
  kinds: EventKind[];
}

export interface EventSubscribeResult {
  subscription_id: string;
}

export interface EventPublishRequest {
  kind: string;
  payload?: JsonValue;
}

export interface EventPublishResult {
  seq?: number;
}

export interface TaskFrontmatter {
  status: string;
  type: string;
  complexity?: string;
  dependencies?: string[];
}

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

export interface TaskListRequest {
  workflow: string;
}

export interface TaskGetRequest {
  workflow: string;
  number: number;
}

export interface TaskCreateRequest {
  workflow: string;
  title: string;
  body?: string;
  frontmatter?: TaskFrontmatter;
}

export interface RunStartRequest {
  runtime: RunConfig;
}

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

export interface RunHandle {
  run_id: string;
  parent_run_id?: string;
}

export interface ArtifactReadRequest {
  path: string;
}

export interface ArtifactReadResult {
  path: string;
  content: string;
}

export interface ArtifactWriteRequest {
  path: string;
  content: string;
}

export interface ArtifactWriteResult {
  path: string;
  bytes_written: number;
}

export interface PromptIssueRef {
  name: string;
  abs_path?: string;
  content?: string;
  code_file?: string;
}

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

export interface PromptRenderRequest {
  template: string;
  params?: PromptRenderParams;
}

export interface PromptRenderResult {
  rendered: string;
}

export interface MemoryReadRequest {
  workflow: string;
  task_file?: string;
}

export interface MemoryReadResult {
  path: string;
  content: string;
  exists: boolean;
  needs_compaction: boolean;
}

export interface MemoryWriteRequest {
  workflow: string;
  task_file?: string;
  content: string;
  mode?: MemoryWriteMode;
}

export interface MemoryWriteResult {
  path: string;
  bytes_written: number;
}
