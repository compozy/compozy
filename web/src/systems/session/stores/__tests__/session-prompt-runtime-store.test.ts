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

  // Invariant: an incomplete session projection never enables a prompt or crashes
  // the runtime selector. Owning layer: interaction-store unit. Canonical suite: this file.
  it("degrades a missing workspace into a disabled prompt input", () => {
    expect(
      sessionPromptRuntimeInput({
        agentName: primarySessionFixture.agent_name,
        canPrompt: true,
        effectiveRuntime: primarySessionFixture.runtime.effective,
        workspaceId: "   ",
      })
    ).toMatchObject({ canPrompt: false, workspaceId: "" });
  });
});
