import { Check, Palette, Plus } from "lucide-react";

import {
  Button,
  cn,
  Command,
  CommandGroup,
  CommandItem,
  CommandList,
  CommandSeparator,
  Spinner,
} from "@compozy/ui";

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
  onEditProfile?: (name: string) => void;
  onOpenSettings: () => void;
  manageable?: boolean;
  isLoading?: boolean;
  error?: Error | null;
  onRetry?: () => void;
}

export function ProfileSwitcherMenu({
  rows,
  aggregate,
  archivedCount,
  onSelectProfile,
  onSelectAggregate,
  onCreate,
  onEditProfile,
  onOpenSettings,
  manageable = true,
  isLoading = false,
  error = null,
  onRetry,
}: ProfileSwitcherMenuProps) {
  const showAggregate = aggregate || rows.length > 1 || archivedCount > 0;

  return (
    <Command className="bg-transparent p-0 shadow-none">
      <CommandList className="max-h-none">
        {isLoading ? (
          <div className="flex items-center justify-center px-2 py-4" role="status">
            <Spinner className="size-4 text-subtle" />
          </div>
        ) : error ? (
          <div
            className="flex flex-col items-center gap-2 px-2 py-4 text-center text-small-body text-danger"
            role="alert"
          >
            <span>{error.message}</span>
            {onRetry ? (
              <Button onClick={onRetry} size="sm" type="button" variant="ghost">
                Retry
              </Button>
            ) : null}
          </div>
        ) : null}
        <CommandGroup className="p-0">
          {rows.map(row => (
            <CommandItem
              key={row.name}
              value={row.name}
              disabled={!manageable || row.disabledReason !== ""}
              onSelect={() => {
                if (manageable) onSelectProfile(row.name);
              }}
              data-testid={`profile-switcher-option-${row.name}`}
              aria-current={row.current && !aggregate ? "true" : undefined}
              className="group/profile-row"
            >
              <ProfileGlyph
                decorative
                size="sm"
                name={row.name}
                color={row.color}
                icon={row.icon}
                emoji={row.emoji}
                current={row.current && !aggregate}
                needsSetup={row.needsSetup}
              />
              <span className="truncate">{row.name}</span>
              {row.disabledReason !== "" ? (
                <span className="ml-auto shrink-0 text-micro font-medium text-warning">
                  {row.disabledReason}
                </span>
              ) : manageable && onEditProfile ? (
                // -my-1 keeps the 24px edit target from stretching the row past
                // sibling-menu rhythm; check and edit share one cell so the row
                // never reserves phantom trailing space.
                <span className="relative -my-1 ml-auto grid size-button-icon-xs shrink-0 place-items-center">
                  {row.current && !aggregate ? (
                    <Check
                      aria-hidden="true"
                      className={cn(
                        "size-3 shrink-0 text-accent transition-opacity",
                        "group-hover/profile-row:opacity-0 group-data-[selected=true]/profile-row:opacity-0"
                      )}
                    />
                  ) : null}
                  <Button
                    size="icon-xs"
                    variant="ghost"
                    aria-label={`Edit identity for ${row.name}`}
                    data-testid={`profile-switcher-edit-${row.name}`}
                    onClick={event => {
                      event.stopPropagation();
                      onEditProfile(row.name);
                    }}
                    className={cn(
                      "absolute inset-0 opacity-0 transition-opacity focus-visible:opacity-100",
                      "group-hover/profile-row:opacity-100 group-data-[selected=true]/profile-row:opacity-100"
                    )}
                  >
                    <Palette aria-hidden="true" />
                  </Button>
                </span>
              ) : row.current && !aggregate ? (
                <Check aria-hidden="true" className="ml-auto size-3 shrink-0 text-accent" />
              ) : null}
            </CommandItem>
          ))}
        </CommandGroup>
        <CommandSeparator className="my-1" />
        <CommandGroup className="p-0">
          {showAggregate ? (
            <CommandItem
              value="__all-profiles"
              onSelect={onSelectAggregate}
              data-testid="profile-switcher-all"
              aria-current={aggregate ? "true" : undefined}
            >
              <ProfileGlyph decorative size="sm" aggregate name="All profiles" />
              <span>All profiles</span>
              {aggregate ? (
                <Check aria-hidden="true" className="ml-auto size-3 shrink-0 text-accent" />
              ) : null}
            </CommandItem>
          ) : null}
          {manageable ? (
            <CommandItem
              value="__create-profile"
              onSelect={onCreate}
              data-testid="profile-switcher-create"
            >
              <span
                aria-hidden="true"
                className="grid size-profile-glyph-sm shrink-0 place-items-center"
              >
                <Plus className="size-3.5 text-subtle" />
              </span>
              <span>Create profile…</span>
            </CommandItem>
          ) : null}
        </CommandGroup>
      </CommandList>
      {archivedCount > 0 ? (
        <div className="flex items-center gap-1.5 px-2 pt-1 pb-1 text-micro text-faint">
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
      <p className="-mx-1 mt-1 border-t border-line px-3 pt-2 pb-1.5 text-micro leading-normal text-subtle">
        {PROFILE_BOUNDARY_ANSWER}
      </p>
    </Command>
  );
}
