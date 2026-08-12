import { useDaemonStatus } from "@/systems/status";

export function useUserHomeDir(options: { enabled?: boolean } = {}): string | undefined {
  const { data } = useDaemonStatus(options);
  return data?.user_home_dir ?? undefined;
}
