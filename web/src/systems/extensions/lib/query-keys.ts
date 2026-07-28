export const extensionKeys = {
  all: ["extensions"] as const,
  list: () => [...extensionKeys.all, "list"] as const,
  provenance: (name: string) => [...extensionKeys.all, "provenance", name] as const,
  bundles: () => [...extensionKeys.all, "bundle-activations"] as const,
  bundle: (id: string) => [...extensionKeys.bundles(), id] as const,
};
