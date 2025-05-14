import {
  AgentProcessor,
  type AgentRequest,
  Logger,
  type Processor,
  ToolProcessor,
  type ToolRequest,
  NatsClient,
} from "./src/index.ts";

async function main() {
  const natsClient = new NatsClient();
  await natsClient.connect();

  const logger = new Logger({ verbose: natsClient.verbose });
  const agentProcessor = new AgentProcessor(logger, natsClient);
  const toolProcessor = new ToolProcessor(logger, natsClient);

  const { id: agentId, requestId: agentRequestId } = agentProcessor
    .parseCommandLineArgs("agent");
  const { id: toolId, requestId: toolRequestId } = toolProcessor
    .parseCommandLineArgs("tool");

  logger.info("Starting runtime execution");
  const processor: Processor = toolId ? toolProcessor : agentProcessor;

  if (!agentId && !toolId) {
    logger.error("Agent ID or Tool ID is required");
    Deno.exit(1);
  }

  try {
    if (agentId) {
      const request = await agentProcessor.readRequestFromStdin<
        AgentRequest["payload"]
      >("AgentRequest");
      logger.info(`Executing agent: ${agentId}`, { requestId: agentRequestId });
      const response = await agentProcessor.processRequest<
        string | Record<string, any>
      >(
        "agent",
        request.payload,
      );
      await natsClient.sendMessage("AgentResponse", response, agentId);
      logger.debug("Successfully processed agent request");
    }
    if (toolId) {
      const request = await toolProcessor.readRequestFromStdin<ToolRequest>(
        "ToolRequest",
      );
      logger.info(`Executing tool: ${toolId}`, { requestId: toolRequestId });
      const response = await toolProcessor.processRequest<unknown>(
        "tool",
        request.payload,
      );
      await natsClient.sendMessage("ToolResponse", response, toolId);
      logger.debug("Successfully processed tool request");
    }
  } catch (error: unknown) {
    await natsClient.sendErrorMessage(
      agentRequestId || toolRequestId || null,
      error,
      {},
    );
    Deno.exit(1);
  } finally {
    processor.cleanup();
    await natsClient.disconnect();
  }
}

if (import.meta.main) {
  main().catch(async (error) => {
    const natsClient = new NatsClient();
    await natsClient.connect();
    await natsClient.sendErrorMessage(null, error, {});
    await natsClient.disconnect();
    Deno.exit(1);
  });
}
