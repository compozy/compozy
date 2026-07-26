export const supportKeys = {
  all: ["support"] as const,
  bundles: () => [...supportKeys.all, "bundle"] as const,
  bundle: (operationId: string) => [...supportKeys.bundles(), operationId] as const,
};
