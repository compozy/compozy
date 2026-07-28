import { beforeEach, describe, expect, it } from "vitest";

import { useSessionStore } from "../../hooks/use-session-store";

describe("session-store", () => {
  beforeEach(() => {
    useSessionStore.setState({ drafts: {} });
  });

  it("stores a draft for a session and merges patches", () => {
    useSessionStore.getState().setDraft("session-a", { text: "Hello" });
    useSessionStore.getState().setDraft("session-a", { channel: "release" });

    expect(useSessionStore.getState().drafts["session-a"]).toEqual({
      text: "Hello",
      channel: "release",
    });
  });

  it("keeps drafts isolated per session", () => {
    useSessionStore.getState().setDraft("session-a", { text: "Alpha draft" });
    useSessionStore.getState().setDraft("session-b", { text: "Bravo draft" });

    expect(useSessionStore.getState().drafts["session-a"]?.text).toBe("Alpha draft");
    expect(useSessionStore.getState().drafts["session-b"]?.text).toBe("Bravo draft");
  });

  it("removes a draft when its content becomes empty", () => {
    useSessionStore.getState().setDraft("session-a", { text: "Hello", channel: "release" });
    useSessionStore.getState().setDraft("session-a", { text: "", channel: undefined });

    expect(useSessionStore.getState().drafts["session-a"]).toBeUndefined();
  });

  it("clearDraft drops a single session draft", () => {
    useSessionStore.getState().setDraft("session-a", { text: "Hello", channel: "release" });
    useSessionStore.getState().setDraft("session-b", { text: "Bravo" });

    useSessionStore.getState().clearDraft("session-a");

    expect(useSessionStore.getState().drafts["session-a"]).toBeUndefined();
    expect(useSessionStore.getState().drafts["session-b"]?.text).toBe("Bravo");
  });

  it("clearAllDrafts resets the draft cache", () => {
    useSessionStore.getState().setDraft("session-a", { text: "Hello" });
    useSessionStore.getState().setDraft("session-b", { text: "Bravo" });

    useSessionStore.getState().clearAllDrafts();

    expect(useSessionStore.getState().drafts).toEqual({});
  });
});
