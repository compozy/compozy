// Shared SVG piece for the token diagrams: the brand live gradient (volt → cyan at 45°, the
// --gradient-live token) as an SVG paint server.

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
