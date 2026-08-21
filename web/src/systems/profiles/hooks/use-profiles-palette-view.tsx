import { Layers, Plus } from "lucide-react";

import { ProfilePaletteRow } from "../components/profile-palette-row";
import {
  activeProfiles,
  archivedProfiles,
  PERMANENT_PROFILE,
  toProfileRows,
  type ProfileRow,
} from "../lib/profile-rows";
import { openProfileDialog } from "../stores/profile-dialog-store";
import type { ProfileLens } from "../types";
import { useActiveProfileView, useSwitchProfile } from "./use-profile-selection";
import { useProfiles } from "./use-profiles";

/**
 * Row contract of the palette view stack, restated structurally.
 *
 * The `os` system owns the shell; this controller only has to produce rows in
 * its shape. Restating the fields keeps `profiles` from importing the OS
 * palette, which would close an import cycle back through this barrel.
 */
export interface ProfilesPaletteViewRow {
  readonly value: string;
  readonly testId: string;
  readonly node: React.ReactNode;
  readonly disabled?: boolean;
  onSelect(): void;
}

export interface ProfilesPaletteViewContent {
  readonly rows: readonly ProfilesPaletteViewRow[];
  readonly header: React.ReactNode | null;
  readonly empty: React.ReactNode;
  readonly note: React.ReactNode | null;
  readonly backHint: string;
  readonly resetKey: string;
  onEmptyQueryBackspace(): boolean;
}

export interface UseProfilesPaletteViewInput {
  query: string;
  lens: ProfileLens;
  onDismiss(): void;
}

function matches(row: ProfileRow, query: string): boolean {
  const needle = query.trim().toLowerCase();
  return needle === "" || row.name.toLowerCase().includes(needle);
}

function rowNode(row: ProfileRow, meta?: string) {
  return <ProfilePaletteRow row={row} {...(meta ? { meta } : {})} />;
}

/**
 * The Profiles palette view.
 *
 * Switching happens here because it is the default read — "which exist, which is
 * current, switch where". Everything that mutates a profile hands off to the
 * canonical dialogs instead, so this view owns no plan, no revision, and no
 * mutation state of its own (ADR-016).
 */
export function useProfilesPaletteView({
  query,
  lens,
  onDismiss,
}: UseProfilesPaletteViewInput): ProfilesPaletteViewContent {
  const profiles = useProfiles();
  const view = useActiveProfileView(lens);
  const switchProfile = useSwitchProfile(lens);

  const all = profiles.data ?? [];
  const currentName = view.kind === "profile" ? view.profile : PERMANENT_PROFILE;
  const active = toProfileRows(activeProfiles(all), currentName).filter(row => matches(row, query));
  const archived = toProfileRows(archivedProfiles(all), currentName).filter(row =>
    matches(row, query)
  );

  const rows: ProfilesPaletteViewRow[] = [
    ...active.map(row => ({
      value: `profile:${row.name}`,
      testId: `os-palette-profile-${row.name}`,
      node: rowNode(row, row.current ? "current" : undefined),
      disabled: row.disabledReason !== "",
      onSelect: () => {
        if (row.disabledReason !== "") return;
        onDismiss();
        switchProfile.mutate({ kind: "profile", profile: row.name });
      },
    })),
    ...archived.map(row => ({
      value: `archived:${row.name}`,
      testId: `os-palette-profile-${row.name}`,
      node: rowNode(row, "unarchive"),
      onSelect: () => {
        onDismiss();
        openProfileDialog({ flow: "unarchive", profile: row.name });
      },
    })),
    {
      value: "action:create",
      testId: "os-palette-profile-create",
      node: (
        <span className="flex items-center gap-2">
          <Plus aria-hidden="true" className="size-3.5 shrink-0" />
          <span>Create profile…</span>
        </span>
      ),
      onSelect: () => {
        onDismiss();
        openProfileDialog({ flow: "create" });
      },
    },
    {
      value: "action:aggregate",
      testId: "os-palette-profile-aggregate",
      node: (
        <span className="flex items-center gap-2">
          <Layers aria-hidden="true" className="size-3.5 shrink-0" />
          <span>Show all profiles</span>
        </span>
      ),
      onSelect: () => {
        onDismiss();
        switchProfile.mutate({ kind: "aggregate" });
      },
    },
  ];

  return {
    rows,
    header: null,
    empty: <span>No profiles match "{query.trim()}".</span>,
    note: null,
    backHint: "Profiles",
    resetKey: `${currentName}|${all.length}|${archived.length}`,
    onEmptyQueryBackspace: () => false,
  };
}
