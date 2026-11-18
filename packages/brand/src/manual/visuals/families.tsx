import { card, h3, lab, rules, specs } from "../cls";
import { Visual } from "../art/visual";
import { Section } from "../parts/section";
import { familyUse } from "./data";

const bleed =
  "relative -mx-[22px] -mt-[22px] mb-6 overflow-hidden rounded-t-card sm:-mx-7 sm:-mt-7";

export function Families() {
  return (
    <Section
      id="families"
      n="02"
      file="families.md"
      title={
        <>
          Two families, one <em>ground</em>
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          Pick the family by what the piece is about. If it is about the product, a release, a
          feature or a number, use Live: our UI is the strongest image we have. If it is about a
          mood or an invitation, use Atmosphere. One piece, one family; when in doubt, show the
          product.
        </p>
      }
    >
      <div className="grid gap-4 lg:grid-cols-2">
        <div className={card}>
          <div className={bleed}>
            <Visual name="swarm" inline ratio="16/9" />
          </div>
          <p className={lab}>Atmosphere · 7 visuals</p>
          <p className={`${h3} mt-2`}>Mood: particles, light, fine lines</p>
          <ul className={`${rules} mt-3 text-[15px]`}>
            <li>
              Generated rasters, approved one by one. Never re-generated per piece; the set grows
              only by review.
            </li>
            <li>
              Dark only. Each ships landscape (16:9) and, for five of them, a portrait crop (4:5)
              picked automatically.
            </li>
            <li>
              Use for covers, brand ads, banners, posters, wallpapers, slide titles, stickers.
            </li>
          </ul>
        </div>
        <div className={card}>
          <div className={bleed}>
            <Visual name="desktop" inline ratio="16/9" />
          </div>
          <p className={lab}>Live · 9 visuals</p>
          <p className={`${h3} mt-2`}>Evidence: our UI, as it ships</p>
          <ul className={`${rules} mt-3 text-[15px]`}>
            <li>
              The Compozy desktop captured from the real product components: always-on agents with
              faces, board, orchestration, approvals. Same capture in the manual, the kit and the
              site.
            </li>
            <li>
              Dark or light (paper). Real harness names, real states, real-looking numbers; samples
              are labelled as samples.
            </li>
            <li>Use for releases, changelog, docs, product posts, proof ads, reports.</li>
          </ul>
        </div>
      </div>
      <div className={`${card} mt-4`}>
        <dl className={specs}>
          {familyUse.map(([dt, dd]) => (
            <div key={dt}>
              <dt>{dt}</dt>
              <dd>{dd}</dd>
            </div>
          ))}
        </dl>
      </div>
    </Section>
  );
}
