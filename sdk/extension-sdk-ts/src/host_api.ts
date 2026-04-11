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

export interface HostCaller {
  call<T>(method: string, params?: unknown): Promise<T>;
}

export class EventsClient {
  constructor(private readonly caller: HostCaller) {}

  subscribe(params: EventSubscribeRequest): Promise<EventSubscribeResult> {
    return this.caller.call("host.events.subscribe", params);
  }

  publish(params: EventPublishRequest): Promise<EventPublishResult> {
    return this.caller.call("host.events.publish", params);
  }
}

export class TasksClient {
  constructor(private readonly caller: HostCaller) {}

  list(params: TaskListRequest): Promise<Task[]> {
    return this.caller.call("host.tasks.list", params);
  }

  get(params: TaskGetRequest): Promise<Task> {
    return this.caller.call("host.tasks.get", params);
  }

  create(params: TaskCreateRequest): Promise<Task> {
    return this.caller.call("host.tasks.create", params);
  }
}

export class RunsClient {
  constructor(private readonly caller: HostCaller) {}

  start(params: RunStartRequest): Promise<RunHandle> {
    return this.caller.call("host.runs.start", params);
  }
}

export class ArtifactsClient {
  constructor(private readonly caller: HostCaller) {}

  read(params: ArtifactReadRequest): Promise<ArtifactReadResult> {
    return this.caller.call("host.artifacts.read", params);
  }

  write(params: ArtifactWriteRequest): Promise<ArtifactWriteResult> {
    return this.caller.call("host.artifacts.write", params);
  }
}

export class PromptsClient {
  constructor(private readonly caller: HostCaller) {}

  render(params: PromptRenderRequest): Promise<PromptRenderResult> {
    return this.caller.call("host.prompts.render", params);
  }
}

export class MemoryClient {
  constructor(private readonly caller: HostCaller) {}

  read(params: MemoryReadRequest): Promise<MemoryReadResult> {
    return this.caller.call("host.memory.read", params);
  }

  write(params: MemoryWriteRequest): Promise<MemoryWriteResult> {
    return this.caller.call("host.memory.write", params);
  }
}

export class HostAPI {
  readonly events: EventsClient;
  readonly tasks: TasksClient;
  readonly runs: RunsClient;
  readonly artifacts: ArtifactsClient;
  readonly prompts: PromptsClient;
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
