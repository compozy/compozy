// 11 · 06 Stationery: business card both sides, letterhead, envelope, email signature.
import { card, h3 } from "../cls";
import { Frame } from "../kit-frame";
import { InContext } from "../parts/in-context";
import { Section } from "../parts/section";
import { stationery } from "./data";

export function Stationery() {
  return (
    <Section
      id="stationery"
      n="06"
      file="templates/offline/"
      title={
        <>
          Paper that says <em>who</em>
        </>
      }
      bg="paper"
      status="proposal"
      intro={
        <p>
          Stationery is quiet. The card back is ink with an Atmosphere crop; letters and envelopes
          are paper with the lockup and, at most, a thin Live strip. Names and contact details are
          fields in the kit.
        </p>
      }
    >
      <div className="grid gap-6 md:grid-cols-2">
        <Frame id="business-card-front" guides />
        <Frame id="business-card-back" guides />
      </div>
      <div className="mt-8 grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.5fr)]">
        <Frame id="letterhead" guides />
        <div className="grid content-start gap-6">
          <Frame id="envelope-dl" guides />
          <Frame id="email-signature" />
          <div className={card}>
            <h3 className={h3}>Specs</h3>
            <dl className="m-0 mt-3 grid text-[14px]">
              {stationery.map(([n, s, d]) => (
                <div
                  key={n}
                  className="grid grid-cols-[130px_minmax(0,1fr)] gap-3 border-b border-(--b-line) py-2.5"
                >
                  <dt>
                    <span className="block font-medium">{n}</span>
                    <span className="font-mono text-[12px] text-(--b-muted)">{s}</span>
                  </dt>
                  <dd className="m-0 text-(--b-muted)">{d}</dd>
                </div>
              ))}
            </dl>
            <p className="m-0 mt-4 text-[14px] text-(--b-muted)">
              The email signature ships as HTML with the lockup as a 2× PNG hosted on compozy.com;
              never attach it as a file.
            </p>
          </div>
        </div>
      </div>
      <InContext
        ids={["business-cards"]}
        title="In context · business cards"
        className="max-w-[760px]"
      />
    </Section>
  );
}
