import type { NatsClient } from "./nats_client.ts";
import type { Logger } from "./logger.ts";
import { Processor } from "./processor.ts";
import type { RequestType, ToolRequest, ToolResponse } from "./types.ts";
import { loadToolDinamically } from "./utils.ts";

export class ToolProcessor extends Processor {
  constructor(logger: Logger, natsClient: NatsClient, verbose: boolean = false) {
    super(logger, natsClient, verbose);
  }

  public async processRequest<T>(
    type: RequestType,
    request: ToolRequest,
  ): Promise<ToolResponse<T>> {
    return await this.withTiming("ProcessToolRequest", async () => {
      try {
        this.logger.setCorrelationId(request.id);
        this.logger.debug("Processing tool request", {
          type,
          toolId: request.tool_id,
          requestId: request.id,
        });

        const toolModule = await loadToolDinamically(request.tool_id);
        const input = request.input || {};
        const output = await toolModule.run(input);

        return {
          id: request.id,
          tool_id: request.tool_id,
          output: output as T,
          status: "Success",
        };
      } catch (error: unknown) {
        const errorMessage = error instanceof Error ? error.message : String(error);
        this.logger.error("Tool processing error", {
          error: errorMessage,
          stack: error instanceof Error ? error.stack : undefined,
          requestId: request.id,
        });
        return {
          id: request.id,
          tool_id: request.tool_id,
          output: errorMessage as unknown as T,
          status: "Error",
        };
      }
    });
  }
}
