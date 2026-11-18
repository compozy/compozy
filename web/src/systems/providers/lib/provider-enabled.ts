/**
 * Whether a provider is turned on. Every inventory payload carries `enabled`; a missing field (an
 * older daemon) reads as on, so nothing disappears from a picker by accident.
 */
export function isProviderEnabled(item: { enabled?: boolean } | null | undefined): boolean {
  return item?.enabled !== false;
}
