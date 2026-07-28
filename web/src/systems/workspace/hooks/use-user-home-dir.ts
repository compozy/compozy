import { useDaemonStatus } from "@/systems/status";

export function useUserHomeDir(): string | undefined {
  const { data } = useDaemonStatus();
  return data?.user_home_dir ?? undefined;
}
