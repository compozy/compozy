import { useState } from "react";

import { useProfileReadScope, useProfiles } from "@/systems/profiles";
import {
  DESTINATION_COPY,
  useActiveWorkspace,
  type FocusedDataScope,
  type WorkspacePayload,
} from "@/systems/workspace";

import {
  prefillSessionProject,
  readLastSessionProject,
  sessionProjectOptions,
  sessionTargetScope,
  type SessionTargetOption,
} from "../lib/session-create-target";

/** Where a new session lands: the bounded destination, or the inline picks (ADR-003). */
export interface SessionCreateTarget {
  /** All projects is on: the Project picker is shown. */
  projectAxis: boolean;
  /** All profiles is on: the Profile picker is shown. */
  profileAxis: boolean;
  projectOptions: SessionTargetOption[];
  /** Picker value: a workspace id or `@user`; `null` outside All projects. */
  projectValue: string | null;
  profileOptions: SessionTargetOption[];
  profile: string;
  /** The session's scope; `null` while no target is known. */
  scope: FocusedDataScope | null;
  /** The project workspace the session runs in; `undefined` for User. */
  workspace: WorkspacePayload | undefined;
  /** Display name of the target project ("User" outside any project). */
  projectLabel: string;
  selectProject(value: string): void;
  selectProfile(value: string): void;
}

interface Picks {
  key: unknown;
  project: string | null;
  profile: string | null;
}

/**
 * The create target for one opening of a session surface. Each axis gets a
 * picker only while it is aggregated; a bounded axis is its own target. Picks
 * reset when `resetKey` changes (the surface reopened), so every opening
 * starts from the prefill.
 */
export function useSessionCreateTarget({
  resetKey,
  focusedBinding,
}: {
  resetKey: unknown;
  focusedBinding?: string | null;
}): SessionCreateTarget {
  const active = useActiveWorkspace();
  const profileScope = useProfileReadScope();
  const projectAxis = active.breadthMode === "all" && !active.pending;
  const profileAxis = profileScope.aggregate;
  const profiles = useProfiles(profileAxis);
  const [picks, setPicks] = useState<Picks>({ key: resetKey, project: null, profile: null });
  const current = picks.key === resetKey ? picks : { key: resetKey, project: null, profile: null };
  if (current !== picks) setPicks(current);

  const projectOptions = projectAxis ? sessionProjectOptions(active.projectWorkspaces) : [];
  const projectValue = projectAxis
    ? (projectOptions.find(option => option.value === current.project)?.value ??
      prefillSessionProject(projectOptions, {
        focusedBinding,
        lastUsed: readLastSessionProject(),
      }))
    : null;
  const profileOptions = profileAxis
    ? (profiles.data ?? [])
        .filter(owner => owner.archived_at == null)
        .map(owner => ({
          value: owner.name,
          label: owner.name,
          monogram: owner.name.slice(0, 2).toUpperCase(),
        }))
    : [];
  const profile =
    profileOptions.find(option => option.value === current.profile)?.value ??
    profileScope.destination;

  const focus = active.dataScope;
  const scope: FocusedDataScope | null =
    projectValue !== null
      ? sessionTargetScope(projectValue)
      : focus && focus.kind !== "all"
        ? focus
        : null;
  const workspace =
    scope?.kind === "project"
      ? active.projectWorkspaces.find(item => item.id === scope.workspaceId)
      : undefined;

  return {
    projectAxis,
    profileAxis,
    projectOptions,
    projectValue,
    profileOptions,
    profile,
    scope,
    workspace,
    projectLabel: workspace?.name ?? DESTINATION_COPY.user,
    selectProject: value => setPicks({ ...current, project: value }),
    selectProfile: value => setPicks({ ...current, profile: value }),
  };
}
