function vaultRefSegments(ref: string): string[] {
  const withoutPrefix = ref.startsWith("vault:") ? ref.slice("vault:".length) : ref;
  return withoutPrefix.split("/").filter(Boolean);
}

/** Last path segment of a vault ref, used as the secret's friendly title. */
export function vaultSecretTitle(ref: string): string {
  return vaultRefSegments(ref).at(-1) ?? ref;
}

/** Path above the title (`providers/openai`), which tells same-titled secrets apart. */
export function vaultSecretLocation(ref: string): string {
  return vaultRefSegments(ref).slice(0, -1).join("/");
}
