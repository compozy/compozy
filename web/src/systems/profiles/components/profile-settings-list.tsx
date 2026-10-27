import { Archive, Palette } from "lucide-react";

import { Button, Pill, Skeleton, SkeletonText } from "@compozy/ui";

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

const LIST_CLASS = "overflow-hidden rounded-lg bg-card shadow-card";
const ROW_CLASS =
  "flex min-h-[var(--height-profile-row)] items-center gap-3 border-t border-line-soft px-3 first:border-t-0";
const SKELETON_ROWS = ["first", "second", "third", "fourth"] as const;

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
    <div className={LIST_CLASS} data-testid={`profiles-${variant}-list`}>
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
            <span className="truncate text-body font-medium text-fg">{profile.name}</span>
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
              <Pill form="plain" tone="warning" data-testid={`profile-needs-setup-${profile.name}`}>
                <Pill.Dot />
                Needs setup
              </Pill>
            ) : null}
            <span className="ml-auto shrink-0 text-meta tabular-nums text-subtle">
              {workItemsLabel(profile.work_items ?? 0)}
            </span>
            {manageable ? (
              <div className="flex shrink-0 items-center gap-1.5">
                <Button
                  size="icon-sm"
                  variant="quiet"
                  aria-label={`Edit identity for ${profile.name}`}
                  onClick={() => onEditIdentity(profile.name)}
                  data-testid={`profile-edit-identity-${profile.name}`}
                >
                  <Palette aria-hidden="true" />
                </Button>
                {archived ? (
                  <Button
                    size="sm"
                    variant="secondary"
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
                        variant="secondary"
                        onClick={() => onRename(profile.name)}
                        data-testid={`profile-rename-${profile.name}`}
                      >
                        Rename
                      </Button>
                    )}
                    {permanent ? null : (
                      <Button
                        size="sm"
                        variant="secondary"
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

/**
 * The profile list in skeleton form: the real list frame and row, with a placeholder for the
 * identity glyph, the name, the work count and the lifecycle actions.
 */
export function ProfileSettingsListSkeleton() {
  return (
    <div aria-hidden="true" className={LIST_CLASS}>
      {SKELETON_ROWS.map(row => (
        <div className={ROW_CLASS} key={row}>
          <Skeleton className="size-7 shrink-0 rounded-md" />
          <SkeletonText className="w-28 text-body font-medium" />
          <SkeletonText className="ml-auto w-16 shrink-0 text-meta" />
          <div className="flex shrink-0 items-center gap-1.5">
            <Skeleton className="size-button-icon-sm rounded-pill" />
            <Skeleton className="h-button-sm w-16 rounded-pill" />
            <Skeleton className="h-button-sm w-16 rounded-pill" />
          </div>
        </div>
      ))}
    </div>
  );
}
