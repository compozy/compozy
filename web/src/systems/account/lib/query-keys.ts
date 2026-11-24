export const accountKeys = {
  all: ["account"] as const,
  status: () => [...accountKeys.all, "status"] as const,
};
