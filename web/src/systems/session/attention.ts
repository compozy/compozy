/**
 * The session system's attention surface, gathered in one place: the badge
 * dictionary and its rendered scales, pending-interaction reads, the operator's
 * list preferences, and the presence lease.
 *
 * These belong together because they answer one question — does this session
 * need the operator, and how do we say so — and grouping them keeps the public
 * barrel readable as a surface contract rather than a flat list.
 */
export {
  isFinishedBadge,
  isNeedsYouBadge,
  SESSION_BADGE_SIGNAL,
  SESSION_BADGES,
  sessionAttentionClass,
  sessionBadgeSignal,
  toSessionBadge,
} from "./lib/session-badge";
export type {
  SessionAttentionClass,
  SessionBadgeShape,
  SessionBadgeSignal,
  SessionBadgeToken,
} from "./lib/session-badge";
export { sessionBadgeWordClass } from "./lib/session-badge-classes";
export {
  maskedAttentionNote,
  pendingClarifyCount,
  pendingInteractionReason,
  pendingInteractions,
  pendingPermissionCount,
} from "./lib/session-pending-interactions";
export {
  SessionBadgeGlyph,
  SessionBadgeMark,
  type SessionBadgeGlyphProps,
  type SessionBadgeMarkProps,
} from "./components/session-badge-mark";
export {
  DEFAULT_SESSION_LIST_PREFERENCES,
  SESSION_LIST_SCOPES,
  SESSION_LIST_SORTS,
  sessionListSortParam,
  toSessionListScope,
  toSessionListSort,
} from "./lib/session-list-preferences";
export type {
  SessionListPreferences,
  SessionListScope,
  SessionListSort,
} from "./lib/session-list-preferences";
export {
  acquireSessionPresence,
  releaseSessionPresence,
  renewSessionPresence,
} from "./adapters/session-presence-api";
export { useSessionPresence } from "./hooks/use-session-presence";
export {
  useSessionListPreferences,
  type SessionListPreferencesModel,
} from "./hooks/use-session-list-preferences";
export { useSessionListView, type SessionListViewModel } from "./hooks/use-session-list-view";
export {
  useWorkspaceSessionGroups,
  type WorkspaceSessionGroup,
} from "./hooks/use-workspace-session-groups";
