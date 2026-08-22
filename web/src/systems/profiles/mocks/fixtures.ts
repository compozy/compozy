import { ownerFromRow } from "../lib/profile-scope";
import type { ProfileListingScope, ProfileOwnerLabel } from "../lib/profile-scope";
import type {
  ArchiveProfilePlan,
  DeleteProfilePlan,
  ProfilePayload,
  ProfileSelectionPayload,
  RenameProfilePlan,
} from "../types";

const CREATED_AT = "2026-08-01T09:00:00Z";

function profile(overrides: Partial<ProfilePayload> & Pick<ProfilePayload, "id" | "name">) {
  return {
    color: "#8a8f98",
    created_at: CREATED_AT,
    emoji: null,
    icon: "user-round",
    state: "active",
    work_items: 0,
    ...overrides,
  } satisfies ProfilePayload;
}

export const defaultProfileFixture = profile({
  id: "00000000000000000000000000",
  name: "default",
  work_items: 10,
});

export const marketingProfileFixture = profile({
  id: "01J9MARKETING00000000000000",
  name: "marketing",
  color: "#c26ad6",
  icon: "megaphone",
  work_items: 4,
});

export const consultingProfileFixture = profile({
  id: "01J9CONSULTING000000000000",
  name: "consulting",
  color: "#4ea7fc",
  icon: "briefcase",
  work_items: 7,
});

export const growthProfileFixture = profile({
  id: "01J9GROWTH0000000000000000",
  name: "growth",
  color: "#4cb782",
  icon: "trending-up",
  needs_setup: true,
});

export const oldAgencyProfileFixture = profile({
  id: "01J9OLDAGENCY00000000000000".slice(0, 26),
  name: "old-agency",
  color: "#b58e5f",
  icon: "folder",
  state: "archived",
  archived_at: "2026-07-01T09:00:00Z",
  work_items: 4,
});

export const scratchProfileFixture = profile({
  id: "01J9SCRATCH000000000000000",
  name: "scratch",
  icon: "pencil",
  state: "archived",
  archived_at: "2026-07-04T09:00:00Z",
});

export const profileFixtures: ProfilePayload[] = [
  defaultProfileFixture,
  marketingProfileFixture,
  consultingProfileFixture,
  growthProfileFixture,
  oldAgencyProfileFixture,
  scratchProfileFixture,
];

export const profileSelectionFixtures: ProfileSelectionPayload[] = [
  { scope: "workspace", workspace_id: "ws-acme", profile: "marketing" },
  { scope: "workspace", workspace_id: "ws-client", profile: "consulting" },
  { scope: "global", profile: "default" },
];

export const renamePlanFixture: RenameProfilePlan = {
  revision: "pl_rename_001",
  machine_folders: ["~/.compozy/profiles/marketing/"],
  repo_candidates: [
    { workspace_id: "ws-acme", workspace: "acme-site", path: ".compozy/profiles/marketing/" },
    { workspace_id: "ws-client", workspace: "client-alpha", path: ".compozy/profiles/marketing/" },
  ],
  dormant_placements: [
    { extension: "growth-kit", resource: "skills/tweet-writer", profile: "marketing" },
  ],
  vault_ref_rewrites: 2,
};

export const archivePlanFixture: ArchiveProfilePlan = {
  revision: "pl_archive_001",
  running_sessions: [],
  approval_blockers: [],
  leased_runs: 0,
  queued_runs_to_freeze: 3,
  automations_to_pause: ["weekly-digest"],
};

export const archiveBlockedPlanFixture: ArchiveProfilePlan = {
  ...archivePlanFixture,
  revision: "pl_archive_blocked",
  running_sessions: ["draft-launch-email", "plan-spring-campaign"],
};

export const deletePlanFixture: DeleteProfilePlan = {
  revision: "pl_delete_001",
  removed: {
    agents: 1,
    skills: 2,
    loops: 0,
    mcp_servers: 1,
    config_keys: 1,
    credential_overrides: 0,
    memory_entries: 12,
    desktop_partitions: 1,
    palette_usage: 2,
    palette_query_hits: 3,
    palette_pins: 1,
    terminal_approvals: 0,
  },
  selections_to_sweep: 1,
  approval_blockers: [],
};

/** A scoped listing: no owner tags, and the empty state names `default`. */
export const scopedListingScopeFixture: ProfileListingScope = {
  aggregate: false,
  destination: "default",
  scopeLabel: "default",
  ownerOf: row => ownerFromRow(row),
};

/**
 * The aggregate listing. Rows carry whatever owner the daemon labelled them
 * with, so a mixed-owner fixture set renders mixed tags without extra wiring.
 */
export const aggregateListingScopeFixture: ProfileListingScope = {
  aggregate: true,
  destination: "default",
  scopeLabel: null,
  ownerOf: row => ownerFromRow(row),
};

/** Owner labels a fixture row can spread to belong to a named profile. */
export function profileOwnerLabels(profile: ProfilePayload): ProfileOwnerLabel {
  return {
    profile_id: profile.id,
    profile_name: profile.name,
    profile_color: profile.color,
    profile_icon: profile.icon ?? undefined,
    profile_archived: profile.state === "archived",
  };
}
