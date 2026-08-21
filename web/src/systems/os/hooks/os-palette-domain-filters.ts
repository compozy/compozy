import type { PaletteDomainChip } from "../components/os-palette-domain-chips";
import type { OsPaletteDomainRow } from "./use-os-palette-domain-search";

export function domainFilter(row: OsPaletteDomainRow): string {
  const status = row.status?.trim().toLowerCase();
  if (!status) return "all";
  if (["draft", "pending", "ready"].includes(status)) return "queued";
  if (status === "in_progress") return "running";
  if (["blocked", "needs_attention"].includes(status)) return "needs-approval";
  if (status === "completed") return "done";
  return status;
}

export function filterLabel(filter: string): string {
  return filter.replaceAll("-", " ").replace(/^./, value => value.toLocaleUpperCase());
}

export function domainChips(
  title: string,
  rows: readonly OsPaletteDomainRow[]
): readonly PaletteDomainChip[] {
  const counts = new Map<string, number>();
  for (const row of rows) {
    const id = domainFilter(row);
    if (id !== "all") counts.set(id, (counts.get(id) ?? 0) + 1);
  }
  const ordered =
    title === "Tasks"
      ? ["queued", "running", "needs-approval", "done", "failed"]
      : [...counts.keys()].sort();
  const chips: PaletteDomainChip[] = [{ id: "all", label: "All", count: rows.length }];
  for (const id of ordered) {
    if (title !== "Tasks" && !counts.has(id)) continue;
    chips.push({ id, label: filterLabel(id), count: counts.get(id) ?? 0 });
  }
  return chips;
}

export function domainEmptyMessage(
  title: string,
  query: string,
  filter: string,
  loading: boolean,
  error: string | null
): string {
  if (loading) return `Loading ${title.toLocaleLowerCase()}…`;
  if (error) return error;
  if (query.trim() !== "") return `No ${title.toLocaleLowerCase()} match “${query.trim()}”.`;
  if (filter !== "all")
    return `No ${title.toLocaleLowerCase()} are ${filterLabel(filter).toLocaleLowerCase()}.`;
  return `No ${title.toLocaleLowerCase()} yet.`;
}
