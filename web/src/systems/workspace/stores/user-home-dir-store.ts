import type { StateCreator } from "zustand";

export interface UserHomeDirState {
  userHomeDir: string | null;
}

export interface UserHomeDirActions {
  setUserHomeDir: (userHomeDir: string | null) => void;
}

export type UserHomeDirStore = UserHomeDirState & UserHomeDirActions;

export const initialUserHomeDirState: UserHomeDirState = {
  userHomeDir: null,
};

export const createUserHomeDirStore: StateCreator<UserHomeDirStore> = set => ({
  ...initialUserHomeDirState,
  setUserHomeDir: userHomeDir => set({ userHomeDir }),
});
