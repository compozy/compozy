// tests/logger.test.ts
import { afterEach, beforeEach, describe, it } from "jsr:@std/testing/bdd";
import { stub } from "jsr:@std/testing/mock";
import { assertEquals, assertExists } from "jsr:@std/assert";
import { IpcClient, LogLevel } from "../src/index.ts";
import { capturedOutput, restoreConsole, setupCapture } from "./utils.ts";
import { Logger } from "../src/logger.ts";

describe("Logger", () => {
  beforeEach(() => {
    setupCapture();
  });

  afterEach(() => {
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
    const ipcClient = new IpcClient();
    const sendMessageStub = stub(ipcClient, "sendMessage", (type, payload) => {
      capturedOutput.push(JSON.stringify({ type, payload }));
    });
    logger.info("Test message", { test: true });

    assertEquals(capturedOutput.length, 1);
    const logData = JSON.parse(capturedOutput[0]);
    assertEquals(logData.type, "Log");
    assertEquals(logData.payload.level, LogLevel.Info);
    assertEquals(logData.payload.message, "Test message");
    assertEquals(logData.payload.context?.test, true);
    assertExists(logData.payload.timestamp);

    sendMessageStub.restore();
  });

  it("should not log debug messages when verbose is false", () => {
    const logger = new Logger({ verbose: false });
    const ipcClient = new IpcClient(false);
    const sendMessageStub = stub(ipcClient, "sendMessage", () => {
      capturedOutput.push("unexpected");
    });
    logger.debug("Debug message");
    assertEquals(capturedOutput.length, 0);
    sendMessageStub.restore();
  });

  it("should log debug messages when verbose is true", () => {
    const ipcClient = new IpcClient();
    const logger = new Logger({ verbose: true });
    const sendMessageStub = stub(ipcClient, "sendMessage", (type, payload) => {
      capturedOutput.push(JSON.stringify({ type, payload }));
    });
    logger.debug("Debug message");
    assertEquals(capturedOutput.length, 1);
    const logData = JSON.parse(capturedOutput[0]);
    assertEquals(logData.payload.level, LogLevel.Debug);
    sendMessageStub.restore();
  });
});
