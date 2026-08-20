// Suite: command palette programmable-view lifecycle
// Invariant: each mounted view owns one causal lifecycle; controlled event counters only
// increase, removed handlers survive exactly two frames, and three consecutive hard misses
// open the circuit without clearing the last valid payload.
// Boundary IN: the per-view XState store and frame patch reducer.
// Boundary OUT: HTTP/SSE transport, React rendering, and extension subprocess timing.

import { describe, expect, it } from "vitest";

import type { CmdPaletteViewFrame } from "../../lib/cmd-palette-types";
import {
  cmdPaletteViewProgramLogic,
  programHandlerIsLive,
} from "../cmd-palette-view-program-store";

describe("command palette programmable-view lifecycle", () => {
  it("Should count controlled events and apply a causally matching frame [UT-163]", () => {
    const store = cmdPaletteViewProgramLogic.createStore();
    store.trigger.openSucceeded({ frame: frame("vr_1", ["search"], "Initial") });

    store.trigger.eventSent({
      seq: 1,
      controlled: true,
      handler: "search",
      args: ["new"],
    });
    expect(store.getSnapshot().context).toMatchObject({
      eventCount: 1,
      pendingSeq: 1,
      phase: "ready",
    });

    store.trigger.eventSent({
      seq: 1,
      controlled: true,
      handler: "search",
      args: ["stale"],
    });
    store.trigger.eventSent({
      seq: 0,
      controlled: true,
      handler: "search",
      args: ["older"],
    });
    expect(store.getSnapshot().context).toMatchObject({
      eventCount: 1,
      lastEvent: { args: ["new"], controlled: true, handler: "search" },
      nextSeq: 1,
      pendingSeq: 1,
    });

    store.trigger.eventSent({
      seq: 2,
      controlled: false,
      handler: "search",
      args: ["uncontrolled"],
    });
    expect(store.getSnapshot().context).toMatchObject({
      eventCount: 1,
      lastEvent: { args: ["uncontrolled"], controlled: false, handler: "search" },
      nextSeq: 2,
      pendingSeq: 2,
    });

    store.trigger.frameReceived({
      frame: frame("vr_2", ["search"], "New result", { in_reply_to: 1, generation: 1 }),
    });
    expect(store.getSnapshot().context).toMatchObject({
      misses: 0,
      pendingSeq: null,
      phase: "ready",
      payload: { sections: [{ rows: [{ title: "New result" }] }] },
    });
  });

  it("Should quarantine removed handlers for two accepted frames [UT-164]", () => {
    const store = cmdPaletteViewProgramLogic.createStore();
    store.trigger.openSucceeded({ frame: frame("vr_1", ["old", "current"], "One") });
    store.trigger.frameReceived({ frame: frame("vr_2", ["current"], "Two") });
    expect(programHandlerIsLive(store.getSnapshot().context, "old")).toBe(true);

    store.trigger.frameReceived({ frame: frame("vr_3", ["current"], "Three") });
    expect(programHandlerIsLive(store.getSnapshot().context, "old")).toBe(true);

    store.trigger.frameReceived({ frame: frame("vr_4", ["current"], "Four") });
    expect(programHandlerIsLive(store.getSnapshot().context, "old")).toBe(false);
  });

  it("Should retain the last frame through busy and degraded bands, then break [UT-165, UT-168]", () => {
    const store = cmdPaletteViewProgramLogic.createStore();
    store.trigger.openSucceeded({ frame: frame("vr_1", ["search"], "Last good") });

    for (const seq of [1, 2, 3]) {
      store.trigger.eventSent({
        seq,
        controlled: true,
        handler: "search",
        args: [`query-${seq}`],
      });
      store.trigger.softBudgetElapsed({ seq });
      expect(store.getSnapshot().context).toMatchObject({
        payload: { sections: [{ rows: [{ title: "Last good" }] }] },
        phase: "busy",
      });
      store.trigger.hardBudgetElapsed({ seq });
      expect(store.getSnapshot().context.phase).toBe(seq === 3 ? "circuit-open" : "degraded");
    }

    expect(store.getSnapshot().context).toMatchObject({ misses: 3, pendingSeq: 3 });
  });

  it("Should apply the React kit root replacement without mutating the prior frame [UT-166]", () => {
    const store = cmdPaletteViewProgramLogic.createStore();
    const first = frame("vr_1", ["search"], "Before");
    store.trigger.openSucceeded({ frame: first });
    store.trigger.frameReceived({
      frame: {
        generation: 1,
        handlers: ["search"],
        revision: "vr_2",
        view_session: "vs_test",
        patch: {
          view_id: "ext.notes.browser",
          from: "vr_1",
          to: "vr_2",
          ops: [
            {
              op: "replace",
              path: "",
              value: payload("After"),
            },
          ],
        },
      },
    });

    expect(first.payload?.sections?.[0]?.rows[0]?.title).toBe("Before");
    expect(store.getSnapshot().context.payload?.sections?.[0]?.rows[0]?.title).toBe("After");
  });

  it("Should keep openEpoch and drop causal state when a session is reopened [RA0253]", () => {
    const store = cmdPaletteViewProgramLogic.createStore();
    store.trigger.openSucceeded({ frame: frame("vr_1", ["search"], "Live") });
    store.trigger.eventSent({ seq: 1, controlled: true, handler: "search", args: ["q"] });
    store.trigger.reloadRequested({});
    const epoch = store.getSnapshot().context.openEpoch;
    store.trigger.openStarted({ preserve: true });
    expect(store.getSnapshot().context).toMatchObject({
      eventCount: 0,
      lastEvent: null,
      nextSeq: 0,
      openEpoch: epoch,
      pendingSeq: null,
    });
  });

  it("Should ignore a late foreign session and a stale patch.from [RA0254]", () => {
    const store = cmdPaletteViewProgramLogic.createStore();
    store.trigger.openSucceeded({ frame: frame("vr_1", ["search"], "Live") });
    store.trigger.frameReceived({
      frame: { ...frame("vr_2", ["search"], "Other"), view_session: "vs_other" },
    });
    expect(store.getSnapshot().context.frame?.revision).toBe("vr_1");

    store.trigger.frameReceived({
      frame: {
        generation: 1,
        handlers: ["search"],
        revision: "vr_3",
        view_session: "vs_test",
        patch: {
          view_id: "ext.notes.browser",
          from: "vr_missing",
          to: "vr_3",
          ops: [{ op: "replace", path: "", value: payload("Stale") }],
        },
      },
    });
    expect(store.getSnapshot().context.phase).toBe("unavailable");
  });

  it("Should clear timeout state when a push frame has no in_reply_to [RA0255]", () => {
    const store = cmdPaletteViewProgramLogic.createStore();
    store.trigger.openSucceeded({ frame: frame("vr_1", ["search"], "Live") });
    store.trigger.eventSent({ seq: 1, controlled: true, handler: "search", args: ["q"] });
    store.trigger.frameReceived({ frame: frame("vr_2", ["search"], "Pushed") });
    expect(store.getSnapshot().context).toMatchObject({
      misses: 0,
      pendingSeq: null,
      phase: "ready",
    });
  });
});

function frame(
  revision: string,
  handlers: string[],
  title: string,
  extra: Partial<CmdPaletteViewFrame> = {}
): CmdPaletteViewFrame {
  return {
    view_session: "vs_test",
    revision,
    generation: 1,
    handlers,
    payload: payload(title),
    ...extra,
  };
}

function payload(title: string): NonNullable<CmdPaletteViewFrame["payload"]> {
  return {
    view: "v1",
    chrome: { complete: true, on_search: "search" },
    sections: [{ rows: [{ id: "one", title }] }],
  };
}
