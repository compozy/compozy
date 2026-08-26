import { AlertCircle, LoaderCircle } from "lucide-react";

import { CommandGroup, CommandItem } from "@compozy/ui";

import { ProfileOwnerTag } from "@/systems/profiles";

import type {
  OsPaletteDomainRow,
  OsPaletteDomainSection,
} from "../hooks/use-os-palette-domain-search";
import { getOsAppDescriptor } from "../lib/app-catalog";
import {
  paletteGroupClass,
  paletteGroupFollowClass,
  paletteRowClass,
} from "../lib/palette-view-inset";

const GROUP_CLASS = `${paletteGroupClass} ${paletteGroupFollowClass}`;

export interface OsPaletteDomainSectionsProps {
  readonly sections: readonly OsPaletteDomainSection[];
  onOpen(row: OsPaletteDomainRow): void;
}

export function OsPaletteDomainSections({ sections, onOpen }: OsPaletteDomainSectionsProps) {
  return sections.map(section => {
    if (!section.loading && section.error === null && section.rows.length === 0) return null;
    return (
      <CommandGroup
        className={GROUP_CLASS}
        data-testid={`os-palette-domain-${section.title.toLowerCase().replaceAll(" ", "-")}`}
        heading={section.title}
        key={section.title}
      >
        {section.loading ? (
          <CommandItem
            className={`${paletteRowClass} text-micro text-subtle`}
            disabled
            forceMount
            value={`${section.title}:loading`}
          >
            <LoaderCircle className="size-3.5 animate-spin" />
            Loading…
          </CommandItem>
        ) : null}
        {section.error === null ? null : (
          <CommandItem
            className={`${paletteRowClass} text-micro text-danger`}
            data-testid={`os-palette-domain-error-${section.title.toLowerCase().replaceAll(" ", "-")}`}
            disabled
            forceMount
            value={`${section.title}:error`}
          >
            <AlertCircle className="size-3.5 shrink-0" />
            {section.error}
          </CommandItem>
        )}
        {section.rows.map(row => {
          const Icon = getOsAppDescriptor(row.app).icon;
          return (
            <CommandItem
              className={paletteRowClass}
              data-palette-row={row.key}
              data-testid={`os-palette-domain-row-${row.key}`}
              forceMount
              key={row.key}
              value={row.key}
              onSelect={() => onOpen(row)}
            >
              <Icon className="size-3.5 shrink-0 text-muted" />
              <span className="min-w-0 truncate leading-none">{row.label}</span>
              {row.detail ? (
                <span className="ml-auto max-w-48 shrink truncate text-micro text-subtle">
                  {row.detail}
                </span>
              ) : null}
              {row.workspaceLabel ? (
                <span className="shrink-0 text-micro text-faint">{row.workspaceLabel}</span>
              ) : null}
              {row.owner ? <ProfileOwnerTag owner={row.owner} /> : null}
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
