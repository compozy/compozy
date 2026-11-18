import { Visual } from "../art/visual";
import { visuals } from "../art/visuals";
import { card, lab, specs } from "../cls";
import { Section } from "../parts/section";
import { liveSource } from "./data";

// Live = product shots captured from web/ plus the token diagrams (ADR-005).
const live = visuals.filter(v => v.family !== "atmosphere");
const PAPER_INDEX = 3;

export function Live() {
  return (
    <Section
      id="live"
      n="04"
      file="art/"
      title={
        <>
          Live: our UI is the <em>image</em>
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          The product is the best picture of the brand. Live visuals show the Compozy desktop as it
          ships: always-on agents with their faces, the board, a session handing work to other
          harnesses, approvals. Screens are captured from the real product components in the current
          theme, so a UI improvement reaches the brand by recapturing; small pieces (faces, status,
          the approval row) render live. Same source in the manual, the kit and the site:{" "}
          <code>src/manual/art/</code>.
        </p>
      }
    >
      <div className="grid gap-6">
        {live.map((v, i) => (
          <div
            key={v.name}
            className="grid items-start gap-4 lg:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]"
          >
            <div className="relative overflow-hidden rounded-card border border-(--b-line)">
              <Visual
                name={v.name}
                inline
                ratio="16/9"
                theme={i === PAPER_INDEX ? "light" : "dark"}
              />
            </div>
            <div className={card}>
              <p className={lab}>
                {v.name}
                {i === PAPER_INDEX ? " · shown on paper" : ""}
              </p>
              <p className="m-0 mt-2 font-display text-[24px] tracking-[-0.015em] text-(--b-fg)">
                {v.title}
              </p>
              <p className="m-0 mt-2 text-[15px] text-(--b-muted)">{v.idea}</p>
              <dl className={`${specs} mt-4`}>
                <div>
                  <dt>Source</dt>
                  <dd>{liveSource[v.family]}</dd>
                </div>
                <div>
                  <dt>Themes</dt>
                  <dd>dark · light</dd>
                </div>
              </dl>
            </div>
          </div>
        ))}
      </div>
      <p className="m-0 mt-8 font-mono text-[12.5px] text-(--b-muted)">
        {'<Visual name="sessions" /> · <Visual name="lanes" theme="light" />'}
      </p>
    </Section>
  );
}
