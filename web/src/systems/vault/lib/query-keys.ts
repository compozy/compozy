import type { VaultListFilter } from "../types";

function normalizeText(value?: string | null): string {
  return value?.trim() ?? "";
}

export const vaultKeys = {
  all: ["vault"] as const,
  lists: () => [...vaultKeys.all, "list"] as const,
  list: (filter: VaultListFilter = {}) =>
    [...vaultKeys.lists(), normalizeText(filter.namespace), normalizeText(filter.prefix)] as const,
  details: () => [...vaultKeys.all, "detail"] as const,
  detail: (ref: string) => [...vaultKeys.details(), normalizeText(ref)] as const,
  sessions: () => [...vaultKeys.all, "sessions"] as const,
  session: (sessionId: string) => [...vaultKeys.sessions(), normalizeText(sessionId)] as const,
};
