import { discover } from "./browser-discovery.js";
import {
  connect,
  type ConnectedState,
  type ConnectOptions,
  type DaemonIdentity,
  type Discovery,
} from "./connect.js";
import { CompozyApiError } from "./errors.js";
import {
  events,
  type SseFrame,
  type SseOperationId,
  type SseOptions,
  type StreamEvent,
} from "./events.js";
import { createClient } from "./generated/sdk/client/index.js";
import { Compozy as GeneratedCompozy } from "./generated/sdk/sdk.gen.js";
import {
  createPromptId,
  prompt,
  promptResponse,
  type PromptInput,
  type PromptReply,
} from "./prompt.js";

export type { ConnectOptions, DaemonIdentity } from "./connect.js";

/** Public application client. Construct a connected instance with Compozy.connect(). */
export class Compozy extends GeneratedCompozy {
  protected static discovery: Discovery = discover;
  protected createPromptId = createPromptId;
  readonly daemon: DaemonIdentity;
  protected readonly state: ConnectedState;

  protected constructor(state: ConnectedState) {
    const client = createClient({
      baseUrl: state.baseUrl,
      headers: state.headers,
      fetch: state.fetch,
      throwOnError: true,
      responseStyle: "data",
    });
    client.interceptors.error.use((error, response) => {
      if (error instanceof Error) return error;
      return response ? new CompozyApiError(response.status, error, response.statusText) : error;
    });
    super({ client });
    this.state = state;
    this.daemon = state.daemon;
  }

  protected static connection(options: ConnectOptions): Promise<ConnectedState> {
    return connect(options, this.discovery);
  }

  static async connect(options: ConnectOptions = {}): Promise<Compozy> {
    return new this(await this.connection(options));
  }

  prompt(input: PromptInput): Promise<PromptReply> {
    return prompt(this.state, input, this.createPromptId);
  }

  promptResponse(input: PromptInput): Promise<Response> {
    return promptResponse(this.state, input, this.createPromptId);
  }

  events<Id extends SseOperationId>(
    operationId: Id,
    options: SseOptions<Id>
  ): AsyncIterable<StreamEvent<SseFrame<Id>>> {
    return events(this.state, operationId, options);
  }
}
