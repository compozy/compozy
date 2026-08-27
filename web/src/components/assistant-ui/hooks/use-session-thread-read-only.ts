import { use } from "react";

import { SessionThreadReadOnlyContext } from "../session-thread-read-only-context";

export function useSessionThreadReadOnly(): boolean {
  return use(SessionThreadReadOnlyContext);
}
