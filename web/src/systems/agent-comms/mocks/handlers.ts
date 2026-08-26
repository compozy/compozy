/**
 * The shared agent-comms mock server.
 *
 * One store, staged through the setters below. MSW registers handlers globally
 * for a test file, so the population has to be reachable from outside the
 * handler array — hence the shared instance. Stories do **not** use this one:
 * they build their own with `createAgentCommsHandlers`, because two stories
 * loaded at once would otherwise overwrite each other's data.
 */
import type { HttpHandler } from "msw";

import { buildAgentCommsHandlers } from "./agent-comms-handlers";
import { createAgentCommsMockStore } from "./agent-comms-mock-store";
import { activityTreeCallsFixture, callMessagesFixture } from "./fixtures";
import type { CallMessagePayload, CallPayload } from "../types";

const store = createAgentCommsMockStore({
  calls: activityTreeCallsFixture,
  messages: callMessagesFixture,
});

export function setAgentCommsMockCalls(next: readonly CallPayload[]): void {
  store.setCalls(next);
}

export function setAgentCommsMockMessages(next: readonly CallMessagePayload[]): void {
  store.setMessages(next);
}

export function resetAgentCommsMockState(): void {
  store.setCalls(activityTreeCallsFixture);
  store.setMessages(callMessagesFixture);
  store.resetSequences();
}

export const handlers: HttpHandler[] = buildAgentCommsHandlers(store);
