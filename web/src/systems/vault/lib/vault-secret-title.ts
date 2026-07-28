/** Last path segment of a vault ref, used as the sheet title. */
export function vaultSecretTitle(ref: string): string {
  const withoutPrefix = ref.startsWith("vault:") ? ref.slice("vault:".length) : ref;
  const segments = withoutPrefix.split("/").filter(Boolean);
  return segments.at(-1) ?? ref;
}
