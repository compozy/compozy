import { use } from "react";

import {
  SessionWindowPresentationContext,
  type SessionWindowPresentation,
} from "../contexts/session-window-presentation-context";

/** Where this session view is shown: its window body, or the board drawer (#1052). */
export function useSessionWindowPresentation(): SessionWindowPresentation {
  return use(SessionWindowPresentationContext);
}
