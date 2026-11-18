import { HI_TIER, visuals, VISUALS_BASE } from "../art/visuals";
import { card, h3, lab, rules } from "../cls";
import { Section } from "../parts/section";

const atmosphere = visuals.filter(v => v.family === "atmosphere");
const link = "text-(--b-accent)";

/** One download link; print (hi tier) links appear only once the file is published (ADR-003). */
function FileLink({ file, hi, label }: { file: string; hi?: boolean; label: string }) {
  if (hi && !HI_TIER.has(file)) return null;
  return (
    <a className={link} href={`${VISUALS_BASE}/${hi ? "hi/" : ""}${file}.jpg`} download>
      {label}
    </a>
  );
}

export function Library() {
  return (
    <Section
      id="library"
      n="09"
      file="brand/visuals/"
      title={
        <>
          Files, components and how the set <em>grows</em>
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          Atmosphere ships as JPG files; Live ships as code. Both are versioned with the manual. Use
          the files as they are: crop and position, nothing else.
        </p>
      }
    >
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)]">
        <div className={card}>
          <p className={lab}>Atmosphere files</p>
          <ul className="m-0 mt-4 grid list-none gap-0 p-0">
            {atmosphere.map(v => (
              <li
                key={v.name}
                className="flex min-w-0 flex-wrap items-center justify-between gap-x-4 gap-y-1 border-b border-(--b-line) py-2.5 font-mono text-[13px]"
              >
                <span>{v.name}</span>
                <span className="flex flex-wrap gap-x-4 gap-y-1">
                  <FileLink file={v.name} label="web ↓" />
                  <FileLink file={v.name} hi label="print 6688 ↓" />
                  {v.portrait && (
                    <>
                      <FileLink file={`${v.name}-portrait`} label="portrait web ↓" />
                      <FileLink file={`${v.name}-portrait`} hi label="portrait print ↓" />
                    </>
                  )}
                </span>
              </li>
            ))}
          </ul>
        </div>
        <div className="grid gap-4">
          <div className={card}>
            <p className={h3}>Live components</p>
            <p className="m-0 text-[15px] text-(--b-muted)">
              <code>src/manual/art/visual.tsx</code> and <code>art/*</code>. They render in any box,
              sized in container units, and export at native size from the template kit.
            </p>
          </div>
          <div className={card}>
            <p className={h3}>Adding to the set</p>
            <ul className={`${rules} text-[15px]`}>
              <li>
                New Atmosphere images are generated from the prompts in{" "}
                <code>prompts/graphics/</code> with an approved image as style reference, reviewed
                next to the set, and approved by the founder before use. Approved images go through
                the Magnific Space “Compozy · Visual system” (Precision sublime, 4×) before they
                enter <code>brand/visuals/hi/</code>.
              </li>
              <li>
                A new Live visual needs a real product surface behind it. If the product does not
                show it, the brand does not draw it.
              </li>
              <li>Retire images instead of editing them; a retired name is never reused.</li>
            </ul>
          </div>
        </div>
      </div>
    </Section>
  );
}
