import { AgentFace } from "../agent-face";
import { card, h3, lab, rules, specs } from "../cls";
import { Section } from "../parts/section";
import { harnessLabel, roster } from "../roster";
import { faceStates, rosterStates } from "./data";

export function Faces() {
  return (
    <Section
      id="faces"
      n="05"
      file="agent-face.tsx"
      title={
        <>
          Every agent has a <em>face</em>
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          Always-on agents are the team that keeps working when you are away. Each one gets a face,
          generated from its name with the same library the product uses (Blobatar), so the same
          agent looks the same in the app, the docs, a slide and a post. The face carries state; the
          corner chip carries the harness.
        </p>
      }
    >
      <div className={`${card} grid gap-8`}>
        <div className="relative flex flex-wrap items-end gap-x-10 gap-y-8">
          {roster.map((a, i) => (
            <div key={a.name} className="grid justify-items-center gap-3 text-center">
              <AgentFace name={a.name} state={rosterStates[i]} size={72} />
              <div>
                <p className="m-0 text-[15px] font-medium">{a.name}</p>
                <p className="m-0 font-mono text-[11.5px] text-(--b-muted)">
                  {harnessLabel[a.harness]}
                </p>
              </div>
            </div>
          ))}
        </div>
      </div>
      <div className="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
        {faceStates.map(([s, t, d]) => (
          <div key={s} className={card}>
            <AgentFace name="checkout-lead" state={s} size={72} />
            <p className={`${lab} mt-5`}>{s}</p>
            <p className="m-0 mt-1 text-[16px] font-medium">{t}</p>
            <p className="m-0 mt-1 text-[14px] text-(--b-muted)">{d}</p>
          </div>
        ))}
      </div>
      <div className="mt-4 grid gap-4 lg:grid-cols-2">
        <div className={card}>
          <p className={h3}>Rules</p>
          <ul className={`${rules} text-[15px]`}>
            <li>
              <strong>Seed is the agent name.</strong> Same name, same face, everywhere. Never
              re-roll a face in marketing.
            </li>
            <li>
              <strong>Agents only.</strong> Faces never stand for people, customers or the company.
              People get photos or initials.
            </li>
            <li>
              <strong>State is honest.</strong> A face shows the state the agent is in on that
              screen. No "happy" agents for decoration.
            </li>
            <li>
              <strong>Colour stays the product's.</strong> Face colours come from the generator;
              brand blue and cyan stay for light and the running dot.
            </li>
            <li>
              <strong>Harness chip from 24 px.</strong> Below that, the face alone; the harness goes
              in the label.
            </li>
          </ul>
        </div>
        <div className={card}>
          <dl className={specs}>
            <div>
              <dt>Library</dt>
              <dd>
                blobatar 2.7 (MIT), <code>AgentAvatar</code> from <code>@compozy/ui</code>
              </dd>
            </div>
            <div>
              <dt>Sizes</dt>
              <dd>16 px list · 24 px row · 32–48 px card · 112 px profile</dd>
            </div>
            <div>
              <dt>State mark</dt>
              <dd>dot 26% of the face, top right; needs-you triangle #F5B14C</dd>
            </div>
            <div>
              <dt>Harness chip</dt>
              <dd>
                40% of the face, bottom right: Claude Code, Codex, Gemini CLI, OpenCode, Cursor
              </dd>
            </div>
            <div>
              <dt>Motion</dt>
              <dd>
                Working faces hover gently (2.4 s); everything else is still. Respect reduced
                motion.
              </dd>
            </div>
          </dl>
        </div>
      </div>
    </Section>
  );
}
