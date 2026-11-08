// Content for chapter 09, Motion. Numbers are the v1.2.0 brand/motion.json values; prose follows
// the voice rules. Values without a @compozy/ui token (sweep, handoff, loop, ease loop) live here only.
import type { VisualName } from "../art/visuals";

export const motion = {
  version: "1.1.0",
  durations: { press: 120, hover: 180, enter: 260, handoff: 900, loop: 2400, sweep: 640 },
  eases: {
    out: [0.22, 1, 0.36, 1],
    inOut: [0.65, 0, 0.35, 1],
    loop: [0.45, 0, 0.2, 1],
  },
  stagger: { reveal: 80, list: 40 },
  video: { fps: 30, sting: 1.6, endCard: 2.4, safeHold: 0.6 },
  visuals: {
    drift: {
      duration_s: 24,
      distance: "1.5% of the box",
      scale: [1.02, 1.05],
      note: "Atmosphere visuals drift slowly, alternate, never loop visibly",
    },
    sweep: {
      duration_ms: 640,
      note: "light band crossing lines (horizon), or a Live bar growing to the now line",
    },
  },
};

export type EaseName = keyof typeof motion.eases;

export const bez = (e: readonly number[]) => `cubic-bezier(${e.join(", ")})`;

export const principles: [string, string, string][] = [
  [
    "01",
    "Motion shows work",
    "Every move says that something is running, handing off or done: a bar grows to the now line, a status changes, a log line arrives. If a move says none of that, it does not ship.",
  ],
  [
    "02",
    "Atmosphere is slow, Live is exact",
    "Atmosphere visuals (swarm, converge, horizon) drift 1.5% of their box over 24 s. Live visuals change in 260–900 ms and always land on a readable state.",
  ],
  [
    "03",
    "Things settle, nothing bounces",
    "Every ease decelerates into rest. No overshoot, no spring, no elastic. Agents are calm infrastructure; a bounce reads as a toy.",
  ],
  [
    "04",
    "One move at a time",
    "One element leads each beat; the rest wait or follow by stagger. Two things move at once only in the handoff, and then along one path.",
  ],
  [
    "05",
    "State before spectacle",
    "Status changes (working, needs you, in review, done) get the clearest motion on the page. Decorative drift stays under 5% scale and never cycles faster than 24 s.",
  ],
  [
    "06",
    "Reduced motion is a full version",
    "With prefers-reduced-motion the final frame is shown at once, states change by crossfade of 120 ms or less, and nothing drifts. Information never depends on movement.",
  ],
];

export const reducedRules: [string, string][] = [
  ["Signatures", "Render the last frame. Play buttons stay, and say “final frame shown”."],
  ["State changes", "Swap label and colour instantly, or crossfade in 120 ms."],
  ["Atmosphere drift", "Off. The image stays at its first frame."],
  ["Light sweep", "Off. The lit lines stay in their final position."],
  ["Type-on and scramble", "Text appears whole. The caret does not blink."],
  [
    "Faces and cards",
    "Face and card land in their final state at once; a state change is a 120 ms crossfade.",
  ],
  ["Video", "Autoplay off on the site; poster frame shows the end card message."],
];

export const durationUse: Record<string, string> = {
  press: "Button press, toggle, checkbox. Feedback under the finger.",
  hover: "Hover and focus colour, chip outline, link underline.",
  enter: "Element entering: card, toast, menu, log line, status change.",
  sweep: "One light sweep across lines, one bar growing to the now line.",
  handoff: "Work passing from one session or agent to the next; particles converging.",
  loop: "One full run in video and hero moments only: a long sweep across the whole frame.",
};

export const easeUse: Record<EaseName, [string, string]> = {
  out: ["--ease-out", "Default. Anything that enters or responds: fast start, long settle."],
  inOut: [
    "--ease-in-out",
    "Things that move from A to B already on screen: handoff, sweep, atmosphere drift.",
  ],
  loop: ["ease loop", "Long moves: patient start, firm settle. Video and signatures only."],
};

// Tokens of the two visual families. Drift and sweep are measured against the visual's own box.
export const atmosphereTokens: [string, string][] = [
  ["Drift distance", "1.5% of the box, translate only"],
  ["Drift scale", "1.02 → 1.05, never below 1.02 (no empty edge)"],
  ["Drift cycle", "24 s, ease inOut, alternate; never faster"],
  ["Light sweep", "640 ms (sweep), ease inOut, band 40% of the width"],
  ["Converge", "900 ms (handoff), ease inOut, 12 ms stagger per particle"],
  ["Live growth", "Bar to the now line: 900 ms, ease out, origin left"],
  ["Live state change", "Crossfade 260 ms; label, dot and colour change together"],
];

export interface Sig {
  id: string;
  name: string;
  dur: string;
  ease: string;
  use: string;
  timeline: [string, string][];
}

export const signatures: Sig[] = [
  {
    id: "converge",
    name: "Converge",
    dur: "900 ms",
    ease: "inOut",
    use: "Hero entrances, section openers, the first frame of a sting. Scattered work becoming one stream.",
    timeline: [
      ["0 ms", "particles scattered on the left, 35% opacity"],
      ["0–900", "each moves to the stream line, 12 ms stagger"],
      ["900", "stream lit, particles at 100%"],
    ],
  },
  {
    id: "sweep",
    name: "Light sweep",
    dur: "640 ms",
    ease: "inOut",
    use: "Light passing across fine lines, the horizon visual and ruled surfaces. Once per piece.",
    timeline: [
      ["0 ms", "lines at rest, 12% white"],
      ["0–640", "band 40% wide crosses left to right, lines lit blue to cyan"],
      ["640", "band fades out, lines return to rest"],
    ],
  },
  {
    id: "lane-grow",
    name: "Lane grows to now",
    dur: "900 ms",
    ease: "out",
    use: "A run in progress: lanes, timelines and activity bars in Live visuals, demos and video.",
    timeline: [
      ["0 ms", "bar at 55% of its length, grey"],
      ["0–900", "bar grows to the now line, gradient fills in"],
      ["900", "label reads “running · 4 m”"],
    ],
  },
  {
    id: "state-change",
    name: "State change",
    dur: "260 ms per step",
    ease: "out",
    use: "Session and task status in product, demos and video.",
    timeline: [
      ["0 ms", "idle: grey dot, pill at 35%"],
      ["600", "running: cyan dot, pill “● running”"],
      ["1800", "done: check, pill “✓ done”"],
    ],
  },
  {
    id: "log-line",
    name: "Log line",
    dur: "260 ms per line",
    ease: "out",
    use: "Output arriving in a session: tool calls, test runs, commits.",
    timeline: [
      ["0 ms", "existing lines at rest"],
      ["0–260", "new line rises 6 px and fades in"],
      ["+700 / line", "the newest line is cyan, the rest muted"],
    ],
  },
  {
    id: "handoff",
    name: "Handoff to another harness",
    dur: "900 ms",
    ease: "inOut",
    use: "Continue with another agent: work moves from one harness to another with its context; also orchestrator to child session.",
    timeline: [
      ["0 ms", "Claude Code session lit, Codex session at 35%"],
      ["0–900", "packet travels one path, context with it"],
      ["900", "first dims to 35%, second lights"],
    ],
  },
  {
    id: "face-state",
    name: "Face changes state",
    dur: "260 ms",
    ease: "out",
    use: "An always-on agent goes from working to needs you in the agents list, the desktop and the menubar. The clearest move on the page.",
    timeline: [
      ["0 ms", "face thinking, cyan dot, “working”"],
      ["0–260", "crossfade to the surprised face; the dot becomes the amber ▲"],
      ["260", "label reads “needs you”; the row moves up in the list"],
    ],
  },
  {
    id: "card-move",
    name: "Card crosses the board",
    dur: "900 ms",
    ease: "inOut",
    use: "A session changes who holds the next step: the card moves from Working to Needs you to In review.",
    timeline: [
      ["0 ms", "card in Working, 100%"],
      ["0–900", "card travels one column on one path, others hold"],
      ["900", "card lands in Needs you; column count updates"],
    ],
  },
  {
    id: "dock-answer",
    name: "Dock answers",
    dur: "260 ms per step",
    ease: "out",
    use: "The approval dock: the agent asks, you answer, the agent continues. Product, demos and video.",
    timeline: [
      ["0 ms", "dock shows the ask, three actions"],
      ["700", "Allow once takes the press (120 ms)"],
      ["960", "dock resolves to “✓ allowed once · running”"],
    ],
  },
  {
    id: "counter",
    name: "Counter roll",
    dur: "640 ms",
    ease: "out",
    use: "Metrics: running sessions, runs done, stars (dated).",
    timeline: [
      ["0 ms", "digits at 0"],
      ["0–640", "each column rolls to its value"],
      ["+40 / column", "right column settles first, left last"],
    ],
  },
];

export const kineticRules: string[] = [
  "Headlines enter word by word: 260 ms each, ease out, 80 ms stagger, 0.3 em rise. Never letter by letter in display type; it slows reading.",
  "Mono labels may scramble into place: 40 ms per character, left to right, from the set 0–9 A–Z / · _. Only labels of 32 characters or fewer.",
  "One kinetic headline per scene. Body copy and captions never animate; they appear with the scene.",
  "Hold every line for reading time: 0.6 s plus 0.3 s per word, minimum 1.2 s, before anything moves again.",
  "The <em> word lights last: its gradient fades in 120 ms after the line settles. That is the only gradient that touches type.",
  "No kinetic type over a moving visual. The Atmosphere image holds still, and a Live visual stays on its final state, while words move.",
];

export const videoFormats: {
  id: string;
  name: string;
  dur: string;
  size: string;
  use: string;
  beats: [string, string][];
}[] = [
  {
    id: "sting",
    name: "Sting",
    dur: "1.6 s",
    size: "1920×1080 · 1080×1920",
    use: "Opens and closes every video. A light sweep over fine lines resolves into the lockup.",
    beats: [
      ["0.0", "ink, empty"],
      ["0.1–0.74", "light sweep crosses the lines"],
      ["0.74–1.0", "lockup settles"],
      ["1.0–1.6", "hold"],
    ],
  },
  {
    id: "end",
    name: "End card",
    dur: "2.4 s",
    size: "1920×1080 · 1080×1920 · 1080×1350",
    use: "Last frame of every piece: one line, the desktop UI, the URL.",
    beats: [
      ["0.0", "headline word by word"],
      ["0.6", "desktop UI settles behind it"],
      ["1.4", "URL and lockup"],
      ["1.8–2.4", "hold, safe 0.6 s"],
    ],
  },
  {
    id: "demo",
    name: "Product demo",
    dur: "30–90 s",
    size: "1920×1080 · 1080×1350 · 1080×1920",
    use: "One capability, shown in the real desktop app.",
    beats: [
      ["0–3 s", "hook: the problem in one line"],
      ["3–20 s", "mechanism: the screen where it happens"],
      ["20–80 s", "proof: the result on screen"],
      ["last 2.4 s", "end card"],
    ],
  },
];

// One visual per storyboard frame; x/y push the art away from the line on screen.
export const storyboard: {
  t: string;
  beat: string;
  visual: VisualName;
  x: number;
  y: number;
  line: string;
}[] = [
  {
    t: "0.0 s",
    beat: "Pain",
    visual: "terrain",
    x: 0.85,
    y: 0.4,
    line: "Five terminals, and the prompt in the wrong window.",
  },
  {
    t: "3.0 s",
    beat: "Desktop",
    visual: "desktop",
    x: 0.7,
    y: 0.3,
    line: "One place to see every agent, in every harness.",
  },
  {
    t: "9 s",
    beat: "Always-on",
    visual: "agents",
    x: 0.7,
    y: 0.3,
    line: "Always-on agents check in. They call you only when needed.",
  },
  {
    t: "16 s",
    beat: "Handoff",
    visual: "orchestrate",
    x: 0.65,
    y: 0.3,
    line: "Continue with another agent. The context goes with it.",
  },
  {
    t: "23 s",
    beat: "Approvals",
    visual: "approval",
    x: 0.65,
    y: 0.3,
    line: "Allow once, always allow or deny, right in the session.",
  },
  {
    t: "28 s",
    beat: "End card",
    visual: "horizon",
    x: 0.5,
    y: 0.5,
    line: "The autonomous control plane for your agents · compozy.com",
  },
];

export const captionRules: string[] = [
  "Burned-in captions on every social cut, because feed video usually starts muted. Open captions as a sidecar .srt for YouTube and the site.",
  "Figtree 600, white on a 70% ink box, radius 6 px. Height 4.4% of frame height in vertical, 3.6% in horizontal.",
  "Two lines maximum, 32 characters per line, sentence case, no full stop at the end of a caption.",
  "Commands, flags and file names (docs and CLI cuts only) in Geist Mono, brand cyan, exactly as typed. Never translate or paraphrase a command.",
  "Bottom safe area: captions sit above 18% of frame height in 9:16 so the platform UI never covers them.",
];

export const sounds: {
  id: "start" | "done" | "needs";
  name: string;
  glyph: string;
  len: string;
  pitch: string;
  level: string;
  use: string;
  f: [number, number];
}[] = [
  {
    id: "start",
    name: "Start",
    glyph: "●",
    len: "180 ms",
    pitch: "C5 → E5, rising third",
    level: "-18 LUFS short-term",
    use: "A session, loop or daemon starts.",
    f: [523.25, 659.25],
  },
  {
    id: "done",
    name: "Done",
    glyph: "✓",
    len: "320 ms",
    pitch: "E5 + B5, open fifth, soft decay",
    level: "-18 LUFS short-term",
    use: "A run ends with a named outcome.",
    f: [659.25, 987.77],
  },
  {
    id: "needs",
    name: "Needs you",
    glyph: "◐",
    len: "2 × 140 ms",
    pitch: "A4, A4, 120 ms apart",
    level: "-16 LUFS short-term",
    use: "An approval is waiting. Never repeats on its own.",
    f: [440, 440],
  },
];

export const soundRules: string[] = [
  "Three cues only. A new cue needs a state in the product that the three do not cover.",
  "Sine and soft triangle tones, no samples of voices, glass, coins or notification clichés.",
  "Under music, cues sit 6 dB above the bed; under voice, 3 dB below the voice peak.",
  "Video master at -14 LUFS integrated, true peak -1 dBTP. Voice centred, music bed -20 to -24 LUFS.",
  "Music: sparse, electronic, 90–110 BPM, no drops, no risers into the logo. The sting has no music hit, only the done cue.",
  "Every cue is optional in product and off by default. Sound never carries information that the screen does not.",
];

export const delivery: {
  codecs: [string, string, string][];
  frame: [string, string][];
  naming: string;
  examples: string[];
} = {
  codecs: [
    ["Master", "ProRes 422 HQ, .mov, Rec.709, 10-bit", "Archive and re-edits"],
    [
      "Social / site",
      "H.264 High, .mp4, CRF 18 or 12–16 Mbps (1080p), AAC 320 kbps",
      "X, LinkedIn, YouTube, site",
    ],
    ["Site loop", "H.264 .mp4 + VP9 .webm, no audio, ≤ 4 MB, poster .jpg", "Hero and inline loops"],
    ["Alpha", "ProRes 4444 .mov", "Stings over footage"],
    ["Product UI", "Lottie .json or CSS", "In-app motion; never video"],
  ],
  frame: [
    ["Frame rate", "30 fps for everything. Screen recordings captured at 60, conformed to 30."],
    ["Horizontal", "1920×1080 (16:9), 3840×2160 for stage screens"],
    ["Vertical", "1080×1920 (9:16)"],
    ["Feed", "1080×1350 (4:5)"],
    ["Square", "1080×1080, only where 4:5 is not accepted"],
    ["Colour", "Rec.709, ink must render as ink, never crushed to pure black"],
  ],
  naming: "compozy_<type>_<topic>_<ratio>_<duration>_v<n>.<ext>",
  examples: [
    "compozy_sting_brand_16x9_1.6s_v1.mov",
    "compozy_demo_loops_4x5_45s_v3.mp4",
    "compozy_end_install_9x16_2.4s_v2.mp4",
  ],
};
