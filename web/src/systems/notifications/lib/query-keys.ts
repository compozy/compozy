export const notificationKeys = {
  all: ["notifications"] as const,
  attentionRoot: () => [...notificationKeys.all, "attention"] as const,
  attention: (profile: string) => [...notificationKeys.attentionRoot(), profile] as const,
};
