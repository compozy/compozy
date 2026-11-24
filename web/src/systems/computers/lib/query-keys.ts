export const computerRegistryKeys = {
  all: ["computers"] as const,
  list: () => [...computerRegistryKeys.all, "list"] as const,
  detail: (id: string) => [...computerRegistryKeys.all, "detail", id] as const,
  available: () => [...computerRegistryKeys.all, "available"] as const,
  sshHosts: () => [...computerRegistryKeys.all, "ssh-hosts"] as const,
  remoteStatus: () => [...computerRegistryKeys.all, "remote", "status"] as const,
  remoteDevices: () => [...computerRegistryKeys.all, "remote", "devices"] as const,
};
