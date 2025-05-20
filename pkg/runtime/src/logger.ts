import { NatsClient } from "./nats_client.ts";
import { type LogContext, LogLevel } from "./types.ts";

export class Logger {
  private correlationID?: string;
  private natsClient: NatsClient;

  constructor(readonly config: { verbose?: boolean } = { verbose: false }) {
    this.natsClient = new NatsClient({ verbose: config.verbose });
  }

  public setCorrelationID(id: string) {
    this.correlationID = id;
  }

  public setClient(client: NatsClient) {
    this.natsClient = client;
  }

  public debug(message: string, context?: LogContext) {
    this.sendLog(LogLevel.Debug, message, context);
  }

  public info(message: string, context?: LogContext) {
    this.sendLog(LogLevel.Info, message, context);
  }

  public warn(message: string, context?: LogContext) {
    this.sendLog(LogLevel.Warn, message, context);
  }

  public error(message: string, context?: LogContext) {
    this.sendLog(LogLevel.Error, message, context);
  }

  async sendLog(level: LogLevel, message: string, context?: LogContext) {
    if (level === LogLevel.Debug && !this.config.verbose) return;

    const enhancedContext = {
      ...context,
      correlationID: this.correlationID
    };

    await this.natsClient.sendLogMessage(level, message, enhancedContext);
  }
}
