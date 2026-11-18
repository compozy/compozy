// Copy for the Imagery chapter (06): every list, recipe and rule the sections render.
// Hex values here are displayed text (recipes, specs), not styling.
import { imageById, imgSrc } from "../images";

export const photo = (id: string) => {
  const i = imageById(id);
  return { src: imgSrc(i), alt: i.alt, caption: i.caption, ratio: i.ratio };
};

export const principles = [
  {
    n: "01",
    name: "Real people, real work",
    rule: "Show someone supervising agents: reviewing a result, answering an approval, handing work to another agent, leaving. Never someone typing in a terminal, never someone posing with a laptop.",
    why: "The reader is an engineer. They spot a model in a hoodie at once and stop trusting the page.",
  },
  {
    n: "02",
    name: "Cold daylight and screen light",
    rule: "Two light sources only: a window at 5,600 K or colder, and the glow of a monitor. No warm tungsten, no ring light, no coloured gels.",
    why: "Cold light belongs to ink and cyan. Warm light drags the photo back to the retired orange brand.",
  },
  {
    n: "03",
    name: "Documentary, not staged",
    rule: "Shoot from where a colleague would stand. Eye level or lower, 35–50 mm, people looking at the work, not at the lens.",
    why: "The product claim is “agents keep running and call you only when needed”. The camera should feel like it was already in the room.",
  },
  {
    n: "04",
    name: "Never a readable screen",
    rule: "Screens are light, not content. Blur them, turn them away or shoot past them. Product UI is shown as a product shot (our desktop UI), never inside a photo, and never replaced by a terminal.",
    why: "A readable screen dates in a week, leaks real data and invites the reader to judge a UI at 4 px.",
  },
  {
    n: "05",
    name: "One cool grade",
    rule: "Every photo goes through the same recipe: cool balance, blacks lifted to ink #0a0a0a, saturation down 20%, fine grain.",
    why: "A portrait, a meetup and a desk shot then read as one family, and sit on ink without a visible edge.",
  },
];

export const types = [
  {
    id: "portrait-window",
    name: "Portrait",
    use: "Team page, talk speakers, founder posts, press kit.",
    rules: [
      "One person, window light from one side, background two stops darker.",
      "Shoulders up or waist up; eyes in the top third.",
      "No crossed arms, no laptop prop, no smile on command.",
    ],
  },
  {
    id: "pair-review",
    name: "Pair",
    use: "Approvals, supervision, the human in the loop.",
    rules: [
      "Two people, one decision: one points or reads, the other decides. The subject is the supervision of agents on the desktop app, not a command line.",
      "The screen between them is out of focus or turned away; if it glows, it is the app’s dark UI, not a terminal.",
      "Both faces at least partly visible, never two backs of heads.",
    ],
  },
  {
    id: "hands-detail",
    name: "Detail",
    use: "Section breaks, docs headers, carousels, thumbnails’ second layer.",
    rules: [
      "Hands on a trackpad or mouse answering something, a notebook, a cable, a phone showing a blurred notification. One object in focus. A keyboard is context, never the subject.",
      "Shallow depth: f/1.8–2.8. The rest falls to ink.",
      "No brand logos on hardware; tape them or shoot around them.",
    ],
  },
  {
    id: "meetup-room",
    name: "Community",
    use: "Meetups, Discord events, community recaps, YouTube banners.",
    rules: [
      "Wide, from the back or the side of the room: people listening, not a speaker posing.",
      "Faces in the crowd stay small or soft unless each person agreed.",
      "The stage screen shows our desktop UI (Live) or an Atmosphere visual, never a terminal and never a slide with text.",
    ],
  },
  {
    id: "laptop-closed",
    name: "The promise",
    use: "Hero, launch posts, ads, the cover of the deck.",
    rules: [
      "The work keeps going while nobody watches: a closed laptop, an empty chair, a status light.",
      "One small light source in the frame (cyan LED, monitor standby) and nothing else lit.",
      "Never literal: no robot, no clock, no “24/7”.",
    ],
  },
];

export const productRules: [rule: string, text: string][] = [
  [
    "Our UI is the hero",
    "The desktop app is the product image: the desktop, the agents list, the board, the handoff picker, the approval dock. Never a terminal window as the main shot; a terminal appears only in docs and CLI pages, small.",
  ],
  [
    "Real UI only",
    "Product shots are captures of the shipping app against the demo seed (desktop, board, orchestrate), rendered at 2x. Mockups drawn in Figma are concept art and say so in the caption.",
  ],
  [
    "On ink",
    "Product shots sit on #0a0a0a or #131313, never on a photo, a gradient field or paper. The app is dark; the ground matches it.",
  ],
  [
    "1px window",
    "One frame: 1 px #383838 border, 12 px radius at 1x, a 36 px title bar with three #262626 dots. No glass, no shadow, no reflection.",
  ],
  [
    "Crop to one task",
    "Show the 600–900 px of the screen that prove the sentence next to it. Full-window shots only on the download page.",
  ],
  [
    "Flat, straight, 1:1",
    "Product shots sit flat and square in the layout. Perspective framing belongs only to covers and backgrounds, where the manual's product-shot frame applies it. No tilted devices, no floating phones. Scale the shot, never skew it.",
  ],
  [
    "Clean data",
    "Seed a demo workspace: real-looking repo names, no customer names, no personal emails, no tokens, no profanity in branch names.",
  ],
  [
    "Annotate in cyan",
    "At most two callouts: a 1 px cyan rule and a Geist Mono label at 12 px. Arrows, circles and highlighter strokes are out.",
  ],
];

export const recipe: [step: string, value: string][] = [
  ["White balance", "Cool: 5,200–5,600 K, tint −4. Skin stays natural, never blue."],
  ["Blacks", "Lifted to ink #0a0a0a (RGB 8,15,20). Nothing in the frame is darker than the page."],
  ["Saturation", "−20% overall; blues and cyans held at 0, oranges and reds −35%."],
  ["Contrast", "−8%, with a gentle S-curve: highlights roll off, shadows stay open."],
  ["Grain", "Fine, 8% amount, size 20, roughness 50. Added last, after resizing."],
  ["Sharpening", "Output only, screen, low. No clarity or texture above +10."],
];

export const lightroom: [control: string, value: string][] = [
  ["Temp / Tint", "−12 / −4"],
  ["Contrast", "−8"],
  ["Blacks", "+18 (check the ink point)"],
  ["Saturation", "−20"],
  ["HSL Orange / Red sat.", "−35 / −35"],
  ["Calibration Blue primary", "Hue −6, Sat +4"],
  ["Grain", "8 / 20 / 50"],
];

export const cssRecipe = `.cz-photo { position: relative; isolation: isolate; }
.cz-photo img { filter: saturate(.8) contrast(.92) brightness(1.03); }
/* cool balance */
.cz-photo::before { content: ""; position: absolute; inset: 0;
  background: #1F87FC; opacity: .14; mix-blend-mode: soft-light; }
/* blacks lifted to ink: lighten keeps every pixel ≥ #0a0a0a */
.cz-photo::after { content: ""; position: absolute; inset: 0;
  background: #0a0a0a url(grain.svg); background-blend-mode: overlay;
  mix-blend-mode: lighten; }`;

export const crops = [
  {
    ratio: "1/1",
    label: "1:1",
    use: "Avatars, LinkedIn post, Discord",
    id: "portrait-window",
    pos: "50% 30%",
  },
  {
    ratio: "4/5",
    label: "4:5",
    use: "Carousels, feed portrait",
    id: "portrait-window",
    pos: "50% 35%",
  },
  { ratio: "3/2", label: "3:2", use: "Blog, docs, press", id: "pair-review", pos: "50% 40%" },
  { ratio: "16/9", label: "16:9", use: "X, YouTube, slides", id: "meetup-room", pos: "50% 50%" },
  { ratio: "9/16", label: "9:16", use: "Vertical covers", id: "operator-night", pos: "40% 50%" },
];

export const promptSchema = `subject:     who or what, doing which task (one sentence)
setting:     place and time of day (cold daylight | screen light at night)
camera:      35mm or 50mm, eye level, documentary, shallow depth
light:       window 5600K from the left | monitor glow only
screen:      out of focus, turned away or off (never readable)
palette:     ink shadows, cool neutrals, one cyan accent at most
grade:       lifted blacks, desaturated 20%, fine film grain
exclude:     text, logos, letters, UI, robots, holograms, glow, stock smile
ratio:       4:5 | 3:2 | 16:9`;

export const promptExample = `subject:  a closed laptop at the end of the day, a status light still on
setting:  a quiet desk by a window, blue hour, the room almost dark
camera:   50mm, eye level, documentary, f/2
light:    cold window light from the left, nothing else
screen:   laptop closed
palette:  ink shadows, cool greys, a single cyan LED
grade:    lifted blacks, desaturated 20%, fine grain
exclude:  text, logos, readable screens, robots, glow effects
ratio:    3:2`;

export const genAllowed = [
  "Mood and direction references inside this manual and internal decks.",
  "Objects, rooms and details where no person is identifiable (desk, keyboard, closed laptop).",
  "Backgrounds for ads and thumbnails that a person will later replace with a real photo.",
  "Explorations for a shoot brief: the photographer sees what to aim for.",
  "Atmosphere art in the house style: pure black ground, blue and cyan only, particles, light and fine lines, no rings, no figures. The approved set is swarm, converge, block, cone, horizon, terrain and field.",
];
export const genForbidden = [
  "Standing in for a real person: team members, partners, design partners, speakers or users.",
  "Testimonials, case studies, “people who use Compozy” or anything implying adoption.",
  "Any text, letters, numbers, logos or UI inside the generated image. Type is added in layout.",
  "Event recaps or meetups that did not happen as shown.",
  "Faces of public figures, or a style prompt naming a living photographer.",
];

export const avoid = [
  {
    name: "Robots and androids",
    why: "Agents are processes, not bodies. A chrome hand turns a tool for engineers into a sci-fi poster.",
    instead: "A person closing a laptop while the work keeps going.",
  },
  {
    name: "Glowing brains",
    why: "Says “intelligence” with no claim behind it, and every AI company already uses it.",
    instead:
      "A Live visual of real state (desktop, board, orchestrate) or an Atmosphere visual such as converge.",
  },
  {
    name: "Matrix code rain",
    why: "Readable-looking text that means nothing; developers read it and wince.",
    instead: "A real agent face with its state, or a board card set as UI.",
  },
  {
    name: "Handshake over a hologram",
    why: "Partnership clichés imply deals and customers that do not exist.",
    instead: "Two people over one screen, deciding.",
  },
  {
    name: "Neon and purple glow",
    why: "Off palette, and it hides that there is nothing to show.",
    instead: "Cold daylight; cyan only where something is live.",
  },
  {
    name: "Floating dashboards",
    why: "Tilted, glassy UI is a fake product. The reader assumes the real one looks worse.",
    instead: "A flat, cropped product shot in a 1 px window.",
  },
  {
    name: "Stock smile at laptop",
    why: "Posed people looking at the lens read as an ad, not as engineers at work.",
    instead: "Documentary angle, people looking at the work.",
  },
  {
    name: "Server racks and data centres",
    why: "Compozy is a desktop app that controls agents, not infrastructure. Racks suggest hosting that is not the product.",
    instead: "One desk, one desktop app, a status light.",
  },
  {
    name: "A terminal as the subject",
    why: "The terminal is the before picture: five windows and the prompt in the wrong one. Showing it as the product tells the reader the product is a CLI.",
    instead: "The desktop UI: faces, board, dock. Show the terminal only to show the problem.",
  },
];

export const consent: [check: string, text: string][] = [
  [
    "Model release",
    "Signed release for every identifiable person, stored with the original file. Verbal is not enough.",
  ],
  [
    "Event notice",
    "Meetup sign-up and the door say photos will be taken. Offer a sticker or lanyard colour for “no photos” and respect it.",
  ],
  ["Minors", "Never photographed at events, even in the background."],
  [
    "Screens",
    "Check every screen in frame for tokens, emails, customer names and Slack messages before export.",
  ],
  [
    "Removal",
    "Anyone can ask to be removed. Take the image down from owned channels within 48 hours.",
  ],
  [
    "Generated",
    "Generated images carry the provenance caption and never depict an identifiable real person.",
  ],
];

export const altPattern = [
  {
    bad: "Photo of developer",
    good: "An engineer reviewing a pull request at night, face lit by the monitor.",
  },
  {
    bad: "Compozy laptop image",
    good: "A closed laptop on a desk at blue hour, a small cyan status light still on.",
  },
  { bad: "Screenshot", good: "Compozy desktop: three always-on agents, one marked Needs you." },
];

export const credits: [kind: string, line: string][] = [
  ["Photo", "Photo: Name Surname, 2026"],
  ["Generated", "Generated reference, not a photograph"],
  ["Product", "Compozy desktop v0.6, real build, demo workspace"],
  ["Event", "Event name, City, Month Year. Photo: Name Surname"],
];
