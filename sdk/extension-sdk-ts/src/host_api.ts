import type {
  ArtifactReadRequest,
  ArtifactReadResult,
  ArtifactWriteRequest,
  ArtifactWriteResult,
  EventPublishRequest,
  EventPublishResult,
  EventSubscribeRequest,
  EventSubscribeResult,
  MemoryReadRequest,
  MemoryReadResult,
  MemoryWriteRequest,
  MemoryWriteResult,
  PromptRenderRequest,
  PromptRenderResult,
  RunHandle,
  RunStartRequest,
  Task,
  TaskCreateRequest,
  TaskGetRequest,
  TaskListRequest,
} from "./types.js";

/** Transport-level interface for issuing JSON-RPC calls to the host. */
export interface HostCaller {
  call<T>(method: string, params?: unknown): Promise<T>;
}

/** Client for the host.events namespace. Requires the events.read or events.publish capability. */
export class EventsClient {
  constructor(private readonly caller: HostCaller) {}

  /** Registers the event kind filter for this extension session. */
  subscribe(params: EventSubscribeRequest): Promise<EventSubscribeResult> {
    return this.caller.call("host.events.subscribe", params);
  }

  /** Publishes a custom event into the run event bus. */
  publish(params: EventPublishRequest): Promise<EventPublishResult> {
    return this.caller.call("host.events.publish", params);
  }
}

/** Client for the host.tasks namespace. Requires the tasks.read or tasks.create capability. */
export class TasksClient {
  constructor(private readonly caller: HostCaller) {}

  /** Lists all tasks in the specified workflow. */
  list(params: TaskListRequest): Promise<Task[]> {
    return this.caller.call("host.tasks.list", params);
  }

  /** Retrieves a single task by workflow and number. */
  get(params: TaskGetRequest): Promise<Task> {
    return this.caller.call("host.tasks.get", params);
  }

  /** Creates a new task file in the specified workflow. */
  create(params: TaskCreateRequest): Promise<Task> {
    return this.caller.call("host.tasks.create", params);
  }
}

/** Client for the host.runs namespace. Requires the runs.start capability. */
export class RunsClient {
  constructor(private readonly caller: HostCaller) {}

  /** Starts a nested Compozy run with the provided configuration. */
  start(params: RunStartRequest): Promise<RunHandle> {
    return this.caller.call("host.runs.start", params);
  }
}

/** Client for the host.artifacts namespace. Requires the artifacts.read or artifacts.write capability. */
export class ArtifactsClient {
  constructor(private readonly caller: HostCaller) {}

  /** Reads an artifact file from the workspace. */
  read(params: ArtifactReadRequest): Promise<ArtifactReadResult> {
    return this.caller.call("host.artifacts.read", params);
  }

  /** Writes content to an artifact file in the workspace. */
  write(params: ArtifactWriteRequest): Promise<ArtifactWriteResult> {
    return this.caller.call("host.artifacts.write", params);
  }
}

/** Client for the host.prompts namespace. No capability required. */
export class PromptsClient {
  constructor(private readonly caller: HostCaller) {}

  /** Renders a prompt template with the provided parameters. */
  render(params: PromptRenderRequest): Promise<PromptRenderResult> {
    return this.caller.call("host.prompts.render", params);
  }
}

/** Client for the host.memory namespace. Requires the memory.read or memory.write capability. */
export class MemoryClient {
  constructor(private readonly caller: HostCaller) {}

  /** Reads workflow or task memory content. */
  read(params: MemoryReadRequest): Promise<MemoryReadResult> {
    return this.caller.call("host.memory.read", params);
  }

  /** Writes or appends to workflow or task memory. */
  write(params: MemoryWriteRequest): Promise<MemoryWriteResult> {
    return this.caller.call("host.memory.write", params);
  }
}

/** Aggregated Host API surface exposed to extension handlers via Extension.host. */
export class HostAPI {
  /** Client for the host.events namespace. */
  readonly events: EventsClient;
  /** Client for the host.tasks namespace. */
  readonly tasks: TasksClient;
  /** Client for the host.runs namespace. */
  readonly runs: RunsClient;
  /** Client for the host.artifacts namespace. */
  readonly artifacts: ArtifactsClient;
  /** Client for the host.prompts namespace. */
  readonly prompts: PromptsClient;
  /** Client for the host.memory namespace. */
  readonly memory: MemoryClient;

  constructor(caller: HostCaller) {
    this.events = new EventsClient(caller);
    this.tasks = new TasksClient(caller);
    this.runs = new RunsClient(caller);
    this.artifacts = new ArtifactsClient(caller);
    this.prompts = new PromptsClient(caller);
    this.memory = new MemoryClient(caller);
  }
}
