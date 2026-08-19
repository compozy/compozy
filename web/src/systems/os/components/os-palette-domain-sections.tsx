import { AlertCircle, LoaderCircle } from "lucide-react";

import { CommandGroup, CommandItem, cn } from "@compozy/ui";

import type {
  OsPaletteDomainRow,
  OsPaletteDomainSection,
} from "../hooks/use-os-palette-domain-search";
import { getOsAppDescriptor } from "../lib/app-catalog";

const GROUP_CLASS = "mt-2 border-t border-line-soft px-2 pt-2 pb-0.5";
const HEADING_CLASS = "**:[[cmdk-group-heading]]:text-faint";
const ROW_CLASS = "h-12 gap-3 px-3";

export interface OsPaletteDomainSectionsProps {
  readonly sections: readonly OsPaletteDomainSection[];
  onOpen(row: OsPaletteDomainRow): void;
}

export function OsPaletteDomainSections({ sections, onOpen }: OsPaletteDomainSectionsProps) {
  return sections.map(section => {
    if (!section.loading && section.error === null && section.rows.length === 0) return null;
    return (
      <CommandGroup
        className={cn(GROUP_CLASS, HEADING_CLASS)}
        data-testid={`os-palette-domain-${section.title.toLowerCase().replaceAll(" ", "-")}`}
        heading={section.title}
        key={section.title}
      >
        {section.loading ? (
          <div className="flex h-10 items-center gap-2 px-3 text-micro text-subtle">
            <LoaderCircle className="size-3.5 animate-spin" />
            Loading…
          </div>
        ) : null}
        {section.error === null ? null : (
          <div
            className="flex min-h-10 items-center gap-2 px-3 text-micro text-danger"
            data-testid={`os-palette-domain-error-${section.title.toLowerCase().replaceAll(" ", "-")}`}
          >
            <AlertCircle className="size-3.5 shrink-0" />
            {section.error}
          </div>
        )}
        {section.rows.map(row => {
          const Icon = getOsAppDescriptor(row.app).icon;
          return (
            <CommandItem
              className={ROW_CLASS}
              data-palette-row={row.key}
              data-testid={`os-palette-domain-row-${row.key}`}
              forceMount
              key={row.key}
              value={row.key}
              onSelect={() => onOpen(row)}
            >
              <Icon className="size-3.5 shrink-0 text-muted" />
              <span className="min-w-0 truncate">{row.label}</span>
              {row.detail ? (
                <span className="ml-auto max-w-48 shrink truncate text-micro text-subtle">
                  {row.detail}
                </span>
              ) : null}
              {row.workspaceLabel ? (
                <span className="shrink-0 text-micro text-faint">{row.workspaceLabel}</span>
              ) : null}
            </CommandItem>
          );
        })}
        {section.total > section.rows.length ? (
          <div className="px-3 py-1 text-micro text-faint">
            showing {section.rows.length} of {section.total}
          </div>
        ) : null}
      </CommandGroup>
    );
  });
}
