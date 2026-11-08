// 02 Product UI as image: how our interface is shown in brand pieces. Replaces the old terminal
// section. Screens are captures of the real product (<Visual>, ADR-005); faces are the live
// @compozy/ui AgentAvatar.
import { cn } from "@compozy/ui/utils";

import { AgentFace } from "../art/pieces";
import { cap, card, h3, lab, rules, specs } from "../cls";
import { DoDont } from "../parts/do-dont";
import { Section } from "../parts/section";
import type { AgentState } from "../art/roster";
import { offBrand, uiRules, uiSpec } from "./data";
import { UiCrop } from "./ui-crop";

const crops: {
  v: "agents" | "approval" | "board";
  t: string;
  d: string;
  s: number;
  ox: number;
  oy: number;
}[] = [
  {
    v: "agents",
    t: "One window: Always-on agents",
    d: "The point is a team that checks in on its own. Faces, state marks and the last check-in note carry it.",
    s: 1.75,
    ox: 50,
    oy: 42,
  },
  {
    v: "approval",
    t: "One window: Approval",
    d: "The point is control. The agent asks, you allow once, always or deny.",
    s: 2,
    ox: 50,
    oy: 52,
  },
  {
    v: "board",
    t: "One window: Board",
    d: "The point is who has the next step. Needs you, Working, In review, Done.",
    s: 1.05,
    ox: 50,
    oy: 42,
  },
];

const states: [AgentState, string, string][] = [
  ["working", "Working", "checkout-lead"],
  ["needs-you", "Needs you", "docs-steward"],
  ["idle", "Idle", "qa-runner"],
  ["paused", "Paused", "platform-engineer"],
  ["failed", "Failed", "test-writer"],
];

const muted = "m-0 mt-4 text-[15px] text-(--b-muted)";

function FacesAtMinimum() {
  return (
    <div className={card}>
      <p className={lab}>Faces at the minimum size</p>
      <p className="m-0 mt-3 text-[14px] text-(--b-muted)">
        Drawn at 24 px on a 240 px frame: state mark and harness chip still read. Smaller than this,
        drop the face and keep the dot.
      </p>
      <div className="mt-5 rounded-[14px] bg-rail p-5">
        <div className="flex items-end justify-between gap-2">
          {states.map(([s, w, who]) => (
            <div key={s} className="grid justify-items-center gap-3">
              <AgentFace name={who} state={s} size={40} />
              <span className="font-mono text-[11px] text-muted-foreground">{w}</span>
            </div>
          ))}
        </div>
      </div>
      <p className={cap}>
        Five states, five faces, each with its state mark. Faces are for agents only.
      </p>
    </div>
  );
}

function TiltDoDont() {
  return (
    <DoDont
      className="mt-4"
      doLabel="Do · built-in angle, one visual"
      dontLabel="Don't · a steep table nobody can read"
      doContent={
        <>
          <UiCrop name="agents" ratio="16/9" scale={1.35} flat={false} oy={46} />
          <p className={muted}>
            Tilt gives depth to a hero. Text inside stays large enough to read at export.
          </p>
        </>
      }
      dontContent={
        <>
          <div
            className="relative overflow-hidden rounded-[14px] border border-(--b-line) bg-rail"
            style={{ aspectRatio: "16/9" }}
          >
            <div
              className="absolute inset-0 flex items-center justify-center"
              style={{ perspective: "500px" }}
            >
              <div
                className="w-[88%] rounded-[10px] border border-line-strong bg-elevated p-3 font-mono text-[7px] leading-[1.5] text-muted-foreground"
                style={{ transform: "rotateX(38deg) rotateY(-34deg) rotateZ(10deg)" }}
              >
                {Array.from({ length: 11 }, (_, i) => (
                  <p key={i} className="m-0 flex justify-between">
                    <span>session_{i + 1}</span>
                    <span>state=working agent=claude pid={4120 + i}</span>
                  </p>
                ))}
              </div>
            </div>
          </div>
          <p className={muted}>
            Steep angles, 7 px type and no faces: unreadable at export and not our UI.
          </p>
        </>
      }
    />
  );
}

function FakeFeaturesDoDont() {
  return (
    <DoDont
      className="mt-4"
      doLabel="Do · what the product does"
      dontLabel="Don't · a terminal as the product"
      doContent={
        <>
          <UiCrop name="approval" ratio="16/9" scale={1.6} oy={53} />
          <p className={muted}>
            Allow once, Always allow, Deny: the choices the permission dock offers.
          </p>
        </>
      }
      dontContent={
        <>
          <div
            className="grid aspect-video place-items-center overflow-hidden rounded-[14px] border border-(--b-line) bg-black p-4 font-mono text-[11px] leading-[1.4]"
            style={{ color: offBrand.matrix }}
          >
            <div className="w-full">
              <span style={{ color: offBrand.trafficRed }}>●</span>{" "}
              <span style={{ color: offBrand.trafficAmber }}>●</span>{" "}
              <span style={{ color: offBrand.trafficGreen }}>●</span>
              <br />
              &gt;&gt;&gt; AI AGENT ONLINE
              <br />
              &gt;&gt;&gt; hacking mainframe... 100%
              <br />
              &gt;&gt;&gt; npm run magic --turbo
            </div>
          </div>
          <p className={muted}>
            A terminal as the image, matrix green, invented commands. It says nothing about the
            control plane.
          </p>
        </>
      }
    />
  );
}

export function ProductUi() {
  return (
    <Section
      id="ui"
      n="02"
      file="product-ui.md"
      title={
        <>
          Show the <em>product</em>, not the prompt
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          Compozy is a control plane: you see every agent and every harness in one desktop, and the
          product is how well that desktop is designed. So brand pieces show the interface, redrawn
          in the brand, never a terminal.
        </p>
      }
    >
      <figure className="m-0">
        <UiCrop name="desktop" ratio="16/9" />
        <figcaption className={cap}>
          The whole desktop: menubar, app rail, projects with Always-on faces, and the session with
          its handoff messages. The hero crop for a launch, a release or a site header.
        </figcaption>
      </figure>

      <h3 className={cn(h3, "mt-16")}>One window, one point</h3>
      <div className="mt-4 grid gap-4 md:grid-cols-2">
        {crops.map(c => (
          <figure key={c.v} className={cn("m-0 min-w-0", c.v === "board" && "md:col-span-2")}>
            <UiCrop
              name={c.v}
              ratio={c.v === "board" ? "21/9" : "4/3"}
              scale={c.s}
              ox={c.ox}
              oy={c.oy}
            />
            <figcaption className={cap}>
              <b className="font-medium text-(--b-fg)">{c.t}.</b> {c.d}
            </figcaption>
          </figure>
        ))}
      </div>

      <div className="mt-16 grid gap-4 lg:grid-cols-[minmax(0,1.35fr)_minmax(0,1fr)]">
        <div className={card}>
          <p className={lab}>Spec</p>
          <dl className={cn(specs, "mt-3")}>
            {uiSpec.map(([k, v]) => (
              <div key={k}>
                <dt>{k}</dt>
                <dd>{v}</dd>
              </div>
            ))}
          </dl>
        </div>
        <FacesAtMinimum />
      </div>

      <ul className={cn(rules, "mt-10 max-w-[76ch] text-[15.5px]")}>
        {uiRules.map(r => (
          <li key={r}>{r}</li>
        ))}
      </ul>

      <h3 className={cn(h3, "mt-16")}>Tilt, with limits</h3>
      <TiltDoDont />

      <h3 className={cn(h3, "mt-16")}>Fake features</h3>
      <FakeFeaturesDoDont />
    </Section>
  );
}
