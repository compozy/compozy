import { createOpenAI } from "npm:@ai-sdk/openai";
import { Agent as MastraAgent } from "npm:@mastra/core/agent";
import {
  createTool as createMastraTool,
  type ToolAction,
} from "npm:@mastra/core/tools";
import { JSONSchemaToZod } from "npm:@dmitryrechkin/json-schema-to-zod";
import { Processor } from "./processor.ts";
import type { AgentRequest, AgentResponse, RequestType } from "./types.ts";
import type { NatsClient } from "./nats_client.ts";
import type { Logger } from "./logger.ts";
import { loadToolsDinamically } from "./utils.ts";

export class AgentProcessor extends Processor {
  constructor(logger: Logger, natsClient: NatsClient, verbose: boolean = false) {
    super(logger, natsClient, verbose);
  }

  private createProvider(request: AgentRequest["payload"]) {
    let baseURL: string = "https://api.openai.com/v1";
    if (request.config.provider === "groq") {
      baseURL = "https://api.groq.com/openai/v1";
    }
    return createOpenAI({
      baseURL,
      apiKey: request.config.api_key,
      compatibility: "compatible",
    });
  }

  private createOpenAIProvider(request: AgentRequest["payload"]) {
    this.logger.debug("Creating OpenAI provider", {
      provider: request.config.provider,
      model: request.config.model,
      api_key: request.config.api_key.slice(0, 4) + "...",
    });

    const openai = this.createProvider(request);
    return openai(request.config.model);
  }

  private getInputSchema(schema: any) {
    try {
      const inputSchema = typeof schema === "string"
        ? JSON.parse(schema)
        : schema;
      return JSONSchemaToZod.convert(inputSchema);
    } catch (error) {
      const errorMessage = error instanceof Error
        ? error.message
        : "Unknown error";
      throw new Error(`Failed to convert input schema: ${errorMessage}`);
    }
  }

  private getOutputSchema(schema: any) {
    try {
      const outputSchema = typeof schema === "string"
        ? JSON.parse(schema)
        : schema;
      return outputSchema ? JSONSchemaToZod.convert(outputSchema) : null;
    } catch (error) {
      const errorMessage = error instanceof Error
        ? error.message
        : "Unknown error";
      throw new Error(`Failed to convert output schema: ${errorMessage}`);
    }
  }

  private getToolSchemas(tool: AgentRequest["payload"]["tools"][number]) {
    try {
      const inputSchemaZod = this.getInputSchema(tool.input_schema);
      const outputSchemaZod = this.getOutputSchema(tool.output_schema);
      return { inputSchemaZod, outputSchemaZod };
    } catch (error) {
      const errorMessage = error instanceof Error
        ? error.message
        : "Unknown error";
      throw new Error(`Failed to convert tool schemas: ${errorMessage}`);
    }
  }

  private async createAgentTools(tools: AgentRequest["payload"]["tools"]) {
    return await this.withTiming("CreateAgentTools", async () => {
      const toolsMap = await loadToolsDinamically(tools, this.logger);
      return tools.reduce(
        (acc, tool) => {
          const toolFile = toolsMap.get(tool.id);
          const { inputSchemaZod, outputSchemaZod } = this.getToolSchemas(tool);
          acc[tool.id] = createMastraTool({
            id: tool.id,
            description: tool.description,
            inputSchema: inputSchemaZod,
            ...(outputSchemaZod ? { outputSchema: outputSchemaZod } : {}),
            execute: async ({ context }) => {
              return await toolFile.run(context);
            },
          });
          return acc;
        },
        {} as Record<string, ToolAction<any, any, any, any>>,
      );
    });
  }

  private async createAgent(request: AgentRequest["payload"]) {
    return await this.withTiming("CreateAgent", async () => {
      this.logger.debug("Creating agent", { agentId: request.agent_id });
      const model = this.createOpenAIProvider(request);
      const tools = await this.createAgentTools(request.tools);
      this.logger.info("Agent created", { agentId: request.agent_id });
      return new MastraAgent({
        name: request.agent_id,
        instructions: request.instructions,
        model,
        tools,
      });
    });
  }

  public async processRequest<T extends string | Record<string, any>>(
    type: RequestType,
    request: AgentRequest["payload"],
  ): Promise<AgentResponse<T>> {
    return await this.withTiming("ProcessRequest", async () => {
      try {
        this.logger.setCorrelationID(request.id);
        this.logger.debug("Processing agent request", {
          type,
          agentId: request.agent_id,
          requestId: request.id,
          toolCount: request.tools.length,
          config: {
            provider: request.config.provider,
            model: request.config.model,
          },
        });

        const agent = await this.createAgent(request);
        this.logger.info("Agent created", { agentId: request.agent_id });
        const outputSchemaZod = this.getOutputSchema(
          request.action.output_schema,
        );
        this.logger.debug("Output schema", { outputSchemaZod });

        let response: string;
        if (outputSchemaZod) {
          this.logger.debug("Generating response", { outputSchemaZod });
          const result = await agent.generate(
            [{ role: "user", content: request.action.prompt }],
            { output: outputSchemaZod },
          );
          this.logger.debug("Response generated", { result });
          response = result.object;
        } else {
          const result = await agent.generate([
            { role: "user", content: request.action.prompt },
          ]);
          response = result.text;
        }

        return {
          id: request.id,
          agent_id: request.agent_id,
          output: response as T,
          status: "Success",
        };
      } catch (error: unknown) {
        const errorMessage = error instanceof Error ? error.message : String(error);
        const stack = error instanceof Error ? error.stack : undefined;
        this.logger.error("Processing error", {
          error: errorMessage,
          stack,
          requestId: request.id,
        });
        throw error;
      }
    });
  }
}
