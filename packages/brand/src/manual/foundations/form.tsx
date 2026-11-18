import { card, lab, rules, voltFill } from "../cls";
import { Section } from "../parts/section";

const radii: [px: number, use: string][] = [
  [4, "tick, kbd"],
  [6, "chip, tag"],
  [10, "button, input"],
  [14, "inner panel, image"],
  [20, "card"],
  [28, "hero, cover"],
  [999, "pill, status"],
];

export function Form() {
  return (
    <Section
      id="form"
      n="08"
      file="form.css"
      title={
        <>
          Soft corners, <em>hard</em> lines
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          Radii follow the size of the object: the bigger the surface, the softer the corner. Lines
          are always 1 px and always quiet.
        </p>
      }
    >
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4 lg:grid-cols-7">
        {radii.map(([r, u]) => (
          <div key={r} className="text-center">
            <div
              className="mx-auto aspect-square w-full border border-line-strong bg-(--b-card)"
              style={{ borderRadius: `${Math.min(r, 60)}px` }}
            />
            <p className="m-0 mt-2.5 font-mono text-[12.5px]">{r} px</p>
            <p className="m-0 text-[12.5px] text-(--b-muted)">{u}</p>
          </div>
        ))}
      </div>
      <div className="mt-10 grid gap-4 md:grid-cols-2">
        <div className={card}>
          <p className={lab}>Borders and depth</p>
          <ul className={`${rules} mt-4 text-[15px]`}>
            <li>
              1 px <code>--color-line</code> separates; 1 px <code>--color-line-strong</code>{" "}
              outlines interactive things.
            </li>
            <li>Depth comes from the ink ladder (ink → ink 2 → ink 3), not from shadows.</li>
            <li>
              One shadow exists: overlays and menus, <code>0 24px 60px rgba(0,0,0,.5)</code>.
            </li>
            <li>No glow, no neon, no inner shine. Light belongs to the gradient only.</li>
          </ul>
        </div>
        <div className={card}>
          <p className={lab}>Nested radius</p>
          <div className="mt-4 rounded-[20px] border border-line-strong p-3">
            <div className="rounded-[14px] border border-line-strong p-3">
              <div
                className={`rounded-[10px] px-4 py-2.5 text-center text-[14px] font-medium ${voltFill}`}
              >
                Install the beta
              </div>
            </div>
          </div>
          <p className="m-0 mt-4 text-[14px] text-(--b-muted)">
            Inner radius = outer radius − padding. 20 → 14 → 10.
          </p>
        </div>
      </div>
    </Section>
  );
}
