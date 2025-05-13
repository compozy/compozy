import { parseArgs } from "jsr:@std/cli";
import type { IpcClient } from "./ipc_client.ts";
import type { Logger } from "./logger.ts";
import type { RequestType } from "./types.ts";

export abstract class Processor {
  private stderrAbortController: AbortController;

  constructor(
    readonly logger: Logger,
    readonly ipcClient: IpcClient,
    readonly verbose: boolean = false,
  ) {
    this.stderrAbortController = new AbortController();
  }

  public cleanup(): void {
    this.stderrAbortController.abort();
  }

  public parseCommandLineArgs(type: RequestType) {
    const args = parseArgs(Deno.args, {
      string: [type === "agent" ? "agent-id" : "tool-id", "request-id"],
    });

    return {
      id: type === "agent"
        ? (args["agent-id"] ?? null)
        : (args["tool-id"] ?? null),
      requestId: args["request-id"] ?? null,
      verbose: args.verbose ?? this.verbose,
    };
  }

  protected async withTiming<T>(operation: string, fn: () => Promise<T>) {
    const start = performance.now();
    try {
      const result = await fn();
      this.logger.debug(`${operation} completed`, {
        duration: performance.now() - start,
      });
      return result;
    } catch (error: any) {
      this.logger.error(`${operation} failed`, {
        duration: performance.now() - start,
        error: error.message,
      });
      throw error;
    }
  }

  public async readRequestFromStdin<T>(expectedType: string) {
    return await this.withTiming("ReadRequestFromStdin", async () => {
      const request = await this.ipcClient.receiveMessage();
      if (request.type === expectedType && request.payload) {
        this.logger.info(
          `Successfully parsed ${expectedType.toLowerCase()} request`,
          {
            size: JSON.stringify(request).length,
          },
        );
        return request as { type: string; payload: T };
      }
      throw new Error(
        `Invalid request type or empty request. Expected: ${expectedType}`,
      );
    });
  }
}
