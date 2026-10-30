// Suite: session-board-absence
// Invariant: selected board cards missing from every settled read leave the selection
// only after a re-read of the same absence, in the same episode, after which the
// board's reads succeeded: a failed read keeps them held and retries in a new episode,
// and a re-read that resolves after the cards came back (or after a later attempt
// began) never confirms anything.
// Owning layer: lib store logic; the board wiring is owned by sessions-board.test.tsx.
import { describe, expect, it } from "vitest";

import { sessionBoardAbsenceKey, sessionBoardAbsenceLogic } from "../session-board-absence";

function absence() {
  const store = sessionBoardAbsenceLogic.createStore();
  return { store, context: () => store.getSnapshot().context };
}

describe("sessionBoardAbsenceLogic", () => {
  it("Should confirm an absence only after its re-read settles with every read succeeding", () => {
    const { store, context } = absence();
    store.trigger.observed({ key: sessionBoardAbsenceKey(["b", "a"]) });
    expect(context()).toMatchObject({ key: "a\nb", status: "rereading" });
    const { episode } = context();
    store.trigger.settled({ episode, ok: true });
    expect(context().status).toBe("rereading");
    store.trigger.reread({ episode });
    expect(context().status).toBe("settling");
    store.trigger.settled({ episode, ok: true });
    expect(context().status).toBe("confirmed");
  });

  it("Should hold an absence whose board read failed and retry it in a new episode", () => {
    const { store, context } = absence();
    store.trigger.observed({ key: "a" });
    const first = context().episode;
    store.trigger.reread({ episode: first });
    store.trigger.settled({ episode: first, ok: false });
    expect(context().status).toBe("unconfirmed");

    store.trigger.retry({ episode: first });
    expect(context()).toMatchObject({ key: "a", status: "rereading" });
    const second = context().episode;
    expect(second).toBeGreaterThan(first);
    // The failed attempt's late results decide nothing; the retry's own can confirm.
    store.trigger.reread({ episode: first });
    expect(context().status).toBe("rereading");
    store.trigger.reread({ episode: second });
    store.trigger.settled({ episode: second, ok: true });
    expect(context().status).toBe("confirmed");
  });

  it("Should ignore a re-read from an absence that already ended", () => {
    const { store, context } = absence();
    store.trigger.observed({ key: "a" });
    const stale = context().episode;
    store.trigger.observed({ key: "" });
    store.trigger.observed({ key: "a" });
    store.trigger.reread({ episode: stale });
    expect(context().status).toBe("rereading");
    expect(context().episode).toBeGreaterThan(stale);
  });
});
