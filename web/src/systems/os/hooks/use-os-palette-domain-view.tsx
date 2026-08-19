import { useState } from "react";

import { useActiveWorkspace } from "@/systems/workspace";

import {
  OsPaletteDomainChips,
  type PaletteDomainChip,
} from "../components/os-palette-domain-chips";
import { OsPaletteDomainRow } from "../components/os-palette-domain-row";
import { OsPaletteViewNote } from "../components/os-palette-view-note";
import { PaletteGridView } from "../components/palette-grid-view";
import type { CmdPaletteViewAction, CmdPaletteViewGrid } from "../lib/cmd-palette-types";
import type {
  PaletteViewContent,
  PaletteViewControllerInput,
  PaletteViewDefinition,
} from "../lib/palette-view-registry";
import { useCmdPaletteRankSignals } from "./use-cmd-palette-rank-signals";
import { useOsShell } from "./use-os-shell";
import {
  useOsPaletteDomainSearch,
  type OsPaletteDomainRow as DomainRow,
} from "./use-os-palette-domain-search";

export function useOsPaletteDomainView(
  definition: PaletteViewDefinition,
  { query, onDismiss }: PaletteViewControllerInput
): PaletteViewContent {
  const domainTitle = definition.domainTitle ?? definition.title;
  const [filter, setFilter] = useState("all");
  const { coordinator } = useOsShell();
  const { runtimeWorkspaceId, scope, workspaces } = useActiveWorkspace();
  const signals = useCmdPaletteRankSignals(runtimeWorkspaceId, true);
  const sections = useOsPaletteDomainSearch({
    open: true,
    query,
    workspaceId: runtimeWorkspaceId,
    scope,
    workspaceNames: new Map(workspaces.map(workspace => [workspace.id, workspace.name])),
    signals: signals.data,
    targetDomain: domainTitle,
  });
  const section = sections.find(candidate => candidate.title === domainTitle);
  const sourceRows = section?.rows ?? [];
  const chips = domainChips(domainTitle, sourceRows);
  const visible =
    filter === "all" ? sourceRows : sourceRows.filter(row => domainFilter(row) === filter);
  const loading = signals.loading || section?.loading === true;

  if (domainTitle === "Marketplace") {
    const grid: CmdPaletteViewGrid = {
      sections: [
        {
          title: "Marketplace",
          tiles: visible.map(row => ({
            id: row.key,
            title: row.label,
            image: { token: "package" },
            actions: [
              {
                title: "Open",
                primary: true,
                action: { kind: "navigate", app: row.app, args: { row_key: row.key } },
              },
            ],
          })),
        },
      ],
    };
    return {
      kind: "grid",
      rows: [],
      body: (
        <PaletteGridView
          grid={grid}
          empty={{
            title: domainEmptyMessage(domainTitle, query, filter, loading, section?.error ?? null),
          }}
          onAction={(action: CmdPaletteViewAction) => {
            const rowKey = action.action?.args?.row_key;
            const row = visible.find(candidate => candidate.key === rowKey);
            if (!row) return;
            onDismiss();
            void coordinator.userOpen({ app: row.app, route: row.route });
          }}
        />
      ),
      header: null,
      empty: null,
      note: null,
      backHint: "back",
      resetKey: "marketplace-grid",
      onEmptyQueryBackspace: () => false,
    };
  }

  return {
    kind: "list",
    rows: visible.map(row => ({
      value: row.key,
      testId: `os-palette-domain-${row.key.replaceAll(":", "-")}`,
      node: <OsPaletteDomainRow row={row} />,
      onSelect: () => {
        onDismiss();
        void coordinator.userOpen({ app: row.app, route: row.route });
      },
    })),
    header:
      chips.length <= 1 ? null : (
        <OsPaletteDomainChips active={filter} chips={chips} onChange={setFilter} />
      ),
    empty: (
      <OsPaletteViewNote placement="empty">
        {domainEmptyMessage(domainTitle, query, filter, loading, section?.error ?? null)}
      </OsPaletteViewNote>
    ),
    note: null,
    backHint: filter === "all" ? "back" : "clear filter",
    resetKey: filter,
    onEmptyQueryBackspace: () => {
      if (filter === "all") return false;
      setFilter("all");
      return true;
    },
  };
}

function domainChips(title: string, rows: readonly DomainRow[]): readonly PaletteDomainChip[] {
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

function domainFilter(row: DomainRow): string {
  const status = row.status?.trim().toLowerCase();
  if (!status) return "all";
  if (["draft", "pending", "ready"].includes(status)) return "queued";
  if (status === "in_progress") return "running";
  if (["blocked", "needs_attention"].includes(status)) return "needs-approval";
  if (status === "completed") return "done";
  return status;
}

function filterLabel(filter: string): string {
  return filter.replaceAll("-", " ").replace(/^./, value => value.toLocaleUpperCase());
}

function domainEmptyMessage(
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
