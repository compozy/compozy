import type { TerminalActorKind, TerminalJournalEntry } from "../types";

const UUID_SHAPE = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

const KIND_LABEL: Record<TerminalActorKind, string> = {
  human: "A person",
  agent: "An agent",
  system: "CompozyOS",
};

/**
 * Who ran the command, from the fields the journal row actually carries.
 *
 * The wire has `actor.id` and `actor.kind` only. A UUID is a machine id, not a
 * name, so the kind label stands in. A readable id is shown as itself.
 */
export function terminalJournalActorLabel(
  actor: Pick<TerminalJournalEntry["actor"], "id" | "kind">
): string {
  const id = actor.id.trim();
  if (id === "" || UUID_SHAPE.test(id)) return KIND_LABEL[actor.kind];
  return id;
}
