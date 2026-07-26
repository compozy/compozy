export const statusKeys = {
  all: ["status"] as const,
  current: () => [...statusKeys.all, "current"] as const,
};
