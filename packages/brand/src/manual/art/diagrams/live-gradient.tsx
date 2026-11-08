// Shared SVG pieces for the token diagrams: the brand live gradient (volt → cyan at 45°, the
// --gradient-live token) as an SVG paint server, and a deterministic seeded random so the
// prerendered diagram and its hydrated twin draw the same bars.

/** SVG twin of --gradient-live; stops read the @compozy/ui brand tokens by name. */
export function LiveGradient({ id }: { id: string }) {
  return (
    <defs>
      <linearGradient id={id} x1="0" y1="1" x2="1" y2="0">
        <stop className="[stop-color:var(--color-brand-volt)]" />
        <stop offset="1" className="[stop-color:var(--color-brand-cyan)]" />
      </linearGradient>
    </defs>
  );
}

/** Park–Miller minimal standard generator: same seed, same sequence, on server and client. */
export function seeded(seed: number) {
  let s = seed;
  return () => (s = (s * 16807) % 2147483647) / 2147483647;
}
