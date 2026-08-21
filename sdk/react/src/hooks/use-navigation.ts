import type { NavigationController } from "../runtime-context.js";
import { useViewRuntime } from "./use-view-runtime.js";

export function useNavigation(): NavigationController {
  return useViewRuntime().navigation;
}
