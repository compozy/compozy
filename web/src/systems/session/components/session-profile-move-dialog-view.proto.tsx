import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { SESSION_PROFILE_MOVE_DIALOG_STATES } from "../mocks/session-profile-move-fixtures";
import { SessionProfileMoveDialogView } from "./session-profile-move-dialog-view";

export const meta = definePrototype({
  title: "Move session to profile",
  route: "/sessions/$id",
  surface: "Session row menu",
  states: [
    { id: "ready", label: "Ready, with worktree" },
    { id: "no-worktree", label: "Ready, no worktree" },
    { id: "running", label: "Running session refused" },
    { id: "has-children", label: "Session with children refused" },
    { id: "daemon-refused", label: "Daemon refusal" },
    { id: "moving", label: "Moving" },
  ],
});

const ignore = () => undefined;

export default function SessionProfileMoveDialogPrototype({ state }: PrototypeProps) {
  const fixture =
    SESSION_PROFILE_MOVE_DIALOG_STATES[state] ?? SESSION_PROFILE_MOVE_DIALOG_STATES.ready;
  if (!fixture) throw new Error("No Move to profile fixtures");
  return <SessionProfileMoveDialogView {...fixture} onOpenChange={ignore} onSubmit={ignore} open />;
}
