// Types
export type {
  ArchiveProfilePlan,
  CreateProfileParams,
  DeleteProfilePlan,
  ProfileDialogIntent,
  ProfileLens,
  ProfileLifecycleFlow,
  ProfileOperation,
  ProfilePayload,
  ProfilePlacement,
  ProfileRemovalSummary,
  ProfileRepoCandidate,
  ProfileSelectionPayload,
  ProfileView,
  RenameProfilePlan,
  UnarchiveProfileResult,
  UpdateProfileParams,
} from "./types";
export { PROFILE_AGGREGATE } from "./types";

// Adapters
export { ProfileApiError } from "./adapters/profiles-api";

// Query infrastructure
export { profileKeys, profileLensKey } from "./lib/query-keys";
export {
  isAggregateView,
  ownerFromRow,
  profileScopeParams,
  profileViewKey,
  type ProfileListingScope,
  type ProfileMutationScopeParams,
  type ProfileOwner,
  type ProfileOwnerLabel,
  type ProfileScopeParams,
} from "./lib/profile-scope";
export {
  readProfileKey,
  readProfileLens,
  readProfileScopeParams,
  readProfileView,
} from "./lib/profile-scope-resolver";
export {
  archivePlanOptions,
  deletePlanOptions,
  profileDetailOptions,
  profileOperationsOptions,
  profileSelectionMapOptions,
  profileSelectionOptions,
  profilesListOptions,
  renamePlanOptions,
} from "./lib/query-options";

// Domain rules
export {
  activeProfiles,
  archivedProfiles,
  actingProfile,
  canDelete,
  isArchived,
  isQuiet,
  PERMANENT_PROFILE,
  toProfileRow,
  toProfileRows,
  viewLabel,
  type ProfileRow,
  type ProfileRowState,
} from "./lib/profile-rows";
export {
  ALL_PROFILES_LABEL,
  ARCHIVED_OWNER_SUFFIX,
  archivedFallbackToast,
  createdInProfileToast,
  emptyAcrossProfiles,
  emptyForScope,
  emptyInProfile,
  NAME_REQUIRED_MESSAGE,
  nameTakenMessage,
  ownedByProfileLine,
  switchToProfileLabel,
  PROFILE_BOUNDARY_ANSWER,
  PROFILE_PERMANENT_LINE,
  PROFILE_REMOTE_MANAGEMENT_LINE,
  PROFILE_SEPARATION_LINE,
  profileErrorMessage,
  workItemsLabel,
} from "./lib/profile-copy";
export {
  PROFILE_EMOJI_OPTIONS,
  PROFILE_ICON_OPTIONS,
  PROFILE_ICON_REGISTRY,
  PROFILE_IDENTITY_SWATCHES,
  starterIdentity,
  symbolOf,
  symbolPatch,
} from "./lib/profile-identity";

// Live delivery
export {
  applyProfileEvent,
  openProfileEventStream,
  parseProfileEvent,
  PROFILE_EVENT_NAMES,
  PROFILE_LIFECYCLE_EVENTS,
  PROFILE_SELECTION_CHANGED_EVENT,
  profileEventStreamUrl,
  reconcileProfiles,
  type ProfileEvent,
  type ProfileEventSourceFactory,
  type ProfileStreamStatus,
} from "./lib/profile-event-stream";

// Stores
export {
  localProfileView,
  profileViewSelectors,
  profileViewStore,
  resetProfileViews,
  restoreProfileView,
  setProfileView,
  sweepProfileView,
} from "./stores/profile-view-store";
export {
  closeProfileDialog,
  openProfileDialog,
  profileDialogSelectors,
  profileDialogStore,
} from "./stores/profile-dialog-store";

// Hooks
export {
  useProfile,
  useProfileOperations,
  useProfileSelectionMap,
  useProfiles,
} from "./hooks/use-profiles";
export {
  useActiveProfileView,
  useRememberedProfile,
  useSwitchProfile,
} from "./hooks/use-profile-selection";
export { useArchivePlan, useDeletePlan, useRenamePlan } from "./hooks/use-profile-plans";
export {
  useArchiveProfile,
  useCreateProfile,
  useDeleteProfile,
  useRenameProfile,
  useUnarchiveProfile,
  useUpdateProfileIdentity,
} from "./hooks/use-profile-mutations";
export { useProfileEventStream } from "./hooks/use-profile-event-stream";
export {
  isStalePlan,
  lifecycleErrorMessage,
  useProfileLifecycle,
} from "./hooks/use-profile-lifecycle";
export { useProfileLens } from "./hooks/use-profile-lens";
export {
  useAggregateDestination,
  useProfileReadScope,
  type ProfileReadScope,
} from "./hooks/use-profile-read-scope";
export { useProfileFlowIntent, type ProfileFlowSearch } from "./hooks/use-profile-flow-intent";
export { useProfilesSettingsPage } from "./hooks/use-profiles-settings-page";
export { useProfileSwitcher } from "./hooks/use-profile-switcher";
export {
  useProfilesPaletteView,
  type ProfilesPaletteViewContent,
  type ProfilesPaletteViewRow,
} from "./hooks/use-profiles-palette-view";

// Components
export {
  ProfileGlyph,
  type ProfileGlyphProps,
  type ProfileGlyphSize,
} from "./components/profile-glyph";
export { ProfileSwitcher, type ProfileSwitcherProps } from "./components/profile-switcher";
export { ProfileSwitcherMenu } from "./components/profile-switcher-menu";
export { ProfileIdentityFields } from "./components/profile-identity-fields";
export { ProfileCreateDialog } from "./components/profile-create-dialog";
export { ProfileIdentityDialog } from "./components/profile-identity-dialog";
export { ProfileRenameDialog } from "./components/profile-rename-dialog";
export { ProfileRenamePlan } from "./components/profile-rename-plan";
export { ProfileArchiveDialog } from "./components/profile-archive-dialog";
export { ProfileUnarchiveDialog } from "./components/profile-unarchive-dialog";
export { ProfileDeleteDialog } from "./components/profile-delete-dialog";
export { ProfileLifecycleDialogs } from "./components/profile-lifecycle-dialogs";
export { ProfileLifecycleHost } from "./components/profile-lifecycle-host";
export {
  WorkspaceProfilesHint,
  type WorkspaceProfilesHintProps,
} from "./components/workspace-profiles-hint";
export { ProfileSwitcherSlot } from "./components/profile-switcher-slot";
export { ProfileSettingsList } from "./components/profile-settings-list";
export { ProfileSelectionMap } from "./components/profile-selection-map";
export { ProfilePaletteRow } from "./components/profile-palette-row";
export { ProfileOwnerTag, type ProfileOwnerTagProps } from "./components/profile-owner-tag";
export {
  ProfileDestinationChip,
  type ProfileDestinationChipProps,
} from "./components/profile-destination-chip";
export {
  ProfileOwnerBanner,
  type ProfileOwnerBannerProps,
} from "./components/profile-owner-banner";
