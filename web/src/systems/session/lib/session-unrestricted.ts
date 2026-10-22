import type { SessionPayload } from "../types";

/** USW `_uiux.md` S5: the grant's plain description, shown on hover and to assistive tech. */
export const SESSION_UNRESTRICTED_COPY = {
  label: "Unrestricted",
  tooltip: "This agent can read and change every project and profile.",
} as const;

/** The session runs with the agent's `unrestricted` workspace grant (immutable per session). */
export function sessionIsUnrestricted(
  session: Pick<SessionPayload, "effective_workspace_access">
): boolean {
  return session.effective_workspace_access === "unrestricted";
}
