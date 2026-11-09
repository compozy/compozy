// Deterministic seeded random for the token diagrams, so the prerendered diagram and its hydrated
// twin draw the same bars.

/** Park–Miller minimal standard generator: same seed, same sequence, on server and client. */
export function seeded(seed: number) {
  let s = seed;
  return () => (s = (s * 16807) % 2147483647) / 2147483647;
}
