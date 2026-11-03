import type { SessionProfileMoveDialogViewProps } from "../components/session-profile-move-dialog-view";

type SessionProfileMoveFixture = Omit<
  SessionProfileMoveDialogViewProps,
  "open" | "onOpenChange" | "onSubmit"
>;

const base: SessionProfileMoveFixture = {
  title: "API cleanup",
  targetName: "work",
  profileName: "default",
  hasWorktree: true,
  refusal: null,
  error: null,
  pending: false,
};

/** Move to profile dialog states, by prototype state id. */
export const SESSION_PROFILE_MOVE_DIALOG_STATES: Record<string, SessionProfileMoveFixture> = {
  ready: base,
  "no-worktree": { ...base, title: "Release notes draft", hasWorktree: false },
  running: { ...base, title: "Docs sweep", refusal: "Stop this session before moving it." },
  "has-children": {
    ...base,
    refusal: "This session has child sessions. Make them top-level first.",
  },
  "daemon-refused": {
    ...base,
    error:
      "the session's worktree has 2 uncommitted file(s) on feat/api-cleanup; commit or discard them first",
  },
  moving: { ...base, pending: true },
};
