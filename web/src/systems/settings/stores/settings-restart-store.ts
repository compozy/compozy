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
  /** Epoch ms when this tab started (or, after an upgrade, rehydrated) the restart. */
  operationStartedAt: number | null;
  lastMutation: PendingSettingsMutation | null;
  mutationGeneration: number;
  snoozedMutationGeneration: number | null;
  snoozedApplyRecordId: string | null;
}

const initialSettingsRestartState: SettingsRestartState = {
  operationId: null,
  operationStartedAt: null,
  lastMutation: null,
  mutationGeneration: 0,
  snoozedMutationGeneration: null,
  snoozedApplyRecordId: null,
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
      snoozedApplyRecordId: null,
    }),
    restartOperationStarted: (context, event: { operationId: string; startedAt?: number }) => ({
      ...context,
      operationId: event.operationId,
      operationStartedAt: event.startedAt ?? null,
    }),
    restartOperationCleared: context => ({
      ...context,
      operationId: null,
      operationStartedAt: null,
    }),
    restartStateReset: () => initialSettingsRestartState,
    restartNoticeDismissed: (context, event: { applyRecordId?: string }) => ({
      ...context,
      operationId: null,
      operationStartedAt: null,
      snoozedMutationGeneration: context.lastMutation?.restartRequired
        ? context.mutationGeneration
        : null,
      snoozedApplyRecordId: event.applyRecordId ?? null,
    }),
  },
}).with(
  persist({
    name: settingsRestartStorageKey,
    storage: settingsRestartStorage,
    version: 2,
    // v0 lacks snoozedApplyRecordId; v0–v1 lack operationStartedAt. A restart already in flight
    // counts from rehydration, so its prolonged-outage guidance can still appear.
    migrate: (
      persisted: Omit<SettingsRestartState, "snoozedApplyRecordId" | "operationStartedAt"> &
        Partial<Pick<SettingsRestartState, "snoozedApplyRecordId" | "operationStartedAt">>
    ) => ({
      ...persisted,
      snoozedApplyRecordId: persisted.snoozedApplyRecordId ?? null,
      operationStartedAt:
        persisted.operationStartedAt ?? (persisted.operationId ? Date.now() : null),
    }),
  })
);
