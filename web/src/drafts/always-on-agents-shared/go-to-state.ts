import { setParam } from "@protohq/cli";

/**
 * Moves the prototype to another state. In the dev render origin the optimized `@protohq/cli`
 * that drafts import and the source copy the render host runs are two module instances, so
 * `setParam` changes the URL without reaching the host's listeners. Both listen to `popstate`.
 */
export function goToState(state: string) {
  setParam("state", state);
  window.dispatchEvent(new PopStateEvent("popstate"));
}
