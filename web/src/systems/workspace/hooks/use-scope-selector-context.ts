import { use } from "react";

import {
  ScopeSelectorContext,
  type ScopeSelectorContextValue,
} from "../contexts/scope-selector-context-value";

export function useScopeSelectorContext(): ScopeSelectorContextValue | null {
  return use(ScopeSelectorContext);
}
