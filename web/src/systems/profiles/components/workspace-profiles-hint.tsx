import { Info } from "lucide-react";
import { useState } from "react";

import { Button } from "@compozy/ui";

import type { WorkspaceDetailPayload } from "@/systems/workspace";

import { openProfileDialog } from "../stores/profile-dialog-store";
import { useProfiles } from "../hooks/use-profiles";

type WorkspaceProfileHint = NonNullable<WorkspaceDetailPayload["profile_hints"]>[number];

export interface WorkspaceProfilesHintProps {
  hints: readonly WorkspaceProfileHint[];
  workspaceId: string;
}

/** Offers the canonical create flow for dormant repository profile folders. */
export function WorkspaceProfilesHint({ hints, workspaceId }: WorkspaceProfilesHintProps) {
  const profiles = useProfiles();
  const [dismissedWorkspace, setDismissedWorkspace] = useState<string | null>(null);
  const known = new Set((profiles.data ?? []).map(profile => profile.name));
  const absent = hints.filter(hint => !known.has(hint.name));

  if (absent.length === 0 || dismissedWorkspace === workspaceId) return null;

  return (
    <aside
      className="absolute top-3 left-1/2 z-30 flex w-[min(42rem,calc(100%-1.5rem))] -translate-x-1/2 items-start gap-2 rounded-md border border-line bg-canvas px-3 py-2.5"
      data-testid="workspace-profiles-hint"
    >
      <Info aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-info" />
      <div className="flex min-w-0 flex-1 flex-wrap items-center gap-2">
        <span className="mr-auto text-sm text-fg">{workspaceProfileHintLabel(absent)}</span>
        {absent.map(hint => (
          <Button
            data-testid={`workspace-profiles-hint-create-${hint.name}`}
            key={hint.name}
            onClick={() => openProfileDialog({ flow: "create", profile: hint.name })}
            size="sm"
            type="button"
            variant="neutral"
          >
            Create {hint.name}
          </Button>
        ))}
        <Button
          onClick={() => setDismissedWorkspace(workspaceId)}
          size="sm"
          type="button"
          variant="ghost"
        >
          Not now
        </Button>
      </div>
    </aside>
  );
}

function workspaceProfileHintLabel(hints: readonly WorkspaceProfileHint[]): string {
  const names = hints.map(hint => hint.name);
  if (names.length === 1) return `This project declares content for profile ${names[0]}.`;
  const last = names.at(-1);
  return `This project declares content for profiles ${names.slice(0, -1).join(", ")} and ${last}.`;
}
