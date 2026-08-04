import { describe, expect, it } from "vitest";

import { getSessionPromptRuntimeSnapshot } from "../../hooks/use-session-prompt-runtime";
import {
  sessionPromptRuntimeInput,
  sessionPromptRuntimeStoreLogic,
} from "../session-prompt-runtime-store";
import { primarySessionFixture } from "../../mocks/fixtures";

function primaryRuntimeInput(canPrompt = true) {
  return sessionPromptRuntimeInput({
    agentName: primarySessionFixture.agent_name,
    canPrompt,
    effectiveRuntime: primarySessionFixture.runtime.effective,
    selectedRuntime: primarySessionFixture.runtime.selected,
    selectionRevision: primarySessionFixture.runtime.selection_revision,
    sessionId: primarySessionFixture.id,
    workspaceId: primarySessionFixture.workspace_id,
  });
}

describe("sessionPromptRuntimeStore", () => {
  // Invariant: a selected runtime remains the next-prompt intent when the
  // server-sourced default later reconciles. Owning layer: interaction-store
  // unit. Canonical suite: this file.
  it("keeps the selected runtime when the default runtime changes", () => {
    const store = sessionPromptRuntimeStoreLogic.createStore(primaryRuntimeInput());
    store.trigger.runtimeSelected({
      value: { model: "claude-opus", provider: "anthropic", reasoning_effort: "high" },
    });
    store.trigger.speedSelected({ speed: "fast" });
    store.trigger.defaultRuntimeResolved({
      speed: "normal",
      value: { model: "gpt-5", provider: "openai", reasoning_effort: "medium" },
    });

    expect(getSessionPromptRuntimeSnapshot(store)).toEqual({
      model: "claude-opus",
      provider: "anthropic",
      reasoning_effort: "high",
      speed: "fast",
    });
  });

  it("uses the resolved default until a runtime is selected", () => {
    const store = sessionPromptRuntimeStoreLogic.createStore(primaryRuntimeInput());
    store.trigger.defaultRuntimeResolved({
      speed: "normal",
      value: { model: "gpt-5", provider: "openai", reasoning_effort: "medium" },
    });

    expect(getSessionPromptRuntimeSnapshot(store)).toEqual({
      model: "gpt-5",
      provider: "openai",
      reasoning_effort: "medium",
    });
  });

  it("uses and reconciles the durable server selection across revisions", () => {
    const input = primaryRuntimeInput();
    const store = sessionPromptRuntimeStoreLogic.createStore({
      ...input,
      selectedRuntime: {
        provider: "claude",
        model: "claude-fable-5",
        reasoning_effort: "max",
      },
      selectionRevision: 3,
    });

    expect(getSessionPromptRuntimeSnapshot(store)).toEqual({
      provider: "claude",
      model: "claude-fable-5",
      reasoning_effort: "max",
    });

    store.trigger.runtimePersisted({
      revision: 4,
      runtime: { provider: "codex", model: "gpt-5.6-terra", reasoning_effort: "high" },
      sessionId: input.sessionId,
      workspaceId: input.workspaceId,
    });
    expect(getSessionPromptRuntimeSnapshot(store)).toEqual({
      provider: "codex",
      model: "gpt-5.6-terra",
      reasoning_effort: "high",
    });

    store.trigger.inputUpdated({
      ...input,
      selectedRuntime: {
        provider: "claude",
        model: "claude-fable-5",
        reasoning_effort: "max",
      },
      selectionRevision: 3,
    });
    expect(store.getSnapshot().context.input.selectionRevision).toBe(4);
    expect(getSessionPromptRuntimeSnapshot(store)).toEqual({
      provider: "codex",
      model: "gpt-5.6-terra",
      reasoning_effort: "high",
    });

    store.trigger.runtimePersisted({
      revision: 3,
      runtime: { provider: "claude", model: "claude-fable-5", reasoning_effort: "max" },
      sessionId: input.sessionId,
      workspaceId: input.workspaceId,
    });
    expect(store.getSnapshot().context.input.selectionRevision).toBe(4);
    expect(getSessionPromptRuntimeSnapshot(store)).toEqual({
      provider: "codex",
      model: "gpt-5.6-terra",
      reasoning_effort: "high",
    });
  });

  // Invariant: an incomplete session projection never enables a prompt or crashes
  // the runtime selector. Owning layer: interaction-store unit. Canonical suite: this file.
  it("degrades a missing workspace into a disabled prompt input", () => {
    expect(
      sessionPromptRuntimeInput({
        agentName: primarySessionFixture.agent_name,
        canPrompt: true,
        effectiveRuntime: primarySessionFixture.runtime.effective,
        selectedRuntime: primarySessionFixture.runtime.selected,
        selectionRevision: primarySessionFixture.runtime.selection_revision,
        sessionId: primarySessionFixture.id,
        workspaceId: "   ",
      })
    ).toMatchObject({ canPrompt: false, workspaceId: "" });
  });
});
