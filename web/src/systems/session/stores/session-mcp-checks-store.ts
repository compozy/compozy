import { createStoreLogic } from "@xstate/store";

/** One server's Check again: running, or answered and still not working. */
export type SessionMCPCheckState = "checking" | "still_failing";

interface SessionMCPChecksContext {
  /** Each server's check state, keyed by its owner and name; absent when it has none. */
  checks: Readonly<Record<string, SessionMCPCheckState>>;
}

function withCheck(
  context: SessionMCPChecksContext,
  key: string,
  next: SessionMCPCheckState | null
): SessionMCPChecksContext {
  if ((context.checks[key] ?? null) === next) return context;
  const { [key]: _previous, ...rest } = context.checks;
  return { checks: next === null ? rest : { ...rest, [key]: next } };
}

/**
 * The composer popover's Check again lifecycle, one owner for every server's check: a check starts,
 * then answers working (cleared), still failing, or with an error (cleared). A server is in flight
 * exactly while its state is `checking`, so a second press reads the current snapshot and is
 * ignored. The session's MCP read itself stays in TanStack Query.
 */
export const sessionMCPChecksLogic = createStoreLogic({
  context: (): SessionMCPChecksContext => ({ checks: {} }),
  on: {
    started: (context, { key }: { key: string }) => withCheck(context, key, "checking"),
    stillFailing: (context, { key }: { key: string }) => withCheck(context, key, "still_failing"),
    settled: (context, { key }: { key: string }) => withCheck(context, key, null),
    /** A panel opening fresh or closing drops an earlier "still not working", never a running check. */
    stillFailingCleared: (context, { key }: { key: string }) =>
      context.checks[key] === "still_failing" ? withCheck(context, key, null) : context,
  },
});
