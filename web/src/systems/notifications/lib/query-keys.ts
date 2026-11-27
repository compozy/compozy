import { computerQueryKey, LOCAL_COMPUTER_ID } from "@/lib/computer-scope";

export const notificationKeys = {
  all: computerQueryKey(LOCAL_COMPUTER_ID, "notifications"),
  attentionRoot: () => [...notificationKeys.all, "attention"] as const,
  attention: (profile: string) => [...notificationKeys.attentionRoot(), profile] as const,
};
