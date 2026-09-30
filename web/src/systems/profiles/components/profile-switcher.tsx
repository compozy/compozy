import { useRef, useState } from "react";
import { UserRound } from "lucide-react";

import {
  cn,
  CommandSelect,
  PopoverContent,
  PopoverTrigger,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";

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

/**
 * Rail-foot profile switcher (D6): a 40px rail item, quiet (a neutral person
 * glyph) until a second active profile exists, then the active identity glyph.
 * The name rides in the accessible name and the tooltip beside the rail.
 */
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
  const menuRef = useRef<HTMLDivElement | null>(null);
  const active = rows.find(row => row.name === activeName);
  const shownName = aggregate ? "All profiles" : activeName;

  const close = (run: () => void) => () => {
    setOpen(false);
    run();
  };

  return (
    <CommandSelect open={open} onOpenChange={setOpen}>
      <Tooltip>
        <PopoverTrigger
          render={
            <TooltipTrigger
              render={
                <button
                  type="button"
                  data-slot="os-rail-profile"
                  data-testid="os-menubar-profile"
                  aria-label={quiet ? "Profile" : `Profile: ${shownName}`}
                  className={cn(
                    "grid size-rail-item shrink-0 place-items-center rounded-lg text-muted outline-none",
                    "transition-[background-color,color] duration-base ease-spring",
                    "hover:bg-surface-2 hover:text-fg focus-visible:shadow-focus-ring",
                    "data-[popup-open]:bg-surface-2 data-[popup-open]:text-fg"
                  )}
                />
              }
            />
          }
        >
          {quiet ? (
            <UserRound aria-hidden="true" className="size-5" strokeWidth={1.75} />
          ) : (
            <ProfileGlyph
              decorative
              name={shownName}
              aggregate={aggregate}
              {...(active ? { color: active.color, icon: active.icon, emoji: active.emoji } : {})}
            />
          )}
        </PopoverTrigger>
        <TooltipContent side="right" sideOffset={10}>
          {quiet ? "Profile" : shownName}
        </TooltipContent>
      </Tooltip>
      <PopoverContent
        side="right"
        align="end"
        className="w-70 p-1"
        data-testid="os-menubar-profile-menu"
        aria-label="Profiles"
        initialFocus={menuRef}
      >
        <ProfileSwitcherMenu
          ref={menuRef}
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
