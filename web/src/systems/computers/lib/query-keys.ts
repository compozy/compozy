export const computerRegistryKeys = {
  all: ["computers"] as const,
  list: () => [...computerRegistryKeys.all, "list"] as const,
  detail: (id: string) => [...computerRegistryKeys.all, "detail", id] as const,
};
