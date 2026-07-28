import { createStore } from "@xstate/store";
import { createJSONStorage, persist } from "@xstate/store/persist";

import type {
  ConfigApplyLifecycle,
  SettingsApplyNextAction,
  SettingsMutationResult,
} from "../types";

export interface PendingSettingsMutation {
  section: SettingsMutationResult["section"];
  restartRequired: boolean;
  restartScope?: string;
  warnings: string[];
  lifecycle?: ConfigApplyLifecycle;
  nextAction?: SettingsApplyNextAction;
  applyRecordId?: string;
  activeGeneration?: number;
  completedAt: string;
}

export interface SettingsRestartState {
  operationId: string | null;
  lastMutation: PendingSettingsMutation | null;
  mutationGeneration: number;
  snoozedMutationGeneration: number | null;
}

const initialSettingsRestartState: SettingsRestartState = {
  operationId: null,
  lastMutation: null,
  mutationGeneration: 0,
  snoozedMutationGeneration: null,
};

export const settingsRestartStorageKey = "compozy:settings:restart:v3";

const settingsRestartStorage = createJSONStorage(() => sessionStorage);

export const settingsRestartStore = createStore({
  context: initialSettingsRestartState,
  on: {
    settingsMutationRecorded: (context, event: { mutation: PendingSettingsMutation | null }) => ({
      ...context,
      lastMutation: event.mutation,
      mutationGeneration: context.mutationGeneration + 1,
      snoozedMutationGeneration: null,
    }),
    restartOperationStarted: (context, event: { operationId: string }) => ({
      ...context,
      operationId: event.operationId,
    }),
    restartOperationCleared: context => ({
      ...context,
      operationId: null,
    }),
    restartStateReset: () => initialSettingsRestartState,
    restartNoticeDismissed: context => ({
      ...context,
      operationId: null,
      snoozedMutationGeneration: context.lastMutation?.restartRequired
        ? context.mutationGeneration
        : null,
    }),
  },
}).with(
  persist({
    name: settingsRestartStorageKey,
    storage: settingsRestartStorage,
  })
);
