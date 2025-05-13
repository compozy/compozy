import { afterEach, beforeEach, describe, it } from "jsr:@std/testing/bdd";
import { assertEquals, assertExists, assertRejects } from "jsr:@std/assert";
import { spy, stub } from "jsr:@std/testing/mock";
import { NatsClient, NatsClientError } from "../src/nats_client.ts";
import { LogLevel } from "../src/types.ts";
import { capturedOutput, getRdnNamespace, restoreConsole, setupCapture, setupNats } from "./utils.ts";

describe("NATS Integration", () => {
  let client: NatsClient;
  const testExecId = "test-exec-id";
  const testNamespace = getRdnNamespace();

  beforeEach(async () => {
    setupCapture();
    client = await setupNats({
      verbose: true,
      namespace: testNamespace,
      execId: testExecId,
    });
  });

  afterEach(async () => {
    await client.disconnect();
    restoreConsole();
  });

  it("should connect to NATS server", () => {
    assertExists(client.isConnected());
    assertEquals(capturedOutput.some(msg => {
      return msg?.includes("Connected to NATS server")
    }), true);
  });

  it("should generate correct subject patterns", () => {
    assertEquals(
      client.getSubjectPattern("AgentRequest", "test-agent"),
      `${testNamespace}.${testExecId}.agent.test-agent.request`,
    );
    assertEquals(
      client.getSubjectPattern("ToolResponse", "test-tool"),
      `${testNamespace}.${testExecId}.tool.test-tool.response`,
    );
    assertEquals(
      client.getSubjectPattern("Error"),
      `${testNamespace}.${testExecId}.error`,
    );
    assertEquals(
      client.getLogSubject(LogLevel.Info),
      `${testNamespace}.${testExecId}.log.info`,
    );
  });

  it("should send and receive agent messages", async () => {
    const payload = { result: "success" };
    const subject = client.getSubjectPattern("AgentResponse", "test-agent");
    const publishSpy = spy(client.getConnection()!, "publish");

    let receivedMessage: any = null;
    const unsubscribe = await client.subscribe("AgentResponse", "test-agent", (data) => {
      receivedMessage = data;
    });

    await client.sendMessage("AgentResponse", payload, "test-agent");
    await new Promise(resolve => setTimeout(resolve, 100)); // Wait for async delivery

    assertExists(receivedMessage);
    assertEquals(receivedMessage.result, "success");
    assertEquals(publishSpy.calls.length, 1);
    assertEquals(publishSpy.calls[0].args[0], subject);
    unsubscribe();
  });

  it("should send and receive tool messages", async () => {
    const payload = { result: "tool-success" };
    const subject = client.getSubjectPattern("ToolResponse", "test-tool");
    const publishSpy = spy(client.getConnection()!, "publish");

    let receivedMessage: any = null;
    const unsubscribe = await client.subscribe("ToolResponse", "test-tool", (data) => {
      receivedMessage = data;
    });

    await client.sendMessage("ToolResponse", payload, "test-tool");
    await new Promise(resolve => setTimeout(resolve, 100));

    assertExists(receivedMessage);
    assertEquals(receivedMessage.result, "tool-success");
    assertEquals(publishSpy.calls.length, 1);
    assertEquals(publishSpy.calls[0].args[0], subject);
    unsubscribe();
  });

  it("should send and receive log messages", async () => {
    const logLevel = LogLevel.Info;
    const subject = client.getLogSubject(logLevel);
    const publishSpy = spy(client.getConnection()!, "publish");

    let receivedLog: any = null;
    // Subscribe directly to the specific log level subject
    const subscription = client.getConnection()!.subscribe(subject);

    (async () => {
      try {
        for await (const msg of subscription) {
          const data = JSON.parse(new TextDecoder().decode(msg.data));
          receivedLog = data.payload;
          break; // Just get the first message
        }
      } catch (error) {
        console.error(`Error processing log message: ${String(error)}`);
      }
    })();

    await client.sendLogMessage(logLevel, "Test log message", { context: "testing" });
    await new Promise(resolve => setTimeout(resolve, 300));

    assertExists(receivedLog);
    assertEquals(receivedLog.level, logLevel);
    assertEquals(receivedLog.message, "Test log message");
    assertEquals(receivedLog.context.context, "testing");
    assertExists(receivedLog.timestamp);
    assertEquals(publishSpy.calls.length, 1);
    assertEquals(publishSpy.calls[0].args[0], subject);

    subscription.unsubscribe();
  });

  it("should send and receive error messages", async () => {
    const subject = client.getSubjectPattern("Error");
    const publishSpy = spy(client.getConnection()!, "publish");

    let receivedError: any = null;
    const unsubscribe = await client.subscribe("Error", "", (data) => {
      receivedError = data;
    });

    const error = new Error("Test error");
    await client.sendErrorMessage("test-request", error, { extra: "data" });
    await new Promise(resolve => setTimeout(resolve, 100));

    assertExists(receivedError);
    assertEquals(receivedError.message, "Test error");
    assertEquals(receivedError.request_id, "test-request");
    assertEquals(receivedError.data.extra, "data");
    assertExists(receivedError.stack);
    assertEquals(publishSpy.calls.length, 1);
    assertEquals(publishSpy.calls[0].args[0], subject);
    unsubscribe();
  });

  it("should handle request/response pattern", async () => {
    const responderClient = new NatsClient({
      verbose: true,
      namespace: testNamespace,
      serverUrl: `nats://localhost:4222`,
      execId: testExecId,
    });
    await responderClient.connect();
    const requestSubject = client.getSubjectPattern("TestRequest", "test-id");
    const requestSpy = spy(client.getConnection()!, "request");
    const subscription = responderClient.getConnection()!.subscribe(requestSubject);
    const responderReady = new Promise<void>((resolve) => {
      (async () => {
        for await (const msg of subscription) {
          const request = JSON.parse(new TextDecoder().decode(msg.data));
          msg.respond(JSON.stringify({
            type: "Response",
            payload: { success: true, received: request.payload.data },
          }));
          break;
        }
      })();
      // Give the responder a moment to be ready
      setTimeout(resolve, 100);
    });

    // Wait for responder to be ready
    await responderReady;
    const response = await client.request<{ data: string }, { success: boolean, received: string }>(
      "TestRequest",
      "test-id",
      { data: "test" },
    );

    assertEquals(response.success, true);
    assertEquals(response.received, "test");
    assertEquals(requestSpy.calls.length, 1);

    subscription.unsubscribe();
    await responderClient.disconnect();
  });

  it("should fail to send message when not connected", async () => {
    const disconnectedClient = new NatsClient({
      namespace: testNamespace,
      serverUrl: `nats://localhost:9999`,
      execId: testExecId,
    });
    // Don't connect the client

    await assertRejects(
      () => disconnectedClient.sendMessage("AgentResponse", { result: "fail" }, "test-agent"),
      NatsClientError,
      "Failed to connect to NATS",
    );
  });

  it("should format errors correctly", async () => {
    const sendMessageStub = stub(client, "sendMessage", (type: string, payload: any) => {
      capturedOutput.push(JSON.stringify({ type, payload }));
      return Promise.resolve();
    });
    const error = new Error("Test error");
    await client.sendErrorMessage("test-request", error, { input: "test" });

    // Find the JSON string in the output
    const jsonOutput = capturedOutput.find(output => {
      try {
        const parsed = JSON.parse(output);
        return parsed.type === "Error";
      } catch (_) {
        return false;
      }
    });

    assertExists(jsonOutput);
    const errorData = JSON.parse(jsonOutput);
    assertEquals(errorData.type, "Error");
    assertEquals(errorData.payload.request_id, "test-request");
    assertEquals(errorData.payload.message, "Test error");
    assertExists(errorData.payload.stack);
    assertEquals(errorData.payload.data.input, "test");
    sendMessageStub.restore();
  });

  it("should handle errors without message", async () => {
    const sendMessageStub = stub(client, "sendMessage", (type: string, payload: any) => {
      capturedOutput.push(JSON.stringify({ type, payload }));
      return Promise.resolve();
    });
    const error = {};
    await client.sendErrorMessage("test-request", error, {});

    // Find the JSON string in the output
    const jsonOutput = capturedOutput.find(output => {
      try {
        const parsed = JSON.parse(output);
        return parsed.type === "Error";
      } catch (_) {
        return false;
      }
    });

    assertExists(jsonOutput);
    const errorData = JSON.parse(jsonOutput);
    assertEquals(errorData.payload.message, "Unknown error");
    sendMessageStub.restore();
  });

});
