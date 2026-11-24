import { computerQueryKey, LOCAL_COMPUTER_ID } from "@/lib/computer-scope";

export const supportKeys = {
  all: computerQueryKey(LOCAL_COMPUTER_ID, "support"),
  bundles: () => [...supportKeys.all, "bundle"] as const,
  bundle: (operationId: string) => [...supportKeys.bundles(), operationId] as const,
};
