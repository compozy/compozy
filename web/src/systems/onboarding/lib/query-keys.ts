import { computerQueryKey, LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";
import type { DirectoryBrowseQuery } from "../types";

export function onboardingKeysFor(computerId: ComputerId = LOCAL_COMPUTER_ID) {
  const all = computerQueryKey(computerId, "onboarding");
  return {
    all,
    status: () => [...all, "status"] as const,
    browse: (query: DirectoryBrowseQuery) => [...all, "browse", query] as const,
  };
}

export const onboardingKeys = {
  ...onboardingKeysFor(LOCAL_COMPUTER_ID),
  forComputer: onboardingKeysFor,
};
