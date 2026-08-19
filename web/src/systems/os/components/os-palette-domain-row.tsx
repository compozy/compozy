import { KindIcon, Pill } from "@compozy/ui";

import { statusTone } from "@/lib/status-tone";

import type { OsPaletteDomainRow } from "../hooks/use-os-palette-domain-search";
import { getOsAppDescriptor } from "../lib/app-catalog";

export function OsPaletteDomainRow({ row }: { row: OsPaletteDomainRow }) {
  const descriptor = getOsAppDescriptor(row.app);
  return (
    <div className="flex min-w-0 flex-1 items-center gap-3">
      <KindIcon fallback={descriptor.icon} kind={row.app} size="sm" />
      <div className="min-w-0 flex-1">
        <div className="truncate text-card-title text-fg">{row.label}</div>
        {row.detail || row.workspaceLabel ? (
          <div className="truncate text-small-body text-muted">
            {[row.detail, row.workspaceLabel].filter(Boolean).join(" · ")}
          </div>
        ) : null}
      </div>
      {row.status ? (
        <Pill data-testid="os-palette-domain-status" size="xs" tone={statusTone(row.status)}>
          {formatStatus(row.status)}
        </Pill>
      ) : null}
    </div>
  );
}

function formatStatus(status: string): string {
  return status.replaceAll("_", " ").replace(/^./, value => value.toLocaleUpperCase());
}
