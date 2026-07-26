import { useEffect } from "react";

import { useDaemonStatus } from "@/systems/status";

import { useUserHomeDirStore } from "./use-user-home-dir-store";

export function useSyncUserHomeDir() {
  const { data } = useDaemonStatus();
  const setUserHomeDir = useUserHomeDirStore(state => state.setUserHomeDir);

  useEffect(() => {
    setUserHomeDir(data?.user_home_dir ?? null);
  }, [data?.user_home_dir, setUserHomeDir]);
}
