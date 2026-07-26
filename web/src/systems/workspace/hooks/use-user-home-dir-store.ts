import { create } from "zustand";

import {
  createUserHomeDirStore,
  initialUserHomeDirState,
  type UserHomeDirStore,
} from "../stores/user-home-dir-store";

export const useUserHomeDirStore = create<UserHomeDirStore>()(createUserHomeDirStore);

export function resetUserHomeDirStore() {
  useUserHomeDirStore.setState(initialUserHomeDirState);
}
