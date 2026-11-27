import { computerQueryKey, LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";

export function providerKeysFor(computerId: ComputerId = LOCAL_COMPUTER_ID) {
  const all = computerQueryKey(computerId, "providers");
  return {
    all,
    lists: () => [...all, "list"] as const,
    detail: (providerId: string) => [...all, "detail", providerId.trim()] as const,
    authProbe: (providerId: string) => [...all, "auth-probe", providerId.trim()] as const,
  };
}

export const providerKeys = {
  ...providerKeysFor(LOCAL_COMPUTER_ID),
  forComputer: providerKeysFor,
};
