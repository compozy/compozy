import { afterEach, beforeEach, describe, it } from "jsr:@std/testing/bdd";
import { stub } from "jsr:@std/testing/mock";
import { assertEquals, assertExists } from "jsr:@std/assert";
import type { NatsClient } from "../src/nats_client.ts";
import { LogLevel } from "../src/types.ts";
import { capturedOutput, getRdnNamespace, restoreConsole, setupCapture, setupNats } from "./utils.ts";
import { Logger } from "../src/logger.ts";

describe("Logger", () => {
  let natsClient: NatsClient;
  const testNamespace = getRdnNamespace();
  const testExecId = "test-exec-id";

  beforeEach(async () => {
    setupCapture();
    natsClient = await setupNats({
      namespace: testNamespace,
      execId: testExecId,
    });
  });

  afterEach(async () => {
    if (natsClient && natsClient.isConnected()) {
      await natsClient.disconnect();
    }
    restoreConsole();
  });

  it("should have correct methods", () => {
    const logger = new Logger();
    assertExists(logger.debug);
    assertExists(logger.info);
    assertExists(logger.warn);
    assertExists(logger.error);
  });

  it("should format messages correctly", () => {
    const logger = new Logger();
    const sendLogMessageStub = stub(natsClient, "sendLogMessage", (level, message, context) => {
      capturedOutput.push(JSON.stringify({ level, message, context }));
      return Promise.resolve();
    });

    logger.setClient(natsClient);
    logger.info("Test message", { test: true });

    assertEquals(capturedOutput.length, 1);
    const logData = JSON.parse(capturedOutput[0]);
    assertEquals(logData.level, LogLevel.Info);
    assertEquals(logData.message, "Test message");
    assertEquals(logData.context?.test, true);

    sendLogMessageStub.restore();
  });

  it("should not log debug messages when verbose is false", () => {
    const logger = new Logger({ verbose: false });
    const sendLogMessageStub = stub(natsClient, "sendLogMessage", () => {
      capturedOutput.push("unexpected");
      return Promise.resolve();
    });

    logger.setClient(natsClient);
    logger.debug("Debug message");
    assertEquals(capturedOutput.length, 0);
    sendLogMessageStub.restore();
  });

  it("should log debug messages when verbose is true", () => {
    const logger = new Logger({ verbose: true });
    const sendLogMessageStub = stub(natsClient, "sendLogMessage", (level, message, context) => {
      capturedOutput.push(JSON.stringify({ level, message, context }));
      return Promise.resolve();
    });

    logger.setClient(natsClient);
    logger.debug("Debug message");
    assertEquals(capturedOutput.length, 1);
    const logData = JSON.parse(capturedOutput[0]);
    assertEquals(logData.level, LogLevel.Debug);
    sendLogMessageStub.restore();
  });
});
