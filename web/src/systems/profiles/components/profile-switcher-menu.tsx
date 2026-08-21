import { Check, Plus } from "lucide-react";

import { cn, Command, CommandGroup, CommandItem, CommandList, CommandSeparator } from "@compozy/ui";

import { PROFILE_BOUNDARY_ANSWER } from "../lib/profile-copy";
import type { ProfileRow } from "../lib/profile-rows";
import { ProfileGlyph } from "./profile-glyph";

export interface ProfileSwitcherMenuProps {
  rows: readonly ProfileRow[];
  aggregate: boolean;
  archivedCount: number;
  onSelectProfile: (name: string) => void;
  onSelectAggregate: () => void;
  onCreate: () => void;
  onOpenSettings: () => void;
}

export function ProfileSwitcherMenu({
  rows,
  aggregate,
  archivedCount,
  onSelectProfile,
  onSelectAggregate,
  onCreate,
  onOpenSettings,
}: ProfileSwitcherMenuProps) {
  return (
    <Command>
      <CommandList className="max-h-none">
        <CommandGroup>
          {rows.map(row => (
            <CommandItem
              key={row.name}
              value={row.name}
              disabled={row.disabledReason !== ""}
              onSelect={() => onSelectProfile(row.name)}
              data-testid={`profile-switcher-option-${row.name}`}
              aria-current={row.current && !aggregate ? "true" : undefined}
            >
              <ProfileGlyph
                size="sm"
                name={row.name}
                color={row.color}
                current={row.current && !aggregate}
                needsSetup={row.needsSetup}
              />
              <span className="truncate">{row.name}</span>
              {row.disabledReason !== "" ? (
                <span className="ml-auto shrink-0 text-micro font-medium text-warning">
                  {row.disabledReason}
                </span>
              ) : null}
              {row.current && !aggregate ? (
                <Check aria-hidden="true" className="ml-auto size-3 shrink-0 text-fg-strong" />
              ) : null}
            </CommandItem>
          ))}
        </CommandGroup>
        <CommandSeparator />
        <CommandGroup>
          <CommandItem
            value="__all-profiles"
            onSelect={onSelectAggregate}
            data-testid="profile-switcher-all"
            aria-pressed={aggregate}
          >
            <ProfileGlyph size="sm" aggregate name="All profiles" />
            <span>All profiles</span>
            {aggregate ? (
              <Check aria-hidden="true" className="ml-auto size-3 shrink-0 text-fg-strong" />
            ) : null}
          </CommandItem>
          <CommandItem
            value="__create-profile"
            onSelect={onCreate}
            data-testid="profile-switcher-create"
          >
            <Plus aria-hidden="true" className="size-3.5 shrink-0" />
            <span>Create profile…</span>
          </CommandItem>
        </CommandGroup>
      </CommandList>
      {archivedCount > 0 ? (
        <div className="flex items-center gap-1.5 px-2.5 pt-1 pb-1 text-micro text-faint">
          <span>{archivedCount} archived</span>
          <span aria-hidden="true">·</span>
          <button
            type="button"
            onClick={onOpenSettings}
            data-testid="profile-switcher-settings-link"
            className={cn(
              "text-muted underline underline-offset-2 outline-none",
              "hover:text-fg focus-visible:shadow-focus-ring"
            )}
          >
            Settings
          </button>
        </div>
      ) : null}
      <p className="mt-1 border-t border-line px-2.5 pt-2 pb-1 text-micro leading-normal text-subtle">
        {PROFILE_BOUNDARY_ANSWER}
      </p>
    </Command>
  );
}
