import type { PillTone } from "@compozy/ui";

export interface AuthoredFileRevisionIdentity {
  id: string;
  created: string;
}

/** History revisions differ per file kind; read only the id and creation stamp both may carry. */
export function authoredFileRevisionIdentity(revision: object): AuthoredFileRevisionIdentity {
  const id = "id" in revision ? String(revision.id) : "";
  const created =
    "created_at" in revision && typeof revision.created_at === "string" ? revision.created_at : "";
  return { id, created };
}

const STATUS_TONES = new Map<string, PillTone>([
  ["valid", "success"],
  ["invalid", "danger"],
]);

export function authoredFileStatusTone(status: string): PillTone {
  return STATUS_TONES.get(status) ?? "neutral";
}
