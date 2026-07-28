import { useSelector } from "@xstate/store-react";
import { clearStorage } from "@xstate/store/persist";

import { settingsRestartStore, type SettingsRestartState } from "./settings-restart-store";

export function useSettingsRestartState<T>(selector: (state: SettingsRestartState) => T): T {
  return useSelector(settingsRestartStore, snapshot => selector(snapshot.context));
}

export function resetSettingsRestartStore() {
  settingsRestartStore.trigger.restartStateReset();
  return clearStorage(settingsRestartStore);
}
