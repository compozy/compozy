import { NatsClient } from "./nats_client.ts";
import { type LogContext, LogLevel } from "./types.ts";

export class Logger {
  private correlationId?: string;
  private natsClient: NatsClient;

  constructor(readonly config: { verbose?: boolean } = { verbose: false }) {
    this.natsClient = new NatsClient({ verbose: config.verbose });
  }

  public setCorrelationId(id: string) {
    this.correlationId = id;
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
      correlationId: this.correlationId
    };

    await this.natsClient.sendLogMessage(level, message, enhancedContext);
  }
}
