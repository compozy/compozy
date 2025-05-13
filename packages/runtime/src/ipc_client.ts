import { EventEmitter } from "node:events";
import type { ErrorMessage, Input, IPCMessageType } from "./types.ts";
import { parseArgs } from "jsr:@std/cli";

export const ipcEmitter = new EventEmitter();

export class IpcClient {
  verbose: boolean = false;

  constructor(verbose?: boolean) {
    const args = parseArgs(Deno.args, {
      boolean: ["verbose"],
      default: { verbose: false },
    });
    this.verbose = verbose ?? args.verbose;
    ipcEmitter.on("send_log_message", (payload: any) => {
      this.sendLogMessage(payload);
    });
  }

  sendMessage<T>(type: IPCMessageType, payload: T) {
    const json = IpcClient.buildMessage(type, payload);
    Deno.stdout.writeSync(new TextEncoder().encode(json + "\n"));
  }
  sendLogMessage<T>(payload: T) {
    const json = IpcClient.buildMessage("Log", payload);
    Deno.stderr.writeSync(new TextEncoder().encode(json + "\n"));
  }

  sendErrorMessage(requestId: string | null, error: any, data: Input) {
    const message = error.message ?? "Unknown error";
    const stack = error?.stack ?? null;
    const context = { request_id: requestId, message, stack };
    const payload: ErrorMessage = { ...context, data };
    const json = IpcClient.buildMessage("Log", payload);
    Deno.stderr.writeSync(new TextEncoder().encode(json + "\n"));
  }

  async receiveMessage() {
    const reader = Deno.stdin.readable.getReader();
    const decoder = new TextDecoder();
    let input = "";
    try {
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        input += decoder.decode(value, { stream: true });
        try {
          const parsed = JSON.parse(input.trim());
          return parsed;
          // deno-lint-ignore no-empty
        } catch (_) {}
      }
      throw new Error("End of input reached without valid JSON");
    } finally {
      reader.releaseLock();
    }
  }

  static buildMessage<T>(type: IPCMessageType, payload: T) {
    return JSON.stringify({ type, payload });
  }
}
