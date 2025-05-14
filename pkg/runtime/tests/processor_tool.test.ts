import { afterEach, beforeEach, describe, it } from "jsr:@std/testing/bdd";
import { stub } from "jsr:@std/testing/mock";
import { assertEquals, assertExists } from "jsr:@std/assert";
import { getRdnNamespace, restoreConsole, setupCapture, setupNats } from "./utils.ts";
import type { NatsClient } from "../src/nats_client.ts";
import { Logger } from "../src/logger.ts";
import { ToolProcessor } from "../src/processor_tool.ts";
import type { ToolRequest } from "../src/types.ts";

describe("ToolProcessor", () => {
  let client: NatsClient;
  let logger: Logger;
  const testExecId = "test-exec-id";
  const testNamespace = getRdnNamespace();

  beforeEach(async () => {
    setupCapture();
    client = await setupNats({
      namespace: testNamespace,
      execId: testExecId,
    });
    logger = new Logger({ verbose: false });
    logger.setClient(client);
  });

  afterEach(async () => {
    if (client && client.isConnected()) {
      await client.disconnect();
    }
    restoreConsole();
  });

  it("should process a tool request successfully", async () => {
    const processor = new ToolProcessor(logger, client, false);
    const request: ToolRequest = {
      id: "test-tool-request",
      tool_id: "echo_tool",
      description: "Echoes back the input message",
      input_schema: JSON.stringify({
        type: "object",
        properties: { message: { type: "string" } },
        required: ["message"],
      }),
      output_schema: JSON.stringify({
        type: "object",
        properties: { echo: { type: "string" } },
      }),
      input: { message: "hello world" },
    };

    const sendMessageStub = stub(client, "sendMessage", () => Promise.resolve());
    const response = await processor.processRequest("tool", request);
    assertExists(response);
    assertEquals(response.id, "test-tool-request");
    assertEquals(response.tool_id, "echo_tool");
    assertEquals(response.status, "Success");
    assertEquals(response.output, { echo: "hello world" });
    sendMessageStub.restore();
  });

  it("should handle tool execution errors", async () => {
    const processor = new ToolProcessor(logger, client, false);
    const request: ToolRequest = {
      id: "test-tool-request",
      tool_id: "invalid_tool",
      description: "Invalid tool",
      input: { message: "hello world" },
    };

    const sendMessageStub = stub(client, "sendMessage", () => Promise.resolve());
    const response = await processor.processRequest("tool", request);
    assertExists(response);
    assertEquals(response.id, "test-tool-request");
    assertEquals(response.tool_id, "invalid_tool");
    assertEquals(response.status, "Error");
    assertEquals(typeof response.output, "string");
    assertExists(response.output);
    sendMessageStub.restore();
  });

  it("should process request with no input", async () => {
    const processor = new ToolProcessor(logger, client, false);
    const request: ToolRequest = {
      id: "test-tool-request",
      tool_id: "echo_tool",
      description: "Echoes back the input message or a default",
      input_schema: JSON.stringify({
        type: "object",
        properties: { message: { type: "string" } },
      }),
      output_schema: JSON.stringify({
        type: "object",
        properties: { echo: { type: "string" } },
      }),
      input: null,
    };

    const sendMessageStub = stub(client, "sendMessage", () => Promise.resolve());
    const response = await processor.processRequest("tool", request);
    assertExists(response);
    assertEquals(response.id, "test-tool-request");
    assertEquals(response.tool_id, "echo_tool");
    assertEquals(response.status, "Success");
    assertExists(response.output);
    sendMessageStub.restore();
  });
});
