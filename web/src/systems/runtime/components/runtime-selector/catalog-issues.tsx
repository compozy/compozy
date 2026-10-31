import { TriangleAlert } from "@compozy/ui/icons";
import { useId } from "react";

import { Eyebrow, Popover, PopoverContent, PopoverTrigger } from "@compozy/ui";

import type { RuntimeProviderOption } from "./types";

const CATALOG_ISSUES_LABEL = "Model list issues";

interface CatalogIssueRow {
  id: string;
  name: string | null;
  message: string;
}

/**
 * Catalog-wide status first, then one row per listed provider with a model
 * list problem, in provider order. Rail filters and search do not narrow it:
 * the header mark reports the whole catalog's health.
 */
function catalogIssueRows(
  status: string | null | undefined,
  providers: readonly RuntimeProviderOption[],
  providerIssues: Readonly<Record<string, string>>
): CatalogIssueRow[] {
  const rows: CatalogIssueRow[] = status ? [{ id: "catalog", name: null, message: status }] : [];
  for (const provider of providers) {
    const message = providerIssues[provider.id];
    if (message) rows.push({ id: provider.id, name: provider.name, message });
  }
  return rows;
}

export interface RuntimeCatalogIssuesProps {
  status?: string | null;
  providers: readonly RuntimeProviderOption[];
  providerIssues: Readonly<Record<string, string>>;
}

/**
 * The selector header's warning mark (issue #827): model list problems live
 * behind it instead of above the model list. It renders nothing while the
 * catalog is healthy. Its popover nests inside the selector popup, so Escape
 * and an outside press close it before the selector.
 */
export function RuntimeCatalogIssues({
  status,
  providers,
  providerIssues,
}: RuntimeCatalogIssuesProps) {
  const titleId = useId();
  const rows = catalogIssueRows(status, providers, providerIssues);
  if (rows.length === 0) return null;

  return (
    <Popover>
      <PopoverTrigger
        type="button"
        aria-label={CATALOG_ISSUES_LABEL}
        title={CATALOG_ISSUES_LABEL}
        data-testid="runtime-selector-catalog-issues"
        className="grid size-6 shrink-0 place-items-center rounded-sm text-warning outline-none transition-colors hover:bg-surface-2 focus-visible:bg-surface-2 focus-visible:shadow-focus-ring data-[popup-open]:bg-surface-2"
      >
        <TriangleAlert aria-hidden="true" className="size-3.5" />
      </PopoverTrigger>
      <PopoverContent
        align="end"
        sideOffset={6}
        aria-labelledby={titleId}
        className="max-h-[min(280px,var(--available-height))] w-72 max-w-[calc(100vw-2rem)] gap-2 overflow-y-auto overscroll-contain"
        data-testid="runtime-selector-catalog-issues-popup"
      >
        <Eyebrow id={titleId} className="px-0.5 text-muted-foreground">
          {CATALOG_ISSUES_LABEL}
        </Eyebrow>
        <ul className="flex flex-col gap-2 px-0.5">
          {rows.map(row => (
            <li key={row.id} className="flex flex-col gap-0.5">
              {row.name ? (
                <span className="text-small-body font-medium text-fg">{row.name}</span>
              ) : null}
              <span className="text-form-hint text-pretty text-muted-foreground">
                {row.message}
              </span>
            </li>
          ))}
        </ul>
      </PopoverContent>
    </Popover>
  );
}
