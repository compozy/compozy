import { computerQueryKey, LOCAL_COMPUTER_ID } from "@/lib/computer-scope";

export const providerKeys = {
  all: computerQueryKey(LOCAL_COMPUTER_ID, "providers"),
  lists: () => [...providerKeys.all, "list"] as const,
  detail: (providerId: string) => [...providerKeys.all, "detail", providerId.trim()] as const,
  authProbe: (providerId: string) =>
    [...providerKeys.all, "auth-probe", providerId.trim()] as const,
};
