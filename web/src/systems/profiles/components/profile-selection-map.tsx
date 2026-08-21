import type { ProfilePayload, ProfileSelectionPayload } from "../types";
import { ProfileGlyph } from "./profile-glyph";

export interface ProfileSelectionMapProps {
  selections: readonly ProfileSelectionPayload[];
  profiles: readonly ProfilePayload[];
  /** Resolves a workspace id to the name the operator knows it by. */
  projectName: (workspaceId: string) => string;
}

/**
 * Where each profile is active.
 *
 * Read-only facts. Switching happens in the menubar, so this stays an
 * inspection surface rather than growing a second set of controls.
 */
export function ProfileSelectionMap({
  selections,
  profiles,
  projectName,
}: ProfileSelectionMapProps) {
  if (selections.length === 0) {
    return (
      <p className="px-1 py-2 text-small-body text-subtle" data-testid="profiles-selection-empty">
        No project has a remembered profile yet.
      </p>
    );
  }

  return (
    <div className="flex flex-col" data-testid="profiles-selection-map">
      {selections.map(selection => {
        const key = `${selection.scope}:${selection.workspace_id ?? ""}`;
        const owner = profiles.find(profile => profile.name === selection.profile);
        const label =
          selection.scope === "global"
            ? "Across projects"
            : projectName(selection.workspace_id ?? "");
        return (
          <div
            key={key}
            className="flex min-h-8 items-center gap-3 border-t border-line-soft py-1 first:border-t-0"
            data-testid={`profiles-selection-row-${key}`}
          >
            <span className="truncate text-small-body text-muted">{label}</span>
            <span className="ml-auto flex shrink-0 items-center gap-1.5">
              <ProfileGlyph
                decorative
                size="sm"
                name={selection.profile}
                {...(owner ? { color: owner.color, icon: owner.icon, emoji: owner.emoji } : {})}
              />
              <span className="text-small-body text-fg">{selection.profile}</span>
            </span>
          </div>
        );
      })}
    </div>
  );
}
