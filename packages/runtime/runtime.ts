import {
  AgentProcessor,
  type AgentRequest,
  IpcClient,
  Logger,
  type Processor,
  ToolProcessor,
  type ToolRequest,
} from "./src/index.ts";

async function main() {
  const ipcClient = new IpcClient();
  const logger = new Logger({ verbose: ipcClient.verbose });
  const agentProcessor = new AgentProcessor(logger, ipcClient);
  const toolProcessor = new ToolProcessor(logger, ipcClient);
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
      ipcClient.sendMessage("AgentResponse", response);
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
      ipcClient.sendMessage("ToolResponse", response);
      logger.debug("Successfully processed tool request");
    }
  } catch (error: any) {
    ipcClient.sendErrorMessage(
      agentRequestId || toolRequestId || null,
      error,
      {},
    );
    Deno.exit(1);
  } finally {
    processor.cleanup();
  }
}

if (import.meta.main) {
  main().catch((error) => {
    const ipcClient = new IpcClient();
    ipcClient.sendErrorMessage(null, error, {});
    Deno.exit(1);
  });
}
