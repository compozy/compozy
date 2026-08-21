import { Archive, Palette, TriangleAlert } from "lucide-react";

import { Button, Pill } from "@compozy/ui";

import { workItemsLabel } from "../lib/profile-copy";
import { canDelete, PERMANENT_PROFILE } from "../lib/profile-rows";
import type { ProfilePayload } from "../types";
import { ProfileGlyph } from "./profile-glyph";

export interface ProfileSettingsListProps {
  profiles: readonly ProfilePayload[];
  currentName: string;
  /** Remote surfaces read the list but never manage it (US-032.AC-2). */
  manageable: boolean;
  onEditIdentity: (name: string) => void;
  onRename: (name: string) => void;
  onArchive: (name: string) => void;
  onUnarchive: (name: string) => void;
  onDelete: (name: string) => void;
  variant: "active" | "archived";
}

const ROW_CLASS =
  "flex min-h-[var(--height-profile-row)] items-center gap-3 border-t border-line-soft px-3 first:border-t-0";

/** Profile rows whose controls mirror the daemon's lifecycle rules. */
export function ProfileSettingsList({
  profiles,
  currentName,
  manageable,
  onEditIdentity,
  onRename,
  onArchive,
  onUnarchive,
  onDelete,
  variant,
}: ProfileSettingsListProps) {
  return (
    <div
      className="overflow-hidden rounded-md border border-line bg-canvas-soft"
      data-testid={`profiles-${variant}-list`}
    >
      {profiles.map(profile => {
        const permanent = profile.name === PERMANENT_PROFILE;
        const archived = variant === "archived";
        return (
          <div key={profile.id} className={ROW_CLASS} data-testid={`profile-row-${profile.name}`}>
            <ProfileGlyph
              decorative
              size="lg"
              name={profile.name}
              color={profile.color}
              icon={profile.icon}
              emoji={profile.emoji}
              current={profile.name === currentName}
              needsSetup={profile.needs_setup === true}
            />
            <span className="truncate text-small-body font-medium text-fg">{profile.name}</span>
            {permanent ? (
              <Pill tone="neutral" size="xs">
                Permanent
              </Pill>
            ) : null}
            {archived ? (
              <Pill tone="info" size="xs">
                <Archive aria-hidden="true" />
                Archived
              </Pill>
            ) : null}
            {profile.needs_setup === true ? (
              <Pill tone="warning" size="xs" data-testid={`profile-needs-setup-${profile.name}`}>
                <TriangleAlert aria-hidden="true" />
                Needs setup
              </Pill>
            ) : null}
            <span className="ml-auto shrink-0 text-small-body tabular-nums text-subtle">
              {workItemsLabel(profile.work_items ?? 0)}
            </span>
            {manageable ? (
              <div className="flex shrink-0 items-center gap-1.5">
                <Button
                  size="icon-sm"
                  variant="ghost"
                  aria-label={`Edit identity for ${profile.name}`}
                  onClick={() => onEditIdentity(profile.name)}
                  data-testid={`profile-edit-identity-${profile.name}`}
                >
                  <Palette aria-hidden="true" />
                </Button>
                {archived ? (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => onUnarchive(profile.name)}
                    data-testid={`profile-unarchive-${profile.name}`}
                  >
                    Unarchive
                  </Button>
                ) : (
                  <>
                    {permanent ? null : (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => onRename(profile.name)}
                        data-testid={`profile-rename-${profile.name}`}
                      >
                        Rename
                      </Button>
                    )}
                    {permanent ? null : (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => onArchive(profile.name)}
                        data-testid={`profile-archive-${profile.name}`}
                      >
                        Archive
                      </Button>
                    )}
                  </>
                )}
                {canDelete(profile) ? (
                  <Button
                    size="sm"
                    variant="destructive"
                    onClick={() => onDelete(profile.name)}
                    data-testid={`profile-delete-${profile.name}`}
                  >
                    Delete
                  </Button>
                ) : null}
              </div>
            ) : null}
          </div>
        );
      })}
    </div>
  );
}
