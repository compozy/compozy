export const providerKeys = {
  all: ["providers"] as const,
  lists: () => [...providerKeys.all, "list"] as const,
  detail: (providerId: string) => [...providerKeys.all, "detail", providerId.trim()] as const,
  authProbe: (providerId: string) =>
    [...providerKeys.all, "auth-probe", providerId.trim()] as const,
};
