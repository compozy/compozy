// tests/agent.test.ts
import { afterEach, beforeEach, describe, it } from "jsr:@std/testing/bdd";
import { stub } from "jsr:@std/testing/mock";
import { assertEquals, assertExists } from "jsr:@std/assert";
import { IpcClient } from "../src/ipc_client.ts";
import { Logger } from "../src/logger.ts";
import { AgentProcessor } from "../src/processor_agent.ts";
import type { AgentRequest } from "../src/types.ts";
import { restoreConsole, setupCapture } from "./utils.ts";

describe("AgentProcessor", () => {
  beforeEach(() => {
    setupCapture();
  });

  afterEach(() => {
    restoreConsole();
  });

  it("should create tools and process request", async () => {
    const ipcClient = new IpcClient();
    const logger = new Logger({ verbose: false });
    const processor = new AgentProcessor(logger, ipcClient, false);
    const request: AgentRequest["payload"] = {
      id: "test-request",
      agent_id: "test-agent",
      instructions: "Echo the message without calling OpenAI",
      action: {
        id: "echo_action",
        prompt: "Echo this: hello world",
        output_schema: JSON.stringify({
          type: "object",
          properties: { echo: { type: "string" } },
        }),
      },
      config: {
        provider: "groq",
        model: "llama-3.1-8b-instant",
        api_key: "test-key",
      },
      tools: [
        {
          id: "1",
          tool_id: "echo_tool",
          input_schema: JSON.stringify({
            type: "object",
            properties: { message: { type: "string" } },
            required: ["message"],
          }),
          output_schema: JSON.stringify({
            type: "object",
            properties: { echo: { type: "string" } },
          }),
          description: "Echoes back the input message",
        },
      ],
    };

    const sendMessageStub = stub(ipcClient, "sendMessage", () => {});
    // deno-lint-ignore require-await
    const processRequestStub = stub(processor, "processRequest", async () => ({
      id: "test-request",
      agent_id: "test-agent",
      output: { echo: "hello world" },
      status: "Success" as const,
    }));
    const response = await processor.processRequest("agent", request);
    assertExists(response.output);
    assertEquals((response.output as any).echo, "hello world");

    sendMessageStub.restore();
    processRequestStub.restore();
  });

  it("should process request with agent and tools", async () => {
    const apiKey = Deno.env.get("OPENAI_API_KEY");
    if (!apiKey) {
      console.log(
        "Skipping request processing test: No OpenAI API key available",
      );
      return;
    }

    const ipcClient = new IpcClient();
    const logger = new Logger({ verbose: false });
    const processor = new AgentProcessor(logger, ipcClient, false);
    const request: AgentRequest["payload"] = {
      id: "test-request",
      agent_id: "test-agent",
      instructions:
        `You are a helpful weather assistant that provides accurate weather information.
Your primary function is to help users get weather details for specific locations.
When responding:
- Keep responses concise but informative
- Format temperature according to the user's preferred units (celsius or fahrenheit)
- Include humidity information when available
- Suggest appropriate activities based on the weather conditions
Use the weather_tool to fetch current weather data.`,
      action: {
        id: "get_current_weather",
        prompt:
          "What is the weather in New York? Please provide the temperature in celsius.",
        output_schema: JSON.stringify({
          type: "object",
          properties: {
            weather: {
              type: "string",
              description: "Current weather conditions",
            },
            temperature: { type: "number", description: "Current temperature" },
            humidity: {
              type: "number",
              description: "Current humidity percentage",
            },
          },
          required: ["weather", "temperature", "humidity"],
        }),
      },
      config: {
        provider: "groq",
        model: "llama-3.3-70b-versatile",
        api_key: apiKey,
      },
      tools: [
        {
          id: "tool_id",
          tool_id: "weather_tool",
          input_schema: JSON.stringify({
            type: "object",
            properties: { city: { type: "string" } },
            required: ["city"],
          }),
          output_schema: JSON.stringify({
            type: "object",
            properties: {
              weather: { type: "string" },
              temperature: { type: "number" },
              humidity: { type: "number" },
            },
            required: ["weather", "temperature", "humidity"],
          }),
          description: "Get the current weather for a specific location",
        },
      ],
    };

    type OutputAction = {
      weather: string;
      temperature: number;
      humidity: number;
    };
    const response = await processor.processRequest<OutputAction>(
      "agent",
      request,
    );
    assertExists(response);
    assertEquals(response.id, "test-request");
    assertEquals(response.agent_id, "test-agent");
    assertEquals(response.status, "Success");
    assertExists(response.output.weather);
    assertExists(response.output.temperature);
    assertExists(response.output.humidity);
  });
});
