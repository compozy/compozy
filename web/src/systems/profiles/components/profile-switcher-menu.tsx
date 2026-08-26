import { Check, Plus } from "lucide-react";

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
  onOpenSettings,
  manageable = true,
  isLoading = false,
  error = null,
  onRetry,
}: ProfileSwitcherMenuProps) {
  const showAggregate = aggregate || rows.length > 1 || archivedCount > 0;

  return (
    <Command>
      <CommandList className="max-h-none">
        {isLoading ? (
          <div className="flex items-center justify-center px-3 py-4" role="status">
            <Spinner className="size-4 text-subtle" />
          </div>
        ) : error ? (
          <div
            className="flex flex-col items-center gap-2 px-3 py-4 text-center text-small-body text-danger"
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
        <CommandGroup>
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
              ) : null}
              {row.current && !aggregate ? (
                <Check aria-hidden="true" className="ml-auto size-3 shrink-0 text-fg-strong" />
              ) : null}
            </CommandItem>
          ))}
        </CommandGroup>
        <CommandSeparator />
        <CommandGroup>
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
                <Check aria-hidden="true" className="ml-auto size-3 shrink-0 text-fg-strong" />
              ) : null}
            </CommandItem>
          ) : null}
          {manageable ? (
            <Button
              type="button"
              variant="ghost"
              onClick={onCreate}
              onKeyDown={event => {
                if (event.key === "Enter" || event.key === " ") event.stopPropagation();
              }}
              data-testid="profile-switcher-create"
              className="h-auto w-full justify-start gap-2 px-2 py-1.5 text-small-body font-normal tracking-normal"
            >
              <Plus aria-hidden="true" className="size-3.5 shrink-0" />
              <span>Create profile…</span>
            </Button>
          ) : null}
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
