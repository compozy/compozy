export type ProfileAutomationIdentity = {
  kind: "job" | "trigger";
  id: string;
};

/** Parses the identities returned by profile archive and unarchive operations. */
export function parseProfileAutomationIdentity(value: string): ProfileAutomationIdentity {
  const separator = value.indexOf(":");
  const kind = value.slice(0, separator);
  const id = value.slice(separator + 1).trim();
  if ((kind !== "job" && kind !== "trigger") || id === "") {
    throw new Error(`Invalid paused automation identity: ${value}`);
  }
  return { kind, id };
}
