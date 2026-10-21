export const providerUsageKeys = {
  all: ["provider-usage"] as const,
  envelope: (profile: string) => [...providerUsageKeys.all, "envelope", profile] as const,
};
