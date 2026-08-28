import { describe, expect, it } from "vitest";

import { ADVANCED_DEFAULTS } from "../../lib/session-create-draft";
import { createSessionCreateStore } from "../session-create-store";

// Suite: session-create store transitions
// Invariant: returning to Simple removes every Advanced-only launch field while preserving
// durable identity, while a composer fallback prompt remains separate from the launch draft.
// Owning layer: session-create interaction store. Canonical suite: this store suite.
// Boundary IN: dialog mode and draft events. Boundary OUT: dialog view-model wiring, owned by hooks/__tests__/use-session-create-dialog.test.tsx.
describe("session-create store", () => {
  it("Should reset every advanced-only field when returning to Simple", () => {
    const store = createSessionCreateStore();
    store.trigger.dialogOpened({ agentName: "claude-agent", workspaceId: "ws_alpha" });
    store.trigger.modeSelected({ mode: "advanced" });
    store.trigger.sessionNameChanged({ sessionName: "Keep this" });
    store.trigger.networkParticipationSelected({
      networkParticipationMode: "live",
      networkChannelId: "release-room",
      networkChannelStrategy: "named",
    });

    store.trigger.modeSelected({ mode: "simple" });

    expect(store.getSnapshot().context).toMatchObject({
      draft: {
        agentName: "claude-agent",
        sessionName: "Keep this",
        workspaceId: "ws_alpha",
        ...ADVANCED_DEFAULTS,
      },
      mode: "simple",
    });
  });

  it("Should keep a staged fallback prompt outside the launch draft", () => {
    const store = createSessionCreateStore();
    store.trigger.dialogOpened({ agentName: "claude-agent", workspaceId: "ws_alpha" });
    store.trigger.fallbackPromptStaged({ prompt: "Investigate the regression" });
    store.trigger.environmentSelected({ environment: { kind: "new", name: "" } });

    store.trigger.modeSelected({ mode: "advanced" });
    store.trigger.modeSelected({ mode: "simple" });
    store.trigger.agentSelected({ agentName: "codex-agent", workspaceId: "ws_alpha" });

    expect(store.getSnapshot().context).toMatchObject({
      draft: {
        agentName: "codex-agent",
        // The environment is workspace-scoped launch state, so it still resets.
        environment: ADVANCED_DEFAULTS.environment,
      },
      pendingPrompt: "Investigate the regression",
    });
    expect(store.getSnapshot().context.draft).not.toHaveProperty("firstMessage");
  });

  it.each(["", "   "])("Should preserve general for an empty agent selection %j", agentName => {
    const store = createSessionCreateStore();
    store.trigger.dialogOpened({ agentName: "general", workspaceId: "ws_alpha" });

    store.trigger.agentSelected({ agentName, workspaceId: "ws_alpha" });

    expect(store.getSnapshot().context.draft.agentName).toBe("general");
  });

  it("Should retract a submit waiting on the environment when the choice changes", () => {
    const store = createSessionCreateStore();
    store.trigger.dialogOpened({ agentName: "claude-agent", workspaceId: "ws_alpha" });
    store.trigger.environmentAwaited({
      agentName: "claude-agent",
      pendingPrompt: "Investigate the regression",
      previousEnvironment: { kind: "root" },
      request: { agent_name: "claude-agent", workspace: "ws_alpha" },
      terminalQuote: null,
      workspaceId: "ws_alpha",
    });
    expect(store.getSnapshot().context.pendingSubmit).not.toBeNull();

    store.trigger.environmentSelected({ environment: { kind: "root" } });

    expect(store.getSnapshot().context.pendingSubmit).toBeNull();
  });

  it("Should keep palette fallback on submitting then idle without dialog navigation [RA0262]", () => {
    const store = createSessionCreateStore();

    store.trigger.fallbackRequested({ agentName: "general", workspaceId: "ws_home" });
    expect(store.getSnapshot().context).toMatchObject({
      open: false,
      operation: { status: "submitting", agentName: "general", workspaceId: "ws_home" },
    });

    store.trigger.fallbackRequested({ agentName: "other", workspaceId: "ws_home" });
    expect(store.getSnapshot().context.operation).toMatchObject({
      status: "submitting",
      agentName: "general",
    });

    const attempt = store.getSnapshot().context.attempt;
    store.trigger.fallbackCompleted({ attempt });
    expect(store.getSnapshot().context.operation).toEqual({ status: "idle" });
    expect(store.getSnapshot().context.open).toBe(false);
  });
});
