import { parseArgs } from "jsr:@std/cli";
import type { NatsClient } from "./nats_client.ts";
import type { Logger } from "./logger.ts";
import type { RequestType } from "./types.ts";

export abstract class Processor {
  private stderrAbortController: AbortController;

  constructor(
    readonly logger: Logger,
    readonly natsClient: NatsClient,
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
    } catch (error: unknown) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      this.logger.error(`${operation} failed`, {
        duration: performance.now() - start,
        error: errorMessage,
      });
      throw error;
    }
  }

  public async readRequestFromStdin<T>(expectedType: string) {
    return await this.withTiming("ReadRequestFromStdin", async () => {
      // Read from stdin
      const buffer = new Uint8Array(1024);
      let result = new Uint8Array(0);
      let totalRead = 0;

      while (true) {
        const readResult = await Deno.stdin.read(buffer);
        if (readResult === null) break;

        const newBuffer = new Uint8Array(totalRead + readResult);
        newBuffer.set(result);
        newBuffer.set(buffer.subarray(0, readResult), totalRead);
        result = newBuffer;
        totalRead += readResult;
      }

      const text = new TextDecoder().decode(result);

      try {
        const request = JSON.parse(text);
        if (request.type === expectedType && request.payload) {
          this.logger.info(
            `Successfully parsed ${expectedType.toLowerCase()} request from stdin`,
            {
              size: text.length,
            },
          );
          return request as { type: string; payload: T };
        }
        throw new Error(
          `Invalid request type or empty request. Expected: ${expectedType}`,
        );
      } catch (error) {
        throw new Error(`Failed to parse stdin input: ${error instanceof Error ? error.message : String(error)}`);
      }
    });
  }
}
