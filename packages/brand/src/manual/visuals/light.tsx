import { card, h3, lab, rules } from "../cls";
import { Section } from "../parts/section";
import { lightSwatches } from "./data";

export function Light() {
  return (
    <Section
      id="light"
      n="06"
      file="light.md"
      title={
        <>
          Light means <em>active</em>
        </>
      }
      status="proposal"
      intro={
        <p>
          Colour in the visuals is never decoration. Blue to cyan light always marks what is
          running, live or selected; everything at rest is white, grey or black. That rule is what
          lets a Live visual and an Atmosphere visual sit in the same brand.
        </p>
      }
    >
      <div className="grid gap-3 sm:grid-cols-5">
        {lightSwatches.map(s => (
          <div key={s.name} className="min-w-0">
            <div
              className="h-24 rounded-[12px] border border-(--b-line)"
              style={{ background: s.fill }}
            />
            <p className="m-0 mt-2 text-[15px] font-medium">{s.name}</p>
            <p className="m-0 font-mono text-[12px] text-(--b-muted)">{s.hex}</p>
            <p className="m-0 mt-1 text-[13px] text-(--b-muted)">{s.text}</p>
          </div>
        ))}
      </div>
      <div className="mt-8 grid gap-4 md:grid-cols-2">
        <div className={card}>
          <p className={h3}>Rules</p>
          <ul className={`${rules} text-[15px]`}>
            <li>One lit area per piece. A second bright spot competes with the headline.</li>
            <li>
              Gradients always run at 45° (CSS <code>45deg</code>), bottom-left to top-right. The
              brand gradient for type, the symbol and details is Sky #1F87FC → Teal #1DF0D7 → Glint
              #B5FAF6. Live visuals keep Volt #2F5BFF → Cyan #3DF2E0 for running work. Deep #0B31F7
              is never in a gradient: on black it loses contrast.
            </li>
            <li>
              No purple, magenta, orange or green in any visual. Status colours (success, warning,
              danger) belong to product UI only.
            </li>
            <li>
              Type on visuals is white or #ececec; mono metadata #989898. Never gradient type on top
              of a gradient visual.
            </li>
          </ul>
        </div>
        <div className={card}>
          <p className={lab}>Grain and glow</p>
          <ul className={`${rules} mt-4 text-[15px]`}>
            <li>Grain is part of the rasters; never add more in layout tools.</li>
            <li>
              Glow lives only around the brightest point (a cyan dot, the converged stream). No
              outer glow on type, cards or logos.
            </li>
            <li>In Live, the only glow is the 0.9 em halo of a running state dot.</li>
          </ul>
        </div>
      </div>
    </Section>
  );
}
