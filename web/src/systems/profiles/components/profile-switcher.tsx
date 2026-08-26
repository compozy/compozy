import { useState } from "react";
import { ChevronsUpDown, UserRound } from "lucide-react";

import { cn, CommandSelect, PopoverContent, PopoverTrigger } from "@compozy/ui";

import type { ProfileRow } from "../lib/profile-rows";
import { ProfileGlyph } from "./profile-glyph";
import { ProfileSwitcherMenu } from "./profile-switcher-menu";

export interface ProfileSwitcherProps {
  rows: readonly ProfileRow[];
  /** Whose work is showing: a profile name, or the aggregate. */
  activeName: string;
  aggregate: boolean;
  /** With only `default`, the trigger is a neutral icon button (US-007.EC-2). */
  quiet: boolean;
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

/** Menubar profile switcher, quiet until a second active profile exists. */
export function ProfileSwitcher({
  rows,
  activeName,
  aggregate,
  quiet,
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
}: ProfileSwitcherProps) {
  const [open, setOpen] = useState(false);
  const active = rows.find(row => row.name === activeName);

  const close = (run: () => void) => () => {
    setOpen(false);
    run();
  };

  return (
    <CommandSelect open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={
          <button
            type="button"
            data-slot="os-menubar-profile"
            data-testid="os-menubar-profile"
            aria-label={quiet ? "Profile" : `Profile: ${aggregate ? "All profiles" : activeName}`}
            className={cn(
              "flex shrink-0 items-center gap-menubar-workspace-gap rounded-md text-muted outline-none",
              "hover:bg-btn-default-fill focus-visible:shadow-focus-ring data-[popup-open]:bg-btn-default-fill",
              quiet ? "grid size-7 place-items-center" : "h-7 px-2"
            )}
          />
        }
      >
        {quiet ? (
          <UserRound aria-hidden="true" className="size-3.5" strokeWidth={1.75} />
        ) : (
          <>
            <ProfileGlyph
              decorative
              size="sm"
              name={aggregate ? "All profiles" : activeName}
              aggregate={aggregate}
              {...(active ? { color: active.color, icon: active.icon, emoji: active.emoji } : {})}
            />
            <span className="max-w-32 truncate text-small-body font-semibold text-fg-strong">
              {aggregate ? "All profiles" : activeName}
            </span>
            <ChevronsUpDown aria-hidden="true" className="size-3 shrink-0 text-subtle" />
          </>
        )}
      </PopoverTrigger>
      <PopoverContent
        align="end"
        className="w-70 p-1"
        data-testid="os-menubar-profile-menu"
        aria-label="Profiles"
      >
        <ProfileSwitcherMenu
          rows={rows}
          aggregate={aggregate}
          archivedCount={archivedCount}
          onSelectProfile={name => close(() => onSelectProfile(name))()}
          onSelectAggregate={close(onSelectAggregate)}
          onCreate={close(onCreate)}
          {...(onEditProfile
            ? { onEditProfile: (name: string) => close(() => onEditProfile(name))() }
            : {})}
          onOpenSettings={close(onOpenSettings)}
          manageable={manageable}
          isLoading={isLoading}
          error={error}
          onRetry={onRetry}
        />
      </PopoverContent>
    </CommandSelect>
  );
}
