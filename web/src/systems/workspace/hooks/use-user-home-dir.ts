import { useRuntimeIdentity } from "@/systems/status";

export function useUserHomeDir(options: { enabled?: boolean } = {}): string | undefined {
  const { data } = useRuntimeIdentity(options);
  return data?.daemon.user_home_dir ?? undefined;
}
