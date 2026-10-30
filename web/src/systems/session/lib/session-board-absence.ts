import { createStoreLogic } from "@xstate/store";

/**
 * Where the board stands on the selected cards missing from every settled read.
 * `rereading`: the board is being read again; `settling`: that read succeeded and
 * its cards are being published; `confirmed`: still missing after it, so they go.
 * A failed re-read leaves the absence `unconfirmed`: the cards stay held.
 */
export type SessionBoardAbsenceStatus =
  | "idle"
  | "rereading"
  | "settling"
  | "confirmed"
  | "unconfirmed";

interface SessionBoardAbsenceContext {
  /** The missing ids, sorted and joined; `""` while nothing selected is missing. */
  key: string;
  status: SessionBoardAbsenceStatus;
  /** Bumped per new absence, so a late re-read never confirms a later one. */
  episode: number;
}

/**
 * One absence at a time: a new set of missing ids starts a new episode and its
 * own re-read; a card that comes back ends the episode, so a later identical
 * absence is confirmed by a fresh re-read, never by an old one.
 */
export const sessionBoardAbsenceLogic = createStoreLogic({
  context: (): SessionBoardAbsenceContext => ({ key: "", status: "idle", episode: 0 }),
  on: {
    observed: (context, { key }: { key: string }) => {
      if (key === context.key) return context;
      return {
        key,
        status: key === "" ? ("idle" as const) : ("rereading" as const),
        episode: context.episode + 1,
      };
    },
    reread: (context, { episode, ok }: { episode: number; ok: boolean }) =>
      episode === context.episode && context.status === "rereading"
        ? { ...context, status: ok ? ("settling" as const) : ("unconfirmed" as const) }
        : context,
    settled: (context, { episode }: { episode: number }) =>
      episode === context.episode && context.status === "settling"
        ? { ...context, status: "confirmed" as const }
        : context,
  },
});

/** The stable key of a set of missing ids. */
export function sessionBoardAbsenceKey(ids: readonly string[]): string {
  return [...ids].sort().join("\n");
}
