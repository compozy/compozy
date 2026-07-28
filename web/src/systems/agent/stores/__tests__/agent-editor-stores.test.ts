import { describe, expect, it, vi } from "vitest";

import { primaryAgentFixture } from "@/systems/agent/testing";

import type { AgentSoulPayload } from "../../types";
import {
  createAuthoredFileEditorLogic,
  shouldAdoptAuthoredFileSource,
} from "../agent-authored-file-editor-store";
import {
  createAgentSettingsEditorLogic,
  shouldAdoptAgentSettingsSource,
} from "../agent-settings-editor-store";

const soulPayload = (overrides: Partial<AgentSoulPayload> = {}): AgentSoulPayload => ({
  active: true,
  present: true,
  enabled: true,
  valid: true,
  validation_status: "valid",
  digest: "digest-a",
  source_path: "agents/coder/SOUL.md",
  frontmatter: { version: "1", role: "coder", tone: [], principles: [] },
  body: "# Soul\n",
  limits: { max_body_bytes: 65536 },
  config_provenance: {
    digest: "config-digest",
    enabled: true,
    max_body_bytes: 65536,
    context_projection_bytes: 0,
  },
  ...overrides,
});

describe("agent editor store logic", () => {
  it("Should retain a dirty authored-file baseline and ignore stale validation results", () => {
    const validate = vi.fn(() => new Promise<never>(() => undefined));
    const store = createAuthoredFileEditorLogic().createStore({
      resourceKey: "agent-a:soul",
      kind: "soul",
      payload: soulPayload(),
    });

    store.trigger.draftChanged({ draft: "# local\n" });
    let snapshot = store.getSnapshot();
    expect(
      shouldAdoptAuthoredFileSource(snapshot.context, {
        resourceKey: "agent-a:soul",
        kind: "soul",
        payload: soulPayload({ digest: "digest-b", body: "# remote\n" }),
      })
    ).toBe(false);
    expect(snapshot.context.draft).toBe("# local\n");
    expect(snapshot.context.baseline.digest).toBe("digest-a");

    expect(store.can.validationRequested({ validate })).toBe(true);
    expect(validate).not.toHaveBeenCalled();
    store.trigger.validationRequested({ validate });
    store.trigger.validationRequested({ validate });
    expect(validate).toHaveBeenCalledOnce();
    snapshot = store.getSnapshot();
    const requestId = snapshot.context.requestId;
    store.trigger.validationSucceeded({
      resourceKey: "agent-b:soul",
      requestId,
      diagnostics: [{ message: "other resource" }],
      validationStatus: "invalid",
    });
    snapshot = store.getSnapshot();
    expect(snapshot.context.phase).toBe("validating");
    store.trigger.validationSucceeded({
      resourceKey: "agent-a:soul",
      requestId: requestId - 1,
      diagnostics: [{ message: "stale" }],
      validationStatus: "invalid",
    });
    snapshot = store.getSnapshot();
    expect(snapshot.context.phase).toBe("validating");
  });

  it("Should own authored-file validation, save, and restore effects", async () => {
    const validate = vi.fn().mockResolvedValue({
      diagnostics: [{ message: "valid" }],
      validation_status: "valid",
    });
    const save = vi.fn().mockResolvedValue(soulPayload({ digest: "digest-b", body: "# saved\n" }));
    const restore = vi
      .fn()
      .mockResolvedValue(soulPayload({ digest: "digest-c", body: "# restored\n" }));
    const store = createAuthoredFileEditorLogic().createStore({
      resourceKey: "agent-a:soul",
      kind: "soul",
      payload: soulPayload(),
    });

    store.trigger.draftChanged({ draft: "# local\n" });
    store.trigger.validationRequested({ validate });
    await vi.waitFor(() => expect(store.getSnapshot().context.phase).toBe("ready"));
    expect(validate).toHaveBeenCalledWith("# local\n");
    expect(store.getSnapshot().context.diagnostics).toEqual([{ message: "valid" }]);

    store.trigger.saveRequested({ save });
    store.trigger.saveRequested({ save });
    expect(save).toHaveBeenCalledOnce();
    expect(save).toHaveBeenCalledWith("# local\n", "digest-a");
    await vi.waitFor(() => expect(store.getSnapshot().context.phase).toBe("ready"));
    expect(store.getSnapshot().context.baseline.digest).toBe("digest-b");
    expect(
      shouldAdoptAuthoredFileSource(store.getSnapshot().context, {
        resourceKey: "agent-a:soul",
        kind: "soul",
        payload: soulPayload(),
      })
    ).toBe(false);

    store.trigger.restoreRequested({ restore, revisionId: "revision-1" });
    store.trigger.restoreRequested({ restore, revisionId: "revision-1" });
    expect(restore).toHaveBeenCalledOnce();
    expect(restore).toHaveBeenCalledWith("revision-1", "digest-b");
    await vi.waitFor(() => expect(store.getSnapshot().context.phase).toBe("ready"));
    expect(store.getSnapshot().context.baseline.digest).toBe("digest-c");
  });

  it("Should adopt diagnostics-only authored-file updates while the draft is clean", () => {
    const store = createAuthoredFileEditorLogic().createStore({
      resourceKey: "agent-a:soul",
      kind: "soul",
      payload: soulPayload({
        diagnostics: [{ code: "first", message: "First diagnostic", line: 2, severity: "warning" }],
      }),
    });

    expect(
      shouldAdoptAuthoredFileSource(store.getSnapshot().context, {
        resourceKey: "agent-a:soul",
        kind: "soul",
        payload: soulPayload({
          diagnostics: [
            { code: "updated", message: "Updated diagnostic", line: 4, severity: "error" },
          ],
        }),
      })
    ).toBe(true);
  });

  it("Should identify the authored file and operation in fallback failures", async () => {
    const save = vi.fn().mockRejectedValue(new Error(" "));
    const validate = vi.fn().mockRejectedValue(new Error(" "));
    const store = createAuthoredFileEditorLogic().createStore({
      resourceKey: "agent-a:soul",
      kind: "soul",
      payload: soulPayload({ present: false, validation_status: "missing" }),
    });

    store.trigger.createRequested({ body: "# Soul\n", save });
    await vi.waitFor(() => expect(store.getSnapshot().context.phase).toBe("failed"));
    expect(store.getSnapshot().context).toMatchObject({
      operation: "create",
      error: "Couldn't create SOUL.md",
    });

    store.trigger.validationRequested({ validate });
    await vi.waitFor(() =>
      expect(store.getSnapshot().context).toMatchObject({
        phase: "failed",
        operation: "validate",
        error: "Couldn't validate SOUL.md",
      })
    );
  });

  it("Should preserve a dirty agent-settings source and fence stale save completions", () => {
    const agent = { ...primaryAgentFixture, origin: "workspace" as const };
    const save = vi.fn(() => new Promise<never>(() => undefined));
    const store = createAgentSettingsEditorLogic().createStore({
      resourceKey: "ws-test:coder",
      agent,
    });

    store.trigger.draftPatched({ patch: { prompt: "local prompt" } });
    let snapshot = store.getSnapshot();
    expect(snapshot.context.draft?.prompt).toBe("local prompt");
    expect(snapshot.context.baseline?.definition_digest).toBe(agent.definition_digest);
    expect(
      shouldAdoptAgentSettingsSource(snapshot.context, {
        ...agent,
        definition_digest: "remote-digest",
        prompt: "remote prompt",
      })
    ).toBe(false);

    store.trigger.saveRequested({ name: agent.name, save, workspaceId: "ws-test" });
    snapshot = store.getSnapshot();
    const requestId = snapshot.context.requestId;
    store.trigger.saveSucceeded({
      requestId: requestId - 1,
      agent: { ...agent, prompt: "local prompt" },
    });
    snapshot = store.getSnapshot();
    expect(snapshot.context.phase).toBe("saving");
  });

  it("Should own save and reload effects without side effects from can probes or duplicate intents", async () => {
    const agent = { ...primaryAgentFixture, origin: "workspace" as const };
    const saved = { ...agent, prompt: "local prompt", definition_digest: "saved-digest" };
    const reloaded = { ...saved, prompt: "reloaded prompt", definition_digest: "reload-digest" };
    const save = vi.fn().mockResolvedValue(saved);
    const reload = vi.fn().mockResolvedValue(reloaded);
    const store = createAgentSettingsEditorLogic().createStore({
      resourceKey: "ws-test:coder",
      agent,
    });

    store.trigger.draftPatched({ patch: { prompt: "local prompt" } });
    expect(store.can.saveRequested({ name: agent.name, save, workspaceId: "ws-test" })).toBe(true);
    expect(save).not.toHaveBeenCalled();

    store.trigger.saveRequested({ name: agent.name, save, workspaceId: "ws-test" });
    store.trigger.saveRequested({ name: agent.name, save, workspaceId: "ws-test" });
    expect(save).toHaveBeenCalledOnce();
    expect(save).toHaveBeenCalledWith(
      expect.objectContaining({
        cacheWorkspace: "ws-test",
        name: agent.name,
        params: expect.objectContaining({ expected_digest: agent.definition_digest }),
      })
    );
    await vi.waitFor(() => expect(store.getSnapshot().context.phase).toBe("ready"));
    expect(store.getSnapshot().context.draft?.prompt).toBe("local prompt");

    store.trigger.reloadRequested({ reload });
    store.trigger.reloadRequested({ reload });
    expect(reload).toHaveBeenCalledOnce();
    await vi.waitFor(() => expect(store.getSnapshot().context.phase).toBe("ready"));
    expect(store.getSnapshot().context.draft?.prompt).toBe("reloaded prompt");
  });
});
