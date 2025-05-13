// tests/ipc.test.ts
import { afterEach, beforeEach, describe, it } from "jsr:@std/testing/bdd";
import { stub } from "jsr:@std/testing/mock";
import { assertEquals, assertExists } from "jsr:@std/assert";
import { IpcClient } from "../src/ipc_client.ts";
import { capturedOutput, restoreConsole, setupCapture } from "./utils.ts";

describe("IpcClient Error Handling", () => {
  beforeEach(() => {
    setupCapture();
  });

  afterEach(() => {
    restoreConsole();
  });

  it("should format errors correctly", () => {
    const ipcClient = new IpcClient();
    const sendMessageStub = stub(ipcClient, "sendMessage", (type, payload) => {
      capturedOutput.push(JSON.stringify({ type, payload }));
    });
    const error = new Error("Test error");
    ipcClient.sendErrorMessage("test-request", error, { input: "test" });
    assertEquals(capturedOutput.length, 1);
    const errorData = JSON.parse(capturedOutput[0]);
    assertEquals(errorData.type, "Error");
    assertEquals(errorData.payload.request_id, "test-request");
    assertEquals(errorData.payload.message, "Test error");
    assertExists(errorData.payload.stack);
    assertEquals(errorData.payload.data.input, "test");
    sendMessageStub.restore();
  });

  it("should handle errors without message", () => {
    const ipcClient = new IpcClient();
    const sendMessageStub = stub(ipcClient, "sendMessage", (type, payload) => {
      capturedOutput.push(JSON.stringify({ type, payload }));
    });
    const error = {};
    ipcClient.sendErrorMessage("test-request", error, {});
    assertEquals(capturedOutput.length, 1);
    const errorData = JSON.parse(capturedOutput[0]);
    assertEquals(errorData.payload.message, "Unknown error");
    sendMessageStub.restore();
  });
});
