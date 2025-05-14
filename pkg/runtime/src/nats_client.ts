import { connect, type ConnectionOptions, type NatsConnection } from "npm:nats@2.17.0";
import { parseArgs } from "jsr:@std/cli";
import type { LogLevel, ErrorMessage, IPCMessageType } from "./types.ts";

// Custom error for NATS-related issues
export class NatsClientError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "NatsClientError";
  }
}

interface NatsClientOptions {
  verbose?: boolean;
  serverUrl?: string;
  namespace?: string;
  execId?: string;
}

export class NatsClient {
  private connection: NatsConnection | null = null;
  private readonly verbose: boolean;
  private readonly execId: string;
  private readonly serverUrl: string;
  private readonly namespace: string;

  constructor(options: NatsClientOptions = {}) {
    const args = parseArgs(Deno.args, {
      boolean: ["verbose"],
      string: ["exec-id", "nats-url", "namespace"],
      default: {
        verbose: false,
        "exec-id": crypto.randomUUID(),
        "nats-url": "nats://localhost:4222",
        namespace: "compozy",
      },
    });

    this.verbose = options.verbose ?? args.verbose;
    this.execId = options.execId ?? args["exec-id"];
    this.serverUrl = options.serverUrl ?? args["nats-url"];
    this.namespace = options.namespace ?? args["namespace"];
  }

  /**
   * Establishes a connection to the NATS server if not already connected.
   * @throws {NatsClientError} If connection fails.
   */
  async connect(): Promise<void> {
    if (this.connection) {
      return;
    }

    try {
      const options: ConnectionOptions = { servers: this.serverUrl };
      this.connection = await connect(options);
      this.logConnectionSuccess();
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      throw new NatsClientError(`Failed to connect to NATS: ${message}`);
    }
  }

  /**
   * Disconnects from the NATS server and cleans up resources.
   */
  async disconnect(): Promise<void> {
    if (!this.connection) {
      return;
    }

    try {
      await this.connection.drain();
    } finally {
      this.connection = null;
    }
  }

  getConnection(): NatsConnection | null {
    return this.connection;
  }

  /**
   * Checks if the client is connected to the NATS server.
   */
  isConnected(): boolean {
    return !!this.connection;
  }

  /**
   * Generates NATS subject based on message type and ID.
   * @param type - The type of message (e.g., AgentRequest, ToolResponse).
   * @param id - Optional ID for specific subjects.
   * @returns The generated subject string.
   */
  private getSubject(type: string, id: string = ""): string {
    const baseSubject = `${this.namespace}.${this.execId}`;
    switch (type) {
      case "AgentRequest":
        return `${baseSubject}.agent.${id}.request`;
      case "AgentResponse":
        return `${baseSubject}.agent.${id}.response`;
      case "ToolRequest":
        return `${baseSubject}.tool.${id}.request`;
      case "ToolResponse":
        return `${baseSubject}.tool.${id}.response`;
      case "Error":
        return `${baseSubject}.error`;
      case "Log":
        return `${baseSubject}.log`;
      default:
        return `${baseSubject}.${type.toLowerCase()}`;
    }
  }

  /**
   * Gets the subject pattern for external use or testing.
   * @param type - The type of message.
   * @param id - Optional ID for specific subjects.
   * @returns The subject pattern.
   */
  getSubjectPattern(type: string, id?: string): string {
    return this.getSubject(type, id ?? "");
  }

  /**
   * Gets the log subject for a specific log level.
   * @param level - The log level.
   * @returns The log subject.
   */
  getLogSubject(level: LogLevel): string {
    return `${this.namespace}.${this.execId}.log.${level.toLowerCase()}`;
  }

  /**
   * Sends a message to a NATS subject.
   * @param type - The IPC message type.
   * @param payload - The message payload.
   * @param id - Optional ID for specific subjects.
   * @throws {NatsClientError} If not connected or publishing fails.
   */
  async sendMessage<T>(type: IPCMessageType, payload: T, id?: string): Promise<void> {
    await this.ensureConnected();
    const subject = this.getSubject(type, id ?? "");
    const message = { exec_id: this.execId, type, payload };

    try {
      this.connection!.publish(subject, JSON.stringify(message));
      if (this.verbose) {
        console.error(`Published message to ${subject}`);
      }
    } catch (error) {
      throw new NatsClientError(`Failed to publish message to ${subject}: ${String(error)}`);
    }
  }

  /**
   * Sends a log message to a NATS subject.
   * @param level - The log level.
   * @param message - The log message.
   * @param context - Optional context for the log.
   * @throws {NatsClientError} If not connected or publishing fails.
   */
  async sendLogMessage<T>(
    level: LogLevel,
    message: string,
    context?: Record<string, any>,
  ): Promise<void> {
    await this.ensureConnected();
    const subject = this.getLogSubject(level);
    const logPayload = {
      level,
      message,
      context,
      timestamp: new Date().toISOString(),
    };

    try {
      this.connection!.publish(subject, JSON.stringify({
        exec_id: this.execId,
        type: "Log",
        payload: logPayload,
      }));
    } catch (error) {
      throw new NatsClientError(`Failed to publish log to ${subject}: ${String(error)}`);
    }
  }

  /**
   * Sends an error message to the error subject.
   * @param requestId - Optional request ID.
   * @param error - The error object or message.
   * @param data - Additional data for context.
   * @throws {NatsClientError} If not connected or publishing fails.
   */
  async sendErrorMessage(
    requestId: string | null,
    error: unknown,
    data: Record<string, any>,
  ): Promise<void> {
    const message = error instanceof Error ? error.message : "Unknown error";
    const stack = error instanceof Error ? error.stack : null;
    const payload: ErrorMessage = { request_id: requestId, message, stack, data };

    await this.sendMessage("Error", payload);
  }

  /**
   * Sends a request and waits for a response.
   * @param type - The request type.
   * @param id - The request ID.
   * @param payload - The request payload.
   * @param timeout - Request timeout in milliseconds.
   * @returns The response payload.
   * @throws {NatsClientError} If not connected, request fails, or response is invalid.
   */
  async request<T, R>(type: string, id: string, payload: T, timeout = 30000): Promise<R> {
    await this.ensureConnected();
    const subject = this.getSubject(type, id);
    const message = { exec_id: this.execId, type, payload };

    try {
      const response = await this.connection!.request(
        subject,
        JSON.stringify(message),
        { timeout },
      );
      const responseData = JSON.parse(new TextDecoder().decode(response.data));

      if (responseData.type === "Error") {
        throw new NatsClientError(responseData.payload.message);
      }

      return responseData.payload as R;
    } catch (error) {
      throw new NatsClientError(`Request to ${subject} failed: ${String(error)}`);
    }
  }

  /**
   * Subscribes to a NATS subject with a callback.
   * @param type - The message type to subscribe to.
   * @param id - The subscription ID.
   * @param callback - The callback to handle received messages.
   * @returns A function to unsubscribe.
   * @throws {NatsClientError} If not connected or subscription fails.
   */
  async subscribe<T>(type: string, id: string, callback: (payload: T) => void): Promise<() => void> {
    await this.ensureConnected();
    const subject = this.getSubject(type, id);
    const subscription = this.connection!.subscribe(subject);

    (async () => {
      try {
        for await (const msg of subscription) {
          const data = JSON.parse(new TextDecoder().decode(msg.data));
          callback(data.payload as T);
        }
      } catch (error) {
        console.error(`Error processing message: ${String(error)}`);
      }
    })();

    return () => {
      subscription.unsubscribe();
    };
  }

  /**
   * Ensures the client is connected, throwing an error if not.
   * @throws {NatsClientError} If not connected.
   */
  private async ensureConnected(): Promise<void> {
    if (!this.connection) {
      await this.connect();
    }
  }

  /**
   * Logs successful connection to the NATS server.
   */
  private logConnectionSuccess(): void {
    if (this.verbose) {
      console.error(`Connected to NATS server at ${this.serverUrl}`);
    }
  }
}
