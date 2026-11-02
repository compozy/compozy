export const statusKeys = {
  all: ["status"] as const,
  identity: () => [...statusKeys.all, "identity"] as const,
  current: () => [...statusKeys.all, "current"] as const,
};
