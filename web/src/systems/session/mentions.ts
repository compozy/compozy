// Composer `@` mention surface — picker data sources and browser-local recents.
export {
  searchSessionFiles,
  type SessionFileEntry,
  type SessionFilesResponse,
} from "./adapters/session-files-api";
export {
  sessionFileSearchOptions,
  SESSION_FILE_SEARCH_LIMIT,
} from "./lib/session-file-search-options";
export {
  readSessionMentionRecents,
  sessionMentionRecentsKey,
  withSessionMentionRecent,
  writeSessionMentionRecents,
  SESSION_MENTION_RECENTS_LIMIT,
  type SessionMentionRecent,
} from "./lib/session-mention-recents";
export {
  SESSION_MENTION_DEBOUNCE_MS,
  useSessionFileSearch,
  type SessionFileSearchView,
  type SessionMentionSourceStatus,
} from "./hooks/use-session-file-search";
export {
  SESSION_MENTION_AGENT_LIMIT,
  useSessionMentionAgents,
  type SessionMentionAgentsView,
} from "./hooks/use-session-mention-agents";
export {
  SESSION_MENTION_SESSION_LIMIT,
  useSessionMentionSessions,
  type SessionMentionSessionsView,
} from "./hooks/use-session-mention-sessions";
