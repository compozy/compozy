import { createContext } from "react";

/**
 * Takes the reader to another message of this transcript by its thread message
 * id (#760: "Reply to" a message). Provided by the transcript viewport, which
 * owns scrolling and opens a collapsed group that hides the target; absent
 * outside one (stories, read-only rows), where the correlation shows no link.
 */
export type SessionMessageJump = (threadMessageId: string) => void;

export const SessionMessageJumpContext = createContext<SessionMessageJump | null>(null);
