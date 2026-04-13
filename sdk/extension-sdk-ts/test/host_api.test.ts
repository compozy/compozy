import { describe, expect, it } from "vitest";

import { Extension } from "../src/extension.js";
import { RPCError } from "../src/transport.js";
import { CAPABILITIES, HOOKS, PROTOCOL_VERSION } from "../src/types.js";
import { TestHarness } from "../src/testing/test_harness.js";
import { createMockTransportPair } from "../src/testing/mock_transport.js";

describe("HostAPI", () => {
  it("round-trips host.tasks.create through the test harness", async () => {
    const extension = new Extension("sdk-ext", "1.0.0").withCapabilities(CAPABILITIES.tasksCreate);
    const harness = new TestHarness({
      granted_capabilities: [CAPABILITIES.tasksCreate],
    });
    harness.handleHostMethod("host.tasks.create", async params => {
      expect(params).toEqual({
        workflow: "demo",
        title: "Hello",
        body: "Body",
        frontmatter: { status: "pending", type: "docs" },
      });
      return {
        workflow: "demo",
        number: 7,
        path: ".compozy/tasks/demo/task_07.md",
        status: "pending",
      };
    });

    const runPromise = harness.run(extension);
    await harness.initialize({
      name: "sdk-ext",
      version: "1.0.0",
      source: "workspace",
    });

    await expect(
      extension.host.tasks.create({
        workflow: "demo",
        title: "Hello",
        body: "Body",
        frontmatter: { status: "pending", type: "docs" },
      })
    ).resolves.toMatchObject({ number: 7 });

    await harness.shutdown({ reason: "run_completed", deadline_ms: 1000 });
    await expect(runPromise).resolves.toBeUndefined();
  });

  it("returns a run id from host.runs.start", async () => {
    const extension = new Extension("sdk-ext", "1.0.0").withCapabilities(CAPABILITIES.runsStart);
    const harness = new TestHarness({
      granted_capabilities: [CAPABILITIES.runsStart],
    });
    harness.handleHostMethod("host.runs.start", async () => ({
      run_id: "run-child-001",
      parent_run_id: "run-parent-001",
    }));

    const runPromise = harness.run(extension);
    await harness.initialize({
      name: "sdk-ext",
      version: "1.0.0",
      source: "workspace",
    });

    await expect(
      extension.host.runs.start({
        runtime: { name: "child" },
      })
    ).resolves.toEqual({
      run_id: "run-child-001",
      parent_run_id: "run-parent-001",
    });

    await harness.shutdown({ reason: "run_completed", deadline_ms: 1000 });
    await expect(runPromise).resolves.toBeUndefined();
  });

  it("returns absent workflow memory documents", async () => {
    const extension = new Extension("sdk-ext", "1.0.0").withCapabilities(CAPABILITIES.memoryRead);
    const harness = new TestHarness({
      granted_capabilities: [CAPABILITIES.memoryRead],
    });
    harness.handleHostMethod("host.memory.read", async () => ({
      path: ".compozy/tasks/demo/memory/task_03.md",
      content: "",
      exists: false,
      needs_compaction: false,
    }));

    const runPromise = harness.run(extension);
    await harness.initialize({
      name: "sdk-ext",
      version: "1.0.0",
      source: "workspace",
    });

    await expect(
      extension.host.memory.read({
        workflow: "demo",
        task_file: "task_03.md",
      })
    ).resolves.toEqual({
      path: ".compozy/tasks/demo/memory/task_03.md",
      content: "",
      exists: false,
      needs_compaction: false,
    });

    await harness.shutdown({ reason: "run_completed", deadline_ms: 1000 });
    await expect(runPromise).resolves.toBeUndefined();
  });

  it("correlates out-of-order host responses by id", async () => {
    const extension = new Extension("sdk-ext", "1.0.0").withCapabilities(
      CAPABILITIES.tasksRead,
      CAPABILITIES.memoryRead
    );
    const [extensionTransport, hostTransport] = createMockTransportPair();
    const runPromise = extension.withTransport(extensionTransport).start();

    await hostTransport.writeMessage({
      id: 1,
      method: "initialize",
      params: {
        protocol_version: PROTOCOL_VERSION,
        supported_protocol_versions: [PROTOCOL_VERSION],
        compozy_version: "dev",
        extension: {
          name: "sdk-ext",
          version: "1.0.0",
          source: "workspace",
        },
        granted_capabilities: [CAPABILITIES.tasksRead, CAPABILITIES.memoryRead],
        runtime: {
          run_id: "run-1",
          workspace_root: ".",
          invoking_command: "start",
          shutdown_timeout_ms: 1000,
          default_hook_timeout_ms: 5000,
        },
      },
    });

    const initializeResponse = await hostTransport.readMessage();
    expect(initializeResponse.result).toMatchObject({
      protocol_version: PROTOCOL_VERSION,
    });

    const tasksPromise = extension.host.tasks.list({ workflow: "demo" });
    const memoryPromise = extension.host.memory.read({ workflow: "demo" });

    const first = await hostTransport.readMessage();
    const second = await hostTransport.readMessage();

    const memoryRequest = first.method === "host.memory.read" ? first : second;
    const taskRequest = first.method === "host.tasks.list" ? first : second;

    await hostTransport.writeMessage({
      id: memoryRequest.id,
      result: {
        path: ".compozy/tasks/demo/memory/MEMORY.md",
        content: "",
        exists: false,
        needs_compaction: false,
      },
    });
    await hostTransport.writeMessage({
      id: taskRequest.id,
      result: [{ workflow: "demo", number: 1, path: "task_01.md", status: "pending" }],
    });

    await expect(memoryPromise).resolves.toMatchObject({ exists: false });
    await expect(tasksPromise).resolves.toMatchObject([{ number: 1 }]);

    await hostTransport.writeMessage({
      id: 2,
      method: "shutdown",
      params: { reason: "run_completed", deadline_ms: 1000 },
    });
    const shutdownResponse = await hostTransport.readMessage();
    expect(shutdownResponse.result).toEqual({ acknowledged: true });

    await expect(runPromise).resolves.toBeUndefined();
  });

  it("returns structured errors from the host", async () => {
    const extension = new Extension("sdk-ext", "1.0.0").withCapabilities(
      CAPABILITIES.artifactsWrite
    );
    const harness = new TestHarness({
      granted_capabilities: [CAPABILITIES.artifactsWrite],
    });
    harness.handleHostMethod("host.artifacts.write", async () => {
      throw new RPCError(-32001, "Capability denied", { reason: "path_out_of_scope" });
    });

    const runPromise = harness.run(extension);
    await harness.initialize({
      name: "sdk-ext",
      version: "1.0.0",
      source: "workspace",
    });

    await expect(
      extension.host.artifacts.write({
        path: "../bad.txt",
        content: "bad",
      })
    ).rejects.toMatchObject<RPCError>({
      code: -32001,
      data: { reason: "path_out_of_scope" },
    });

    await harness.shutdown({ reason: "run_completed", deadline_ms: 1000 });
    await expect(runPromise).resolves.toBeUndefined();
  });

  it("rejects pending calls when the harness transport terminates", async () => {
    const extension = new Extension("sdk-ext", "1.0.0").onPromptPostBuild(
      () => new Promise<never>(() => {})
    );
    const harness = new TestHarness({
      granted_capabilities: [CAPABILITIES.promptMutate],
    });

    void harness.run(extension).catch(() => undefined);
    await harness.initialize({
      name: "sdk-ext",
      version: "1.0.0",
      source: "workspace",
    });

    const pending = harness.dispatchHook(
      "hook-pending",
      {
        name: HOOKS.promptPostBuild,
        event: HOOKS.promptPostBuild,
        mutable: true,
        required: false,
        priority: 500,
        timeout_ms: 5000,
      },
      { prompt_text: "original" }
    );

    await harness.extensionTransport.close();

    await expect(pending).rejects.toThrow("test harness terminated");
  });
});
