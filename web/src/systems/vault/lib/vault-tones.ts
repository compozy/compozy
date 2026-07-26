export function vaultNamespaceTone(namespace: string): "info" | "neutral" {
  return namespace === "sessions" ? "info" : "neutral";
}
