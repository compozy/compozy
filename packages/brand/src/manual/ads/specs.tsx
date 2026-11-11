import { card, lab } from "../cls";
import { Section } from "../parts/section";
import { specRows, utm, utmExample } from "./data";

const th = "px-5 py-3 font-medium";

function SpecTable() {
  return (
    <div className="overflow-x-auto rounded-card border border-(--b-line) bg-(--b-card)">
      <table className="w-full min-w-[880px] border-collapse text-left text-[14px]">
        <thead>
          <tr className="border-b border-(--b-line) mono-label text-[11.5px] text-(--b-muted)">
            <th className={th}>Channel</th>
            <th className={th}>Image sizes</th>
            <th className={th}>Characters</th>
            <th className={th}>File</th>
            <th className={th}>Note</th>
          </tr>
        </thead>
        <tbody>
          {specRows.map(r => (
            <tr key={r.channel} className="border-b border-(--b-line) align-top last:border-0">
              <td className="px-5 py-3.5 font-semibold">{r.channel}</td>
              <td className="px-5 py-3.5 font-mono text-[12.5px]">{r.sizes}</td>
              <td className="px-5 py-3.5 font-mono text-[12.5px]">{r.text}</td>
              <td className="px-5 py-3.5 font-mono text-[12.5px] whitespace-nowrap text-(--b-muted)">
                {r.limits}
              </td>
              <td className="px-5 py-3.5 text-(--b-muted)">{r.note}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function Specs() {
  return (
    <Section
      id="specs"
      n="05"
      file="specs.md"
      title={
        <>
          Sizes, limits and <em>links</em>
        </>
      }
      bg="paper"
      status="proposal"
      intro={
        <p>
          Character limits and sizes per channel, checked in November 2026. Platforms change these
          without notice, so confirm in the ad manager before upload and update this table when they
          move.
        </p>
      }
    >
      <SpecTable />
      <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)]">
        <div className={card}>
          <p className={lab}>UTM convention</p>
          <dl className="m-0 mt-4 grid gap-0">
            {utm.map(([p, r, e]) => (
              <div
                key={p}
                className="grid gap-1 border-b border-(--b-line) py-2.5 last:border-0 md:grid-cols-[130px_minmax(0,1fr)_minmax(0,1.2fr)] md:gap-4"
              >
                <dt className="font-mono text-[13px] text-(--b-accent)">{p}</dt>
                <dd className="m-0 text-[14px]">{r}</dd>
                <dd className="m-0 font-mono text-[12.5px] text-(--b-muted)">{e}</dd>
              </div>
            ))}
          </dl>
        </div>
        <div className={card}>
          <p className={lab}>Example</p>
          <p className="m-0 mt-4 rounded-[12px] bg-rail p-4 font-mono text-[12.5px] leading-[1.7] break-all text-fg">
            {utmExample}
          </p>
          <ul className="m-0 mt-4 list-disc pl-[18px] text-[14px] text-(--b-muted) [&_li]:mb-1.5">
            <li>
              Lowercase and kebab-case everywhere; analytics treats “X” and “x” as two sources.
            </li>
            <li>utm_content is always the template id, so results map back to this page.</li>
            <li>Land on the page that shows the same view as the ad, not the home page.</li>
          </ul>
        </div>
      </div>
    </Section>
  );
}
