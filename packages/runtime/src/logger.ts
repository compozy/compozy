import { ipcEmitter } from "./ipc_client.ts";
import { type LogContext, LogLevel, type LogMessage } from "./types.ts";

export class Logger {
  private correlationId?: string;

  constructor(readonly config: { verbose?: boolean } = { verbose: false }) {}

  public setCorrelationId(id: string) {
    this.correlationId = id;
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

  sendLog(level: LogLevel, message: string, context?: LogContext) {
    if (level === LogLevel.Debug && !this.config.verbose) return;
    const payload: LogMessage = {
      level,
      message,
      context: { ...context, correlationId: this.correlationId },
      timestamp: new Date().toISOString(),
    };
    ipcEmitter.emit("send_log_message", payload);
  }
}
