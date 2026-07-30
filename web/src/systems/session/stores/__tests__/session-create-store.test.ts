import { describe, expect, it } from "vitest";

import { ADVANCED_DEFAULTS } from "../../lib/session-create-draft";
import { createSessionCreateStore } from "../session-create-store";

// Suite: session-create store transitions
// Invariant: returning to Simple removes every Advanced-only launch field while preserving
// the durable session identity. Owning layer: session-create interaction store. Canonical suite: this store suite.
// Boundary IN: dialog mode and draft events. Boundary OUT: dialog view-model wiring, owned by hooks/__tests__/use-session-create-dialog.test.tsx.
describe("session-create store", () => {
  it("Should reset every advanced-only field when returning to Simple", () => {
    const store = createSessionCreateStore();
    store.trigger.dialogOpened({ agentName: "claude-agent", workspaceId: "ws_alpha" });
    store.trigger.modeSelected({ mode: "advanced" });
    store.trigger.sessionNameChanged({ sessionName: "Keep this" });
    store.trigger.workspacePathChanged({ workspacePath: "services/checkout" });
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
});
