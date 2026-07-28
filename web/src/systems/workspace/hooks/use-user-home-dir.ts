import { useUserHomeDirStore } from "./use-user-home-dir-store";

export function useUserHomeDir(): string | undefined {
  const userHomeDir = useUserHomeDirStore(state => state.userHomeDir);
  return userHomeDir ?? undefined;
}
