import { card, lab } from "../cls";
import { Section } from "../parts/section";

const refs: [group: string, items: [title: string, href: string][]][] = [
  [
    "Process",
    [
      ["Rebrand rounds r1–r13 and the brand board", "~/Dev/compozy/_graphic/rebrand-2026"],
      ["Decision log", "second-brain/compozy/rebrand-direcao.md"],
      ["Research for this manual", "_research/ (positioning, current surfaces, Reload blueprint)"],
    ],
  ],
  [
    "Product copy",
    [
      ["COPY.md — runtime and public copy rules", "compozy/COPY.md"],
      ["PRODUCT.md and DESIGN.md", "compozy/"],
      ["Glossary", "compozy/docs/_memory/glossary.md"],
    ],
  ],
  [
    "Type",
    [
      ["Instrument Sans", "https://fonts.google.com/specimen/Instrument+Sans"],
      ["Figtree", "https://fonts.google.com/specimen/Figtree"],
      ["Geist Mono", "https://vercel.com/font"],
    ],
  ],
  [
    "Method",
    [
      ["Reload brand manual (structure this manual follows)", "https://reload.dev/branding"],
      ["WCAG 2.2 contrast", "https://www.w3.org/TR/WCAG22/#contrast-minimum"],
      ["Savee boards used in exploration", "https://savee.com"],
    ],
  ],
];

export function References() {
  return (
    <Section
      id="references"
      n="05"
      file="references.md"
      title={
        <>
          Where this <em>came from</em>
        </>
      }
      status="approved"
      intro={
        <p>
          The decisions behind the identity, the sources the copy must agree with, and the
          references used along the way.
        </p>
      }
    >
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        {refs.map(([g, items]) => (
          <div key={g} className={card}>
            <p className={lab}>{g}</p>
            <ul className="m-0 mt-4 grid list-none gap-3 p-0 text-[14.5px]">
              {items.map(([t, h]) => (
                <li key={t}>
                  {h.startsWith("http") ? (
                    <a
                      href={h}
                      className="text-(--b-fg) underline decoration-(--b-line) underline-offset-4 hover:decoration-(--b-accent)"
                    >
                      {t} ↗
                    </a>
                  ) : (
                    <span>{t}</span>
                  )}
                  <span className="mt-0.5 block font-mono text-[11.5px] break-all text-(--b-muted)">
                    {h}
                  </span>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
    </Section>
  );
}
