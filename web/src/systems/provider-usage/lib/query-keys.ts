import { computerQueryKey, LOCAL_COMPUTER_ID } from "@/lib/computer-scope";

export const providerUsageKeys = {
  all: computerQueryKey(LOCAL_COMPUTER_ID, "provider-usage"),
  envelope: (profile: string) => [...providerUsageKeys.all, "envelope", profile] as const,
  refresh: (profile: string) => [...providerUsageKeys.all, "refresh", profile] as const,
};
