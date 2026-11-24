export const accountKeys = {
  all: ["account"] as const,
  status: () => [...accountKeys.all, "status"] as const,
  devices: () => [...accountKeys.all, "devices"] as const,
  approvals: () => [...accountKeys.all, "approvals"] as const,
  tokens: () => [...accountKeys.all, "tokens"] as const,
};
