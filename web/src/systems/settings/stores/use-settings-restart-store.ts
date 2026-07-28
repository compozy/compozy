import { useSelector } from "@xstate/store-react";
import { clearStorage } from "@xstate/store/persist";

import { settingsRestartStore, type SettingsRestartState } from "./settings-restart-store";

export function useSettingsRestartState<T>(
  selector: (state: SettingsRestartState) => T,
  compare?: (previous: T | undefined, next: T) => boolean
): T {
  return useSelector(settingsRestartStore, snapshot => selector(snapshot.context), compare);
}

export function resetSettingsRestartStore() {
  settingsRestartStore.trigger.restartStateReset();
  return clearStorage(settingsRestartStore);
}
