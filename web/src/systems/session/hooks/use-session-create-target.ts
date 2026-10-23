import { useState } from "react";

import { useProfileReadScope, useProfiles } from "@/systems/profiles";
import {
  DESTINATION_COPY,
  useActiveWorkspace,
  type FocusedDataScope,
  type DataScope,
  type WorkspacePayload,
} from "@/systems/workspace";

import {
  prefillSessionProject,
  readLastSessionProject,
  sessionProfileOptions,
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
  seed,
}: {
  resetKey: unknown;
  focusedBinding?: string | null;
  /** The destination the opening surface named; each opening starts its picks from it. */
  seed?: { project: string | null; profile: string | null };
}): SessionCreateTarget {
  const active = useActiveWorkspace();
  const profileScope = useProfileReadScope();
  const projectAxis = active.breadthMode === "all" && !active.pending;
  const profileAxis = profileScope.aggregate;
  const profiles = useProfiles(profileAxis);
  const opening = { key: resetKey, project: seed?.project ?? null, profile: seed?.profile ?? null };
  const [picks, setPicks] = useState<Picks>(opening);
  const current = picks.key === resetKey ? picks : opening;
  if (current !== picks) setPicks(current);

  const projectOptions = projectAxis ? sessionProjectOptions(active.projectWorkspaces) : [];
  const projectValue = resolveProjectPick(
    projectAxis,
    projectOptions,
    current.project,
    focusedBinding
  );
  const profileOptions = profileAxis ? sessionProfileOptions(profiles.data ?? []) : [];
  const profile =
    profileOptions.find(option => option.value === current.profile)?.value ??
    profileScope.destination;

  const scope = resolveCreateScope(projectValue, active.dataScope);
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

function resolveProjectPick(
  enabled: boolean,
  options: SessionTargetOption[],
  selected: string | null,
  focusedBinding: string | null | undefined
): string | null {
  if (!enabled) return null;
  return (
    options.find(option => option.value === selected)?.value ??
    prefillSessionProject(options, { focusedBinding, lastUsed: readLastSessionProject() })
  );
}

function resolveCreateScope(
  project: string | null,
  focus: DataScope | null
): FocusedDataScope | null {
  if (project !== null) return sessionTargetScope(project);
  return focus && focus.kind !== "all" ? focus : null;
}
