import { computerQueryKey, LOCAL_COMPUTER_ID } from "@/lib/computer-scope";

export const statusKeys = {
  all: computerQueryKey(LOCAL_COMPUTER_ID, "status"),
  identity: () => [...statusKeys.all, "identity"] as const,
  current: () => [...statusKeys.all, "current"] as const,
};
