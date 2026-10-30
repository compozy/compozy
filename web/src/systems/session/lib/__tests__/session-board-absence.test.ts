// Suite: session-board-absence
// Invariant: selected board cards missing from every settled read leave the selection
// only after a successful re-read of the same absence, in the same episode: a failed
// re-read keeps them held, and a re-read that resolves after the cards came back (or
// after a new absence began) never confirms anything.
// Owning layer: lib store logic; the board wiring is owned by sessions-board.test.tsx.
import { describe, expect, it } from "vitest";

import { sessionBoardAbsenceKey, sessionBoardAbsenceLogic } from "../session-board-absence";

function absence() {
  const store = sessionBoardAbsenceLogic.createStore();
  return { store, context: () => store.getSnapshot().context };
}

describe("sessionBoardAbsenceLogic", () => {
  it("Should confirm an absence only after its successful re-read settles", () => {
    const { store, context } = absence();
    store.trigger.observed({ key: sessionBoardAbsenceKey(["b", "a"]) });
    expect(context()).toMatchObject({ key: "a\nb", status: "rereading" });
    const { episode } = context();
    store.trigger.settled({ episode });
    expect(context().status).toBe("rereading");
    store.trigger.reread({ episode, ok: true });
    expect(context().status).toBe("settling");
    store.trigger.settled({ episode });
    expect(context().status).toBe("confirmed");
  });

  it("Should keep an absence held when its re-read fails", () => {
    const { store, context } = absence();
    store.trigger.observed({ key: "a" });
    store.trigger.reread({ episode: context().episode, ok: false });
    store.trigger.settled({ episode: context().episode });
    expect(context().status).toBe("unconfirmed");
  });

  it("Should ignore a re-read from an absence that already ended", () => {
    const { store, context } = absence();
    store.trigger.observed({ key: "a" });
    const stale = context().episode;
    store.trigger.observed({ key: "" });
    store.trigger.observed({ key: "a" });
    store.trigger.reread({ episode: stale, ok: true });
    expect(context().status).toBe("rereading");
    expect(context().episode).toBeGreaterThan(stale);
  });
});
